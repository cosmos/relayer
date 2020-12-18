package relayer

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/types"
)

// CreateClients creates clients for src on dst and dst on src if the client ids are unspecified.
func (c *Chain) CreateClients(dst *Chain) (modified bool, err error) {
	// Handle off chain light clients
	if err := c.ValidateLightInitialized(); err != nil {
		return false, err
	}

	if err = dst.ValidateLightInitialized(); err != nil {
		return false, err
	}

	srcH, dstH, err := UpdatesWithHeaders(c, dst)
	if err != nil {
		return false, err
	}

	// Create client for the destination chain on the source chain if client id is unspecified
	if c.PathEnd.ClientID == "" {
		if c.debug {
			c.logCreateClient(dst, dstH.Header.Height)
		}
		ubdPeriod, err := dst.QueryUnbondingPeriod()
		if err != nil {
			return modified, err
		}
		msgs := []sdk.Msg{
			c.PathEnd.CreateClient(
				dstH,
				dst.GetTrustingPeriod(),
				ubdPeriod,
				c.MustGetAddress(),
			),
		}
		res, success, err := c.SendMsgs(msgs)
		if err != nil {
			return modified, err
		}
		if !success {
			return modified, fmt.Errorf("tx failed: %s", res.RawLog)
		}

		// update the client identifier
		// use index 0, the transaction only has one message
		clientID, err := ParseClientIDFromEvents(res.Logs[0].Events)
		if err != nil {
			return modified, err
		}
		c.PathEnd.ClientID = clientID
		modified = true

	} else {
		// Ensure client exists in the event of user inputted identifiers
		_, err := c.QueryClientState(srcH.Header.Height)
		if err != nil {
			return false, fmt.Errorf("please ensure provided on-chain client (%s) exists on the chain (%s): %v", c.PathEnd.ClientID, c.ChainID, err)
		}
	}

	// Create client for the source chain on destination chain if client id is unspecified
	if dst.PathEnd.ClientID == "" {
		if dst.debug {
			dst.logCreateClient(c, srcH.Header.Height)
		}
		ubdPeriod, err := c.QueryUnbondingPeriod()
		if err != nil {
			return modified, err
		}
		msgs := []sdk.Msg{
			dst.PathEnd.CreateClient(
				srcH,
				c.GetTrustingPeriod(),
				ubdPeriod,
				dst.MustGetAddress(),
			),
		}
		res, success, err := dst.SendMsgs(msgs)
		if err != nil {
			return modified, err
		}
		if !success {
			return modified, fmt.Errorf("tx failed: %s", res.RawLog)
		}

		// update client identifier
		clientID, err := ParseClientIDFromEvents(res.Logs[0].Events)
		if err != nil {
			return modified, err
		}
		dst.PathEnd.ClientID = clientID
		modified = true

	} else {
		// Ensure client exists in the event of user inputted identifiers
		_, err := dst.QueryClientState(dstH.Header.Height)
		if err != nil {
			return false, fmt.Errorf("please ensure provided on-chain client (%s) exists on the chain (%s): %v", dst.PathEnd.ClientID, dst.ChainID, err)
		}

	}

	c.Log(fmt.Sprintf("★ Clients created: client(%s) on chain[%s] and client(%s) on chain[%s]",
		c.PathEnd.ClientID, c.ChainID, dst.PathEnd.ClientID, dst.ChainID))

	return modified, nil
}

// UpdateClients updates clients for src on dst and dst on src given the configured paths
func (c *Chain) UpdateClients(dst *Chain) (err error) {
	clients := &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}}

	sh, err := NewSyncHeaders(c, dst)
	if err != nil {
		return err
	}

	srcUH, dstUH, err := sh.GetTrustedHeaders(c, dst)
	if err != nil {
		return err
	}

	clients.Src = append(clients.Src, c.PathEnd.UpdateClient(dstUH, c.MustGetAddress()))
	clients.Dst = append(clients.Dst, dst.PathEnd.UpdateClient(srcUH, dst.MustGetAddress()))

	// Send msgs to both chains
	if clients.Ready() {
		if clients.Send(c, dst); clients.Success() {
			c.Log(fmt.Sprintf("★ Clients updated: [%s]client(%s) {%d}->{%d} and [%s]client(%s) {%d}->{%d}",
				c.ChainID,
				c.PathEnd.ClientID,
				MustGetHeight(srcUH.TrustedHeight),
				srcUH.Header.Height,
				dst.ChainID,
				dst.PathEnd.ClientID,
				MustGetHeight(dstUH.TrustedHeight),
				dstUH.Header.Height,
			),
			)
		}
	}

	return nil
}

// UpgradesClients upgrades the client on src after dst chain has undergone an upgrade.
func (c *Chain) UpgradeClients(dst *Chain) error {
	sh, err := NewSyncHeaders(c, dst)
	if err != nil {
		return err
	}
	if err := sh.Updates(c, dst); err != nil {
		return err
	}
	height := int64(sh.GetHeight(dst.ChainID))

	// TODO: construct method of only attempting to get dst header
	// Note: we explicitly do not check the error since the source
	// trusted header will fail
	_, dstUpdateHeader, _ := sh.GetTrustedHeaders(c, dst)

	// query proofs on counterparty
	clientState, proofUpgradeClient, _, err := dst.QueryUpgradedClient(height)
	if err != nil {
		return err
	}

	consensusState, proofUpgradeConsensusState, _, err := dst.QueryUpgradedConsState(height)
	if err != nil {
		return err
	}

	upgradeMsg := &clienttypes.MsgUpgradeClient{c.PathEnd.ClientID, clientState, consensusState, proofUpgradeClient, proofUpgradeConsensusState, c.MustGetAddress().String()}
	if err != nil {
		return err
	}

	msgs := []sdk.Msg{
		c.PathEnd.UpdateClient(dstUpdateHeader, c.MustGetAddress()),
		upgradeMsg,
	}

	_, _, err = c.SendMsgs(msgs)
	if err != nil {
		return err
	}

	return nil
}
