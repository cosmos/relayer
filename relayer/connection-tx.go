package relayer

import (
	"fmt"
	"time"

	retry "github.com/avast/retry-go"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/types"
	conntypes "github.com/cosmos/cosmos-sdk/x/ibc/core/03-connection/types"
	ibcexported "github.com/cosmos/cosmos-sdk/x/ibc/core/exported"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
)

// CreateConnection runs the connection creation messages on timeout until they pass
// TODO: add max retries or something to this function
func (c *Chain) CreateConnection(dst *Chain, to time.Duration) error {
	ticker := time.NewTicker(to)
	failed := 0
	for ; true; <-ticker.C {
		success, lastStep, err := c.ExecuteConnectionStep(dst)
		if err != nil {
			return err
		}

		switch {
		// In the case of success and this being the last transaction
		// debug logging, log created connection and break
		case success && lastStep:
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

		// reset the failures counter
		case success:
			failed = 0
			continue

		// increment the failures counter and exit if this is the 3rd failure
		case !success:
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

// ExecuteConnectionStep executes the next connection step based on the
// states of two connection ends specified by the relayer configuration
// file. The booleans return indicate if the message was successfully
// executed and if this was the last handshake step.
func (c *Chain) ExecuteConnectionStep(dst *Chain) (bool, bool, error) {
	// client identifiers must be filled in
	if err := ValidatePaths(c, dst); err != nil {
		return false, false, err
	}

	// update the off chain light clients to the latest header and return the header
	sh, err := NewSyncHeaders(c, dst)
	if err != nil {
		return false, false, err
	}

	// variables needed to determine the current handshake step
	var (
		srcUpdateHeader, dstUpdateHeader *tmclient.Header
		srcConn, dstConn                 *conntypes.QueryConnectionResponse
		srcCsRes, dstCsRes               *clienttypes.QueryClientStateResponse
		srcCS, dstCS                     ibcexported.ClientState
		srcCons, dstCons                 *clienttypes.QueryConsensusStateResponse
		srcConsH, dstConsH               ibcexported.Height
	)

	// create a go routine to construct update headers to update the on chain light clients
	if err := retry.Do(func() error {
		srcUpdateHeader, dstUpdateHeader, err = sh.GetTrustedHeaders(c, dst)
		return err
	}, rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
		// callback for each retry attempt
		logRetryUpdateHeaders(c, dst, n, err)
		sh.Updates(c, dst)
	})); err != nil {
		return false, false, err
	}

	// Query Connection data from src and dst
	srcConn, dstConn, err = QueryConnectionPair(c, dst, int64(sh.GetHeight(c.ChainID))-1, int64(sh.GetHeight(dst.ChainID))-1)
	if err != nil {
		return false, false, err
	}

	/*	// if the connection already exists on both chains
		if !(srcConn.Connection.State == conntypes.UNINITIALIZED && dstConn.Connection.State == conntypes.UNINITIALIZED) {
			// Query client state from each chain's client
			srcCsRes, dstCsRes, err = QueryClientStatePair(c, dst, int64(sh.GetHeight(c.ChainID))-1, int64(sh.GetHeight(dst.ChainID))-1)
			if err != nil && (srcCsRes == nil || dstCsRes == nil) {
				return false, false, err
			}
			srcCS, err = clienttypes.UnpackClientState(srcCsRes.ClientState)
			if err != nil {
				return false, false, err
			}
			dstCS, err = clienttypes.UnpackClientState(dstCsRes.ClientState)
			if err != nil {
				return false, false, err
			}

			// Store the heights
			srcConsH, dstConsH = srcCS.GetLatestHeight(), dstCS.GetLatestHeight()

			// NOTE: We query connection at height - 1 because of the way tendermint returns
			// proofs the commit for height n is contained in the header of height n + 1
			srcCons, dstCons, err = QueryClientConsensusStatePair(
				c, dst, int64(sh.GetHeight(c.ChainID))-1, int64(sh.GetHeight(dst.ChainID))-1, srcConsH, dstConsH)
			if err != nil {
				return false, false, err
			}
		}
	*/
	// TODO: Query for existing identifier and fill config

	switch {

	// OpenInit on source
	// Neither connection has been initialized
	case c.PathEnd.ConnectionID == "" && dst.PathEnd.ConnectionID == "":
		if c.debug {
			// TODO: log that we are attempting to create new connection ends
		}

		// cosntruct OpenInit message to be submitted on source chain
		msgs := []sdk.Msg{
			c.PathEnd.UpdateClient(dstUpdateHeader, c.MustGetAddress()),
			c.PathEnd.ConnInit(dst.PathEnd, c.MustGetAddress()),
		}

		// TODO: with the introduction of typed events, we can abstract sending
		// and event parsing to the bottom of this function. Until then it is
		// easier to parse events if we know exactly what message we are parsing.
		res, success, err := c.SendMsgs(msgs)
		if !success {
			return false, false, err
		}

		// update connection identifier in PathEnd
		c.HandleOpenInitEvents(res)

	// OpenTry on source
	// source connection does not exist, but counterparty connection exists
	case c.PathEnd.ConnectionID == "" && dst.PathEnd.ConnectionID != "":
		if c.debug {
			// TODO: update logging
		}

		msgs := []sdk.Msg{
			c.PathEnd.UpdateClient(dstUpdateHeader, c.MustGetAddress()),
			c.PathEnd.ConnTry(dst.PathEnd, dstCsRes, dstConn, dstCons, c.MustGetAddress()),
		}
		res, success, err := c.SendMsgs(msgs)
		if !success {
			return false, false, err
		}

		// update connection identifier in PathEnd
		c.HandleOpenTryEvents(res)

	// OpenTry on counterparty
	// source connection exists, but counterparty connection does not exist
	case c.PathEnd.ConnectionID != "" && dst.PathEnd.ConnectionID == "":
		if dst.debug {
			// TODO: update logging
		}

		msgs := []sdk.Msgs{
			dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
			dst.PathEnd.ConnTry(c.PathEnd, srcCsRes, srcConn, srcCons, dst.MustGetAddress()),
		}
		res, success, err := dst.SendMsgs(msgs)
		if !success {
			return false, false, err
		}

		// update connection identifier in PathEnd
		dst.HandleOpenTryEvents(res)

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
		out.Last = true

	// Handshake has confirmed on src (3 steps done), relay `connOpenConfirm` and `updateClient` to dst end
	case srcConn.Connection.State == conntypes.OPEN && dstConn.Connection.State == conntypes.TRYOPEN:
		if dst.debug {
			logConnectionStates(dst, c, dstConn, srcConn)
		}
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
			dst.PathEnd.ConnConfirm(srcConn, dst.MustGetAddress()),
		)
		out.Last = true
	}

	return out, nil
}
