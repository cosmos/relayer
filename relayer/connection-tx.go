package relayer

import (
	"fmt"
	"time"

	clientTypes "github.com/cosmos/cosmos-sdk/x/ibc/02-client/types"
	connTypes "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/types"
	ibctypes "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint/types"
	ibcExported "github.com/cosmos/cosmos-sdk/x/ibc/exported"
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
				srcConn, dstConn, err := QueryConnectionPair(c, dst, 0, 0)
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
	srch, dsth, err := UpdatesWithHeaders(c, dst)
	if err != nil {
		return nil, err
	}

	// Query a number of things all at once
	var (
		eg                               = new(errgroup.Group)
		srcUpdateHeader, dstUpdateHeader *tmclient.Header
		srcConn, dstConn                 *connTypes.QueryConnectionResponse
		srcCsRes, dstCsRes               *clientTypes.QueryClientStateResponse
		srcCS, dstCS                     ibcExported.ClientState
	)

	// create the UpdateHeaders for src and dest Chains
	eg.Go(func() error {
		srcUpdateHeader, dstUpdateHeader, err = InjectTrustedFieldsHeaders(c, dst, srch, dsth)
		return err
	})

	// Query Connection data from src and dst
	eg.Go(func() error {
		srcConn, dstConn, err = QueryConnectionPair(c, dst, srch.Header.Height, dsth.Header.Height)
		return err

	})
	// Query client state from each chain's client
	eg.Go(func() error {
		srcCsRes, dstCsRes, err = QueryClientStatePair(c, dst, srch.Header.Height, dsth.Header.Height)
		if err != nil && (srcCsRes == nil || dstCsRes == nil) {
			return err
		}
		srcCS, err = clientTypes.UnpackClientState(srcCsRes.ClientState)
		if err != nil {
			return err
		}
		dstCS, err = clientTypes.UnpackClientState(dstCsRes.ClientState)
		return err
	})

	if err = eg.Wait(); err != nil {
		return nil, err
	}

	// Store the heights
	srcConsH, dstConsH := int64(MustGetHeight(srcCS.GetLatestHeight())), int64(MustGetHeight(dstCS.GetLatestHeight()))

	// NOTE: We query connection at height - 1 because of the way tendermint returns
	// proofs the commit for height n is contained in the header of height n + 1
	srcCons, dstCons, err := QueryClientConsensusStatePair(c, dst, srch.Header.Height, dsth.Header.Height, srcConsH, dstConsH)
	if err != nil {
		return nil, err
	}

	fmt.Println("src header", srch.Header.Height)
	fmt.Println("src trusted header", srcUpdateHeader.TrustedHeight)
	fmt.Println("dst header", dsth.Header.Height)
	fmt.Println("dst trusted header", dstUpdateHeader.TrustedHeight)
	fmt.Println("src conn proof height", srcConn.ProofHeight)
	fmt.Println("dst conn proof height", dstConn.ProofHeight)
	fmt.Println("src client state proof height", srcCsRes.ProofHeight)
	fmt.Println("src client state height", srcConsH)
	fmt.Println("dst client state proof height", dstCsRes.ProofHeight)
	fmt.Println("dst client state height", dstConsH)
	fmt.Println("src client consensus state proof height", srcCons.ProofHeight)
	fmt.Println("dst client consensus state proof height", dstCons.ProofHeight)

	switch {
	// Handshake hasn't been started on src or dst, relay `connOpenInit` to src
	case srcConn.Connection.State == ibctypes.UNINITIALIZED && dstConn.Connection.State == ibctypes.UNINITIALIZED:
		if c.debug {
			logConnectionStates(c, dst, srcConn, dstConn)
		}
		out.Src = append(out.Src, c.PathEnd.ConnInit(dst.PathEnd, c.MustGetAddress()))

	// Handshake has started on dst (1 stepdone), relay `connOpenTry` and `updateClient` on src
	case srcConn.Connection.State == ibctypes.UNINITIALIZED && dstConn.Connection.State == ibctypes.INIT:
		if c.debug {
			logConnectionStates(c, dst, srcConn, dstConn)
		}
		out.Src = append(out.Src,
			c.PathEnd.UpdateClient(dstUpdateHeader, c.MustGetAddress()),
			c.PathEnd.ConnTry(dst.PathEnd, dstCsRes, dstConn, dstCons, c.MustGetAddress()),
		)

	// Handshake has started on src (1 step done), relay `connOpenTry` and `updateClient` on dst
	case srcConn.Connection.State == ibctypes.INIT && dstConn.Connection.State == ibctypes.UNINITIALIZED:
		if dst.debug {
			logConnectionStates(dst, c, dstConn, srcConn)
		}
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
			dst.PathEnd.ConnTry(c.PathEnd, srcCsRes, srcConn, srcCons, dst.MustGetAddress()),
		)

	// Handshake has started on src end (2 steps done), relay `connOpenAck` and `updateClient` to dst end
	case srcConn.Connection.State == ibctypes.TRYOPEN && dstConn.Connection.State == ibctypes.INIT:
		if dst.debug {
			logConnectionStates(dst, c, dstConn, srcConn)
		}
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
			dst.PathEnd.ConnAck(c.PathEnd, srcCsRes, srcConn, srcCons, dst.MustGetAddress()),
		)

	// Handshake has started on dst end (2 steps done), relay `connOpenAck` and `updateClient` to src end
	case srcConn.Connection.State == ibctypes.INIT && dstConn.Connection.State == ibctypes.TRYOPEN:
		if c.debug {
			logConnectionStates(c, dst, srcConn, dstConn)
		}
		out.Src = append(out.Src,
			c.PathEnd.UpdateClient(dstUpdateHeader, c.MustGetAddress()),
			c.PathEnd.ConnAck(dst.PathEnd, dstCsRes, dstConn, dstCons, c.MustGetAddress()),
		)

	// Handshake has confirmed on dst (3 steps done), relay `connOpenConfirm` and `updateClient` to src end
	case srcConn.Connection.State == ibctypes.TRYOPEN && dstConn.Connection.State == ibctypes.OPEN:
		if c.debug {
			logConnectionStates(c, dst, srcConn, dstConn)
		}
		out.Src = append(out.Src,
			c.PathEnd.UpdateClient(dstUpdateHeader, c.MustGetAddress()),
			c.PathEnd.ConnConfirm(dstConn, c.MustGetAddress()),
		)
		out.last = true

	// Handshake has confirmed on src (3 steps done), relay `connOpenConfirm` and `updateClient` to dst end
	case srcConn.Connection.State == ibctypes.OPEN && dstConn.Connection.State == ibctypes.TRYOPEN:
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
