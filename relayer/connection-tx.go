package relayer

import (
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	connState "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/exported"
)

// CreateConnection runs the connection creation messages on timeout until they pass
// TODO: add max retries or something to this function
func (src *Chain) CreateConnection(dst *Chain, to time.Duration) error {
	ticker := time.NewTicker(to)
	failed := 0
	for ; true; <-ticker.C {
		connSteps, err := src.CreateConnectionStep(dst)
		if err != nil {
			return err
		}

		if !connSteps.Ready() {
			break
		}

		connSteps.Send(src, dst)

		switch {
		// In the case of success and this being the last transaction
		// debug logging, log created connection and break
		case connSteps.success && connSteps.last:
			if src.debug {
				conns, err := QueryConnectionPair(src, dst, 0, 0)
				if err != nil {
					return err
				}
				logConnectionStates(src, dst, conns)
			}

			src.Log(fmt.Sprintf("★ Connection created: [%s]client{%s}conn{%s} -> [%s]client{%s}conn{%s}",
				src.ChainID, src.PathEnd.ClientID, src.PathEnd.ConnectionID,
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
					src.ChainID, src.PathEnd.ClientID, src.PathEnd.ConnectionID,
					dst.ChainID, dst.PathEnd.ClientID, dst.PathEnd.ConnectionID)
			}
		}
	}

	return nil
}

// CreateConnectionStep returns the next set of messags for creating a channel
// with the given identifier between chains src and dst. If handshake hasn't started,
// CreateConnetionStep will start the handshake on src
func (src *Chain) CreateConnectionStep(dst *Chain) (*RelayMsgs, error) {
	out := &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}, last: false}

	if err := src.PathEnd.Validate(); err != nil {
		return nil, src.ErrCantSetPath(err)
	}

	if err := dst.PathEnd.Validate(); err != nil {
		return nil, dst.ErrCantSetPath(err)
	}

	hs, err := UpdatesWithHeaders(src, dst)
	if err != nil {
		return nil, err
	}

	scid, dcid := src.ChainID, dst.ChainID

	// Query Connection data from src and dst
	// NOTE: We query connection at height - 1 because of the way tendermint returns
	// proofs the commit for height n is contained in the header of height n + 1
	conn, err := QueryConnectionPair(src, dst, hs[scid].Height-1, hs[dcid].Height-1)
	if err != nil {
		return nil, err
	}

	// NOTE: We query connection at height - 1 because of the way tendermint returns
	// proofs the commit for height n is contained in the header of height n + 1
	cs, err := QueryClientStatePair(src, dst)
	if err != nil {
		return nil, err
	}

	// TODO: log these heights or something about client state? debug?
	if cs[scid] == nil || cs[dcid] == nil {
		return nil, err
	}

	// Store the heights
	srcConsH, dstConsH := int64(cs[scid].ClientState.GetLatestHeight()), int64(cs[dcid].ClientState.GetLatestHeight())

	// NOTE: We query connection at height - 1 because of the way tendermint returns
	// proofs the commit for height n is contained in the header of height n + 1
	cons, err := QueryClientConsensusStatePair(src, dst, hs[scid].Height-1, hs[dcid].Height-1, srcConsH, dstConsH)
	if err != nil {
		return nil, err
	}

	switch {
	// Handshake hasn't been started on src or dst, relay `connOpenInit` to src
	case conn[scid].Connection.Connection.State == connState.UNINITIALIZED && conn[dcid].Connection.Connection.State == connState.UNINITIALIZED:
		if src.debug {
			logConnectionStates(src, dst, conn)
		}
		out.Src = append(out.Src, src.PathEnd.ConnInit(dst.PathEnd, src.MustGetAddress()))

	// Handshake has started on dst (1 stepdone), relay `connOpenTry` and `updateClient` on src
	case conn[scid].Connection.Connection.State == connState.UNINITIALIZED && conn[dcid].Connection.Connection.State == connState.INIT:
		if src.debug {
			logConnectionStates(src, dst, conn)
		}
		out.Src = append(out.Src,
			src.PathEnd.UpdateClient(hs[dcid], src.MustGetAddress()),
			src.PathEnd.ConnTry(dst.PathEnd, conn[dcid], cons[dcid], dstConsH, src.MustGetAddress()),
		)

	// Handshake has started on src (1 step done), relay `connOpenTry` and `updateClient` on dst
	case conn[scid].Connection.Connection.State == connState.INIT && conn[dcid].Connection.Connection.State == connState.UNINITIALIZED:
		if dst.debug {
			logConnectionStates(dst, src, conn)
		}
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(hs[scid], dst.MustGetAddress()),
			dst.PathEnd.ConnTry(src.PathEnd, conn[scid], cons[scid], srcConsH, dst.MustGetAddress()),
		)

	// Handshake has started on src end (2 steps done), relay `connOpenAck` and `updateClient` to dst end
	case conn[scid].Connection.Connection.State == connState.TRYOPEN && conn[dcid].Connection.Connection.State == connState.INIT:
		if dst.debug {
			logConnectionStates(dst, src, conn)
		}
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(hs[scid], dst.MustGetAddress()),
			dst.PathEnd.ConnAck(conn[scid], cons[scid], srcConsH, dst.MustGetAddress()),
		)

	// Handshake has started on dst end (2 steps done), relay `connOpenAck` and `updateClient` to src end
	case conn[scid].Connection.Connection.State == connState.INIT && conn[dcid].Connection.Connection.State == connState.TRYOPEN:
		if src.debug {
			logConnectionStates(src, dst, conn)
		}
		out.Src = append(out.Src,
			src.PathEnd.UpdateClient(hs[dcid], src.MustGetAddress()),
			src.PathEnd.ConnAck(conn[dcid], cons[dcid], dstConsH, src.MustGetAddress()),
		)

	// Handshake has confirmed on dst (3 steps done), relay `connOpenConfirm` and `updateClient` to src end
	case conn[scid].Connection.Connection.State == connState.TRYOPEN && conn[dcid].Connection.Connection.State == connState.OPEN:
		if src.debug {
			logConnectionStates(src, dst, conn)
		}
		out.Src = append(out.Src,
			src.PathEnd.UpdateClient(hs[dcid], src.MustGetAddress()),
			src.PathEnd.ConnConfirm(conn[dcid], src.MustGetAddress()),
		)
		out.last = true

	// Handshake has confirmed on src (3 steps done), relay `connOpenConfirm` and `updateClient` to dst end
	case conn[scid].Connection.Connection.State == connState.OPEN && conn[dcid].Connection.Connection.State == connState.TRYOPEN:
		if dst.debug {
			logConnectionStates(dst, src, conn)
		}
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(hs[scid], dst.MustGetAddress()),
			dst.PathEnd.ConnConfirm(conn[scid], dst.MustGetAddress()),
		)
		out.last = true
	}

	return out, nil
}
