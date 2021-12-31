package relayer

import (
	"fmt"

	"github.com/avast/retry-go"
	ibcexported "github.com/cosmos/ibc-go/v2/modules/core/exported"
	"github.com/cosmos/relayer/relayer/provider"
)

// CreateClients creates clients for src on dst and dst on src if the client ids are unspecified.
// TODO: de-duplicate code
func (c *Chain) CreateClients(dst *Chain, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour, override bool) (modified bool, err error) {
	var srcUpdateHeader, dstUpdateHeader ibcexported.Header

	var srch, dsth int64
	if err = retry.Do(func() error {
		srch, dsth, err = QueryLatestHeights(c, dst)
		if srch == 0 || dsth == 0 {
			return fmt.Errorf("failed to query latest heights")
		}
		return err
	}); err != nil {
		return false, err
	}

	srcUpdateHeader, dstUpdateHeader, err = GetLightSignedHeadersAtHeights(c, dst, srch, dsth)
	if err != nil {
		return false, err
	}

	// Create client for the destination chain on the source chain if client id is unspecified
	if c.PathEnd.ClientID == "" {
		if c.debug {
			c.logCreateClient(dst, dstUpdateHeader.GetHeight().GetRevisionHeight())
		}
		ubdPeriod, err := dst.ChainProvider.QueryUnbondingPeriod()
		if err != nil {
			return modified, err
		}

		// Create the ClientState we want on 'c' tracking 'dst'
		clientState, err := c.ChainProvider.NewClientState(dstUpdateHeader, dst.GetTrustingPeriod(), ubdPeriod, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour)
		if err != nil {
			return modified, err
		}

		var (
			clientID string
			found    bool
		)
		// Will not reuse same client if override is true
		if !override {
			// Check if an identical light client already exists
			clientID, found = c.ChainProvider.FindMatchingClient(dst.ChainProvider, clientState)
			fmt.Println(found)
		}
		if !found || override {
			createMsg, err := c.ChainProvider.CreateClient(clientState, dstUpdateHeader)
			if err != nil {
				return modified, err
			}

			msgs := []provider.RelayerMessage{createMsg}

			// if a matching client does not exist, create one
			res, success, err := c.ChainProvider.SendMessages(msgs)
			if err != nil {
				c.LogFailedTx(res, err, msgs)
				return modified, err
			}
			if !success {
				c.LogFailedTx(res, err, msgs)
				return modified, fmt.Errorf("tx failed: %s", res.Data)
			}

			// update the client identifier
			// use index 0, the transaction only has one message
			clientID, err = ParseClientIDFromEvents(res.Events)
			if err != nil {
				return modified, err
			}
		} else if c.debug {
			c.logIdentifierExists(dst, "client", clientID)
		}

		c.PathEnd.ClientID = clientID
		modified = true
	} else {
		// Ensure client exists in the event of user inputted identifiers
		// TODO: check client is not expired
		_, err := c.ChainProvider.QueryClientStateResponse(int64(srcUpdateHeader.GetHeight().GetRevisionHeight()), c.ClientID())
		if err != nil {
			return false, fmt.Errorf("please ensure provided on-chain client (%s) exists on the chain (%s): %v",
				c.PathEnd.ClientID, c.ChainID(), err)
		}
	}

	// Create client for the source chain on destination chain if client id is unspecified
	if dst.PathEnd.ClientID == "" {
		if dst.debug {
			dst.logCreateClient(c, srcUpdateHeader.GetHeight().GetRevisionHeight())
		}
		ubdPeriod, err := c.ChainProvider.QueryUnbondingPeriod()
		if err != nil {
			return modified, err
		}
		// Create the ClientState we want on 'dst' tracking 'c'
		clientState, err := dst.ChainProvider.NewClientState(srcUpdateHeader, c.GetTrustingPeriod(), ubdPeriod, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour)
		if err != nil {
			return modified, err
		}

		var (
			clientID string
			found    bool
		)
		// Will not reuse same client if override is true
		if !override {
			// Check if an identical light client already exists
			// NOTE: we pass in 'dst' as the source and 'c' as the
			// counterparty.
			clientID, found = dst.ChainProvider.FindMatchingClient(c.ChainProvider, clientState)
		}
		if !found || override {
			createMsg, err := dst.ChainProvider.CreateClient(clientState, srcUpdateHeader)
			if err != nil {
				return modified, err
			}

			msgs := []provider.RelayerMessage{createMsg}

			// if a matching client does not exist, create one
			res, success, err := dst.ChainProvider.SendMessages(msgs)
			if err != nil {
				dst.LogFailedTx(res, err, msgs)
				return modified, err
			}
			if !success {
				dst.LogFailedTx(res, err, msgs)
				return modified, fmt.Errorf("tx failed: %s", res.Data)
			}

			// update client identifier
			clientID, err = ParseClientIDFromEvents(res.Events)
			if err != nil {
				return modified, err
			}
		} else if c.debug {
			c.logIdentifierExists(dst, "client", clientID)
		}

		dst.PathEnd.ClientID = clientID
		modified = true

	} else {
		// Ensure client exists in the event of user inputted identifiers
		// TODO: check client is not expired
		_, err := dst.ChainProvider.QueryClientStateResponse(int64(dstUpdateHeader.GetHeight().GetRevisionHeight()), dst.ClientID())
		if err != nil {
			return false, fmt.Errorf("please ensure provided on-chain client (%s) exists on the chain (%s): %v",
				dst.PathEnd.ClientID, dst.ChainID(), err)
		}

	}

	c.Log(fmt.Sprintf("★ Clients created: client(%s) on chain[%s] and client(%s) on chain[%s]",
		c.PathEnd.ClientID, c.ChainID(), dst.PathEnd.ClientID, dst.ChainID()))

	return modified, nil
}

// UpdateClients updates clients for src on dst and dst on src given the configured paths
func (c *Chain) UpdateClients(dst *Chain) (err error) {
	var (
		clients                          = &RelayMsgs{Src: []provider.RelayerMessage{}, Dst: []provider.RelayerMessage{}}
		srcUpdateHeader, dstUpdateHeader ibcexported.Header
	)

	srch, dsth, err := QueryLatestHeights(c, dst)
	if err != nil {
		return err
	}

	srcUpdateHeader, dstUpdateHeader, err = GetLightSignedHeadersAtHeights(c, dst, srch, dsth)
	if err != nil {
		return err
	}

	srcUpdateMsg, err := c.ChainProvider.UpdateClient(c.ClientID(), dstUpdateHeader)
	if err != nil {
		return err
	}
	dstUpdateMsg, err := dst.ChainProvider.UpdateClient(dst.ClientID(), srcUpdateHeader)
	if err != nil {
		return err
	}

	clients.Src = append(clients.Src, srcUpdateMsg)
	clients.Dst = append(clients.Dst, dstUpdateMsg)

	// Send msgs to both chains
	if clients.Ready() {
		if clients.Send(c, dst); clients.Success() {
			c.Log(fmt.Sprintf("★ Clients updated: [%s]client(%s) {%d}->{%d} and [%s]client(%s) {%d}->{%d}",
				c.ChainID(),
				c.PathEnd.ClientID,
				provider.MustGetHeight(srcUpdateHeader.GetHeight()),
				srcUpdateHeader.GetHeight().GetRevisionHeight(),
				dst.ChainID(),
				dst.PathEnd.ClientID,
				provider.MustGetHeight(dstUpdateHeader.GetHeight()),
				dstUpdateHeader.GetHeight().GetRevisionHeight(),
			),
			)
		}
	}

	return nil
}

// UpgradeClients upgrades the client on src after dst chain has undergone an upgrade.
func (c *Chain) UpgradeClients(dst *Chain, height int64) error {
	dstHeader, err := dst.ChainProvider.GetLightSignedHeaderAtHeight(height)
	if err != nil {
		return err
	}

	// updates off-chain light client
	updateMsg, err := c.ChainProvider.UpdateClient(c.ClientID(), dstHeader)
	if err != nil {
		return err
	}

	if height == 0 {
		height, err = dst.ChainProvider.QueryLatestHeight()
		if err != nil {
			return err
		}
	}

	// query proofs on counterparty
	clientRes, err := dst.ChainProvider.QueryUpgradedClient(height)
	if err != nil {
		return err
	}

	consRes, err := dst.ChainProvider.QueryUpgradedConsState(height)
	if err != nil {
		return err
	}

	upgradeMsg := c.ChainProvider.MsgUpgradeClient(c.ClientID(), consRes, clientRes)

	msgs := []provider.RelayerMessage{
		updateMsg,
		upgradeMsg,
	}

	res, _, err := c.ChainProvider.SendMessages(msgs)
	if err != nil {
		c.LogFailedTx(res, err, msgs)
		return err
	}

	return nil
}
