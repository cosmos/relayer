package relayer

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// CreateClients creates clients for src on dst and dst on src if the client ids are unspecified.
func (c *Chain) CreateClients(dst *Chain) (err error) {
	var (
		clients = &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}}
	)

	srcH, dstH, err := UpdatesWithHeaders(c, dst)
	if err != nil {
		return err
	}

	// Create client for the destination chain on the source chain if client id is unspecified
	if c.PathEnd.ClientID == "" {
		if c.debug {
			c.logCreateClient(dst, dstH.Header.Height)
		}
		ubdPeriod, err := dst.QueryUnbondingPeriod()
		if err != nil {
			return err
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
			return err
		}
		if !success {
			return fmt.Errorf("tx failed: %s", res.RawLog)
		}

		// update the client identifier
		// use index 0, the transaction only has one message
		clientID, err := ParseClientIDFromEvents(res.Logs[0].Events)
		if err != nil {
			return err
		}
		c.PathEnd.ClientID = clientID

		// TOOD: ensure config file is updated

	} else {
		// TODO return error if the client does not exist
	}

	// Create client for the source chain on destination chain if client id is unspecified
	if dst.PathEnd.ClientID == "" {
		if dst.debug {
			dst.logCreateClient(c, srcH.Header.Height)
		}
		ubdPeriod, err := c.QueryUnbondingPeriod()
		if err != nil {
			return err
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
			return err
		}
		if !success {
			return fmt.Errorf("tx failed: %s", res.RawLog)
		}

		// update client identifier
		clientID, err := ParseClientIDFromEvents(res.Logs[0].Events)
		if err != nil {
			return err
		}
		dst.PathEnd.ClientID = clientID

		// TODO: ensure config file is updated

	} else {
		// TODO: throw error if the client does not exist
	}

	// Send msgs to both chains
	if clients.Ready() {
		// TODO: Add retry here for out of gas or other errors
		if clients.Send(c, dst); clients.Success() {
			c.Log(fmt.Sprintf("★ Clients created: [%s]client(%s) and [%s]client(%s)",
				c.ChainID, c.PathEnd.ClientID, dst.ChainID, dst.PathEnd.ClientID))
		}
	}

	return nil
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
