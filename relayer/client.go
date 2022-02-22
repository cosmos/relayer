package relayer

import (
	"fmt"
	"time"

	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"

	"github.com/avast/retry-go"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	"github.com/cosmos/relayer/relayer/provider"
)

// CreateClients creates clients for src on dst and dst on src if the client ids are unspecified.
func (c *Chain) CreateClients(dst *Chain, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour, override bool) (bool, error) {
	var (
		srcUpdateHeader, dstUpdateHeader ibcexported.Header
		srch, dsth                       int64
		modified                         bool
		err                              error
	)

	// Query the latest heights on src and dst and retry if the query fails
	if err = retry.Do(func() error {
		srch, dsth, err = QueryLatestHeights(c, dst)
		if srch == 0 || dsth == 0 || err != nil {
			return fmt.Errorf("failed to query latest heights. Err: %w", err)
		}
		return err
	}, RtyAtt, RtyDel, RtyErr); err != nil {
		return false, err
	}

	// Query the light signed headers for src & dst at the heights srch & dsth, retry if the query fails
	if err = retry.Do(func() error {
		srcUpdateHeader, dstUpdateHeader, err = GetLightSignedHeadersAtHeights(c, dst, srch, dsth)
		if err != nil {
			return fmt.Errorf("failed to query light signed headers. Err: %w", err)
		}
		return err
	}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		c.LogRetryGetLightSignedHeader(n, err)
		srch, dsth, _ = QueryLatestHeights(c, dst)
	})); err != nil {
		return false, err
	}

	// Create client on src for dst if the client id is unspecified
	modified, err = CreateClient(c, dst, srcUpdateHeader, dstUpdateHeader, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour, override)
	if err != nil {
		return modified, fmt.Errorf("failed to create client on src chain{%s}. Err: %w", c.ChainID(), err)
	}

	// Create client on dst for src if the client id is unspecified
	modified, err = CreateClient(dst, c, dstUpdateHeader, srcUpdateHeader, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour, override)
	if err != nil {
		return modified, fmt.Errorf("failed to create client on dst chain{%s}. Err: %w", dst.ChainID(), err)
	}

	c.Log(fmt.Sprintf("★ Clients created: client(%s) on chain[%s] and client(%s) on chain[%s]",
		c.PathEnd.ClientID, c.ChainID(), dst.PathEnd.ClientID, dst.ChainID()))

	return modified, nil
}

func CreateClient(src, dst *Chain, srcUpdateHeader, dstUpdateHeader ibcexported.Header, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour, override bool) (bool, error) {
	var (
		modified, found, success bool
		err                      error
		ubdPeriod, tp            time.Duration
		clientID                 string
		res                      *provider.RelayerTxResponse
	)

	// Create client for the destination chain on the source chain if client id is unspecified
	if src.PathEnd.ClientID == "" {

		// Query the trusting period for dst and retry if the query fails
		if err = retry.Do(func() error {
			tp, err = dst.GetTrustingPeriod()
			if err != nil || tp.String() == "0s" {
				return fmt.Errorf("failed to get trusting period for chain{%s}. Err: %w", dst.ChainID(), err)
			}
			return err
		}, RtyAtt, RtyDel, RtyErr); err != nil {
			return modified, err
		}

		if src.debug {
			src.logCreateClient(dst, dstUpdateHeader.GetHeight().GetRevisionHeight(), &tp)
		}

		// Query the unbonding period for dst and retry if the query fails
		if err = retry.Do(func() error {
			ubdPeriod, err = dst.ChainProvider.QueryUnbondingPeriod()
			if err != nil {
				return fmt.Errorf("failed to query unbonding period for chain{%s}. Err: %w \n", dst.ChainID(), err)
			}
			return err
		}, RtyAtt, RtyDel, RtyErr); err != nil {
			return modified, err
		}

		// Create the ClientState we want on 'src' tracking 'dst'
		clientState, err := src.ChainProvider.NewClientState(dstUpdateHeader, tp, ubdPeriod, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour)
		if err != nil {
			return modified, fmt.Errorf("failed to create new client state for chain{%s} tracking chain{%s}. Err: %w \n", src.ChainID(), dst.ChainID(), err)
		}

		// Will not reuse same client if override is true
		if !override {
			// Check if an identical light client already exists
			clientID, found = src.ChainProvider.FindMatchingClient(dst.ChainProvider, clientState)
		}

		if !found || override {
			if src.debug {
				src.Log(fmt.Sprintf("No client found on src chain {%s} tracking the state of counterparty chain {%s}", src.ChainID(), dst.ChainID()))
			}

			createMsg, err := src.ChainProvider.CreateClient(clientState, dstUpdateHeader)
			if err != nil {
				return modified, fmt.Errorf("failed to compose CreateClient msg for chain{%s}. Err: %w \n", src.ChainID(), err)
			}

			msgs := []provider.RelayerMessage{createMsg}

			// if a matching client does not exist, create one
			if err = retry.Do(func() error {
				res, success, err = src.ChainProvider.SendMessages(msgs)
				if err != nil {
					src.LogFailedTx(res, err, msgs)
					return fmt.Errorf("failed to send messages on chain{%s}. Err: %w \n", src.ChainID(), err)
				}

				if !success {
					src.LogFailedTx(res, err, msgs)
					return fmt.Errorf("tx failed on chain{%s}: %s", src.ChainID(), res.Data)
				}
				return err
			}, RtyAtt, RtyDel, RtyErr); err != nil {
				return modified, err
			}

			// update the client identifier
			// use index 0, the transaction only has one message
			if clientID, err = ParseClientIDFromEvents(res.Events); err != nil {
				return modified, err
			}
		} else if src.debug {
			src.logIdentifierExists(dst, "client", clientID)
		}

		src.PathEnd.ClientID = clientID
		modified = true
	} else {
		// Ensure client exists in the event of user inputted identifiers
		// TODO: check client is not expired
		_, err := src.ChainProvider.QueryClientStateResponse(int64(srcUpdateHeader.GetHeight().GetRevisionHeight()), src.ClientID())
		if err != nil {
			return false, fmt.Errorf("please ensure provided on-chain client (%s) exists on the chain (%s): %v",
				src.PathEnd.ClientID, src.ChainID(), err)
		}
	}

	return modified, nil
}

// UpdateClients updates clients for src on dst and dst on src given the configured paths
func (c *Chain) UpdateClients(dst *Chain) (err error) {
	var (
		clients                          = &RelayMsgs{Src: []provider.RelayerMessage{}, Dst: []provider.RelayerMessage{}}
		srcUpdateHeader, dstUpdateHeader ibcexported.Header
		srch, dsth                       int64
	)

	if err = retry.Do(func() error {
		srch, dsth, err = QueryLatestHeights(c, dst)
		if srch == 0 || dsth == 0 || err != nil {
			c.Log(fmt.Sprintf("failed to query latest heights. Err: %v", err))
			return err
		}
		return err
	}, RtyAtt, RtyDel, RtyErr); err != nil {
		return err
	}

	if err = retry.Do(func() error {
		srcUpdateHeader, dstUpdateHeader, err = GetIBCUpdateHeaders(srch, dsth, c.ChainProvider, dst.ChainProvider, c.ClientID(), dst.ClientID())
		if err != nil {
			c.Log(fmt.Sprintf("failed to query light signed headers. Err: %v", err))
			return err
		}
		return err
	}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		c.Log(fmt.Sprintf("failed to get IBC update headers, try(%d/%d). Err: %v", n+1, RtyAttNum, err))
		srch, dsth, _ = QueryLatestHeights(c, dst)
	})); err != nil {
		return err
	}

	srcUpdateMsg, err := c.ChainProvider.UpdateClient(c.ClientID(), dstUpdateHeader)
	if err != nil {
		if c.debug {
			c.Log(fmt.Sprintf("failed to update client on chain{%s} \n", c.ChainID()))
		}
		return err
	}

	dstUpdateMsg, err := dst.ChainProvider.UpdateClient(dst.ClientID(), srcUpdateHeader)
	if err != nil {
		if dst.debug {
			dst.Log(fmt.Sprintf("failed to update client on chain{%s} \n", dst.ChainID()))
		}
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
				MustGetHeight(srcUpdateHeader.GetHeight()),
				srcUpdateHeader.GetHeight().GetRevisionHeight(),
				dst.ChainID(),
				dst.PathEnd.ClientID,
				MustGetHeight(dstUpdateHeader.GetHeight()),
				dstUpdateHeader.GetHeight().GetRevisionHeight()))
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

	upgradeMsg, err := c.ChainProvider.MsgUpgradeClient(c.ClientID(), consRes, clientRes)
	if err != nil {
		return err
	}

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

// MustGetHeight takes the height inteface and returns the actual height
func MustGetHeight(h ibcexported.Height) clienttypes.Height {
	height, ok := h.(clienttypes.Height)
	if !ok {
		panic("height is not an instance of height!")
	}
	return height
}
