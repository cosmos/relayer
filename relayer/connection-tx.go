package relayer

import (
	"fmt"
	"time"

	retry "github.com/avast/retry-go"
	clienttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/types"
	conntypes "github.com/cosmos/cosmos-sdk/x/ibc/core/03-connection/types"
	ibcexported "github.com/cosmos/cosmos-sdk/x/ibc/core/exported"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
	"golang.org/x/sync/errgroup"
)

// CreateConnection runs the connection creation messages on timeout until they pass
// TODO: add max retries or something to this function
func (c *Chain) CreateConnection(dst *Chain, to time.Duration) error {
	ticker := time.NewTicker(to)
	failed := 0
	for ; true; <-ticker.C {
		connSteps, err := c.CreateConnectionStep(dst)
		if err != nil {
			return err
		}

		if !connSteps.Ready() {
			break
		}

		connSteps.Send(c, dst)

		switch {
		// In the case of success and this being the last transaction
		// debug logging, log created connection and break
		case connSteps.success && connSteps.last:
			if c.debug {
				srcH, dstH, err := GetLatestLightHeights(c, dst)
				if err != nil {
					return err
				}
				srcConn, dstConn, err := QueryConnectionPair(c, dst, srcH, dstH)
				if err != nil {
					return err
				}
				logConnectionStates(c, dst, srcConn, dstConn)
			}

			c.Log(fmt.Sprintf("★ Connection created: [%s]client{%s}conn{%s} -> [%s]client{%s}conn{%s}",
				c.ChainID, c.PathEnd.ClientID, c.PathEnd.ConnectionID,
				dst.ChainID, dst.PathEnd.ClientID, dst.PathEnd.ConnectionID))
			return nil
		// In the case of success, reset the failures counter
		case connSteps.success:
			failed = 0
			continue
		// In the case of failure, increment the failures counter and exit if this is the 3rd failure
		case !connSteps.success:
			failed++
			c.Log(fmt.Sprintf("retrying transaction..."))
			time.Sleep(5 * time.Second)
			if failed > 2 {
				return fmt.Errorf("! Connection failed: [%s]client{%s}conn{%s} -> [%s]client{%s}conn{%s}",
					c.ChainID, c.PathEnd.ClientID, c.PathEnd.ConnectionID,
					dst.ChainID, dst.PathEnd.ClientID, dst.PathEnd.ConnectionID)
			}
		}
	}

	return nil
}

// CreateConnectionStep returns the next set of messags for creating a channel
// with the given identifier between chains src and dst. If handshake hasn't started,
// CreateConnetionStep will start the handshake on src
func (c *Chain) CreateConnectionStep(dst *Chain) (*RelayMsgs, error) {
	out := NewRelayMsgs()
	if err := ValidatePaths(c, dst); err != nil {
		return nil, err
	}

	// First, update the light clients to the latest header and return the header
	sh, err := NewSyncHeaders(c, dst)
	if err != nil {
		return nil, err
	}

	// Query a number of things all at once
	var (
		eg                               = new(errgroup.Group)
		srcUpdateHeader, dstUpdateHeader *tmclient.Header
		srcConn, dstConn                 *conntypes.QueryConnectionResponse
		srcCsRes, dstCsRes               *clienttypes.QueryClientStateResponse
		srcCS, dstCS                     ibcexported.ClientState
		srcCons, dstCons                 *clienttypes.QueryConsensusStateResponse
		srcConsH, dstConsH               ibcexported.Height
	)

	// create the UpdateHeaders for src and dest Chains
	eg.Go(func() error {
		var err error
		retry.Do(func() error {
			srcUpdateHeader, dstUpdateHeader, err = sh.GetTrustedHeaders(c, dst)
			if err != nil {
				sh.Updates(c, dst)
			}
			return err
		})
		return err
	})

	// Query Connection data from src and dst
	eg.Go(func() error {
		srcConn, dstConn, err = QueryConnectionPair(c, dst, int64(sh.GetHeight(c.ChainID))-1, int64(sh.GetHeight(dst.ChainID))-1)
		return err

	})

	if err = eg.Wait(); err != nil {
		return nil, err
	}

	if !(srcConn.Connection.State == conntypes.UNINITIALIZED && dstConn.Connection.State == conntypes.UNINITIALIZED) {
		// Query client state from each chain's client
		srcCsRes, dstCsRes, err = QueryClientStatePair(c, dst, int64(sh.GetHeight(c.ChainID))-1, int64(sh.GetHeight(dst.ChainID))-1)
		if err != nil && (srcCsRes == nil || dstCsRes == nil) {
			return nil, err
		}
		srcCS, err = clienttypes.UnpackClientState(srcCsRes.ClientState)
		if err != nil {
			return nil, err
		}
		dstCS, err = clienttypes.UnpackClientState(dstCsRes.ClientState)
		if err != nil {
			return nil, err
		}

		// Store the heights
		srcConsH, dstConsH = srcCS.GetLatestHeight(), dstCS.GetLatestHeight()

		// NOTE: We query connection at height - 1 because of the way tendermint returns
		// proofs the commit for height n is contained in the header of height n + 1
		srcCons, dstCons, err = QueryClientConsensusStatePair(
			c, dst, int64(sh.GetHeight(c.ChainID))-1, int64(sh.GetHeight(dst.ChainID))-1, srcConsH, dstConsH)
		if err != nil {
			return nil, err
		}
	}

	switch {
	// Handshake hasn't been started on src or dst, relay `connOpenInit` to src
	case srcConn.Connection.State == conntypes.UNINITIALIZED && dstConn.Connection.State == conntypes.UNINITIALIZED:
		if c.debug {
			logConnectionStates(c, dst, srcConn, dstConn)
		}
		out.Src = append(out.Src,
			c.PathEnd.UpdateClient(dstUpdateHeader, c.MustGetAddress()),
			c.PathEnd.ConnInit(dst.PathEnd, c.MustGetAddress()),
		)

	// Handshake has started on dst (1 stepdone), relay `connOpenTry` and `updateClient` on src
	case srcConn.Connection.State == conntypes.UNINITIALIZED && dstConn.Connection.State == conntypes.INIT:
		if c.debug {
			logConnectionStates(c, dst, srcConn, dstConn)
		}

		out.Src = append(out.Src,
			c.PathEnd.UpdateClient(dstUpdateHeader, c.MustGetAddress()),
			c.PathEnd.ConnTry(dst.PathEnd, dstCsRes, dstConn, dstCons, c.MustGetAddress()),
		)

	// Handshake has started on src (1 step done), relay `connOpenTry` and `updateClient` on dst
	case srcConn.Connection.State == conntypes.INIT && dstConn.Connection.State == conntypes.UNINITIALIZED:
		if dst.debug {
			logConnectionStates(dst, c, dstConn, srcConn)
		}

		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
			dst.PathEnd.ConnTry(c.PathEnd, srcCsRes, srcConn, srcCons, dst.MustGetAddress()),
		)

	// Handshake has started on src end (2 steps done), relay `connOpenAck` and `updateClient` to dst end
	case srcConn.Connection.State == conntypes.TRYOPEN && dstConn.Connection.State == conntypes.INIT:
		if dst.debug {
			logConnectionStates(dst, c, dstConn, srcConn)
		}

		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
			dst.PathEnd.ConnAck(c.PathEnd, srcCsRes, srcConn, srcCons, dst.MustGetAddress()),
		)

	// Handshake has started on dst end (2 steps done), relay `connOpenAck` and `updateClient` to src end
	case srcConn.Connection.State == conntypes.INIT && dstConn.Connection.State == conntypes.TRYOPEN:
		if c.debug {
			logConnectionStates(c, dst, srcConn, dstConn)
		}
		out.Src = append(out.Src,
			c.PathEnd.UpdateClient(dstUpdateHeader, c.MustGetAddress()),
			c.PathEnd.ConnAck(dst.PathEnd, dstCsRes, dstConn, dstCons, c.MustGetAddress()),
		)

	// Handshake has confirmed on dst (3 steps done), relay `connOpenConfirm` and `updateClient` to src end
	case srcConn.Connection.State == conntypes.TRYOPEN && dstConn.Connection.State == conntypes.OPEN:
		if c.debug {
			logConnectionStates(c, dst, srcConn, dstConn)
		}
		out.Src = append(out.Src,
			c.PathEnd.UpdateClient(dstUpdateHeader, c.MustGetAddress()),
			c.PathEnd.ConnConfirm(dstConn, c.MustGetAddress()),
		)
		out.last = true

	// Handshake has confirmed on src (3 steps done), relay `connOpenConfirm` and `updateClient` to dst end
	case srcConn.Connection.State == conntypes.OPEN && dstConn.Connection.State == conntypes.TRYOPEN:
		if dst.debug {
			logConnectionStates(dst, c, dstConn, srcConn)
		}
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
			dst.PathEnd.ConnConfirm(srcConn, dst.MustGetAddress()),
		)
		out.last = true
	}

	return out, nil
}
