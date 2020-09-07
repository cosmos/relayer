package relayer

import (
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clientTypes "github.com/cosmos/cosmos-sdk/x/ibc/02-client/types"
	ibctypes "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/types"
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
				conns, err := QueryConnectionPair(c, dst, 0, 0)
				if err != nil {
					return err
				}
				logConnectionStates(c, dst, conns)
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
	out := &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}, last: false}

	if err := c.PathEnd.Validate(); err != nil {
		return nil, c.ErrCantSetPath(err)
	}

	if err := dst.PathEnd.Validate(); err != nil {
		return nil, dst.ErrCantSetPath(err)
	}

	hs, err := UpdatesWithHeaders(c, dst)
	if err != nil {
		return nil, err
	}

	scid, dcid := c.ChainID, dst.ChainID

	// create the UpdateHeaders for src and dest Chains
	srcUpdateHeader, err := InjectTrustedFields(c, dst, hs[scid])
	if err != nil {
		fmt.Printf("%#v\n", dst.PathEnd)
		fmt.Println("SRC")
		fmt.Println(scid)
		return nil, err
	}
	dstUpdateHeader, err := InjectTrustedFields(dst, c, hs[dcid])
	if err != nil {
		fmt.Println("DST")
		return nil, err
	}

	// Query Connection data from src and dst
	// NOTE: We query connection at height - 1 because of the way tendermint returns
	// proofs the commit for height n is contained in the header of height n + 1
	conn, err := QueryConnectionPair(c, dst, hs[scid].Header.Height-1, hs[dcid].Header.Height-1)
	if err != nil {
		return nil, err
	}

	// NOTE: We query connection at height - 1 because of the way tendermint returns
	// proofs the commit for height n is contained in the header of height n + 1
	cs, err := QueryClientStatePair(c, dst)
	if err != nil {
		return nil, err
	}

	// TODO: log these heights or something about client state? debug?
	if cs[scid] == nil || cs[dcid] == nil {
		return nil, err
	}

	srcCS, err := clientTypes.UnpackClientState(cs[scid].ClientState)
	if err != nil {
		return nil, err
	}
	dstCS, err := clientTypes.UnpackClientState(cs[dcid].ClientState)
	if err != nil {
		return nil, err
	}

	// Store the heights
	srcConsH, dstConsH := int64(MustGetHeight(srcCS.GetLatestHeight())), int64(MustGetHeight(dstCS.GetLatestHeight()))

	// NOTE: We query connection at height - 1 because of the way tendermint returns
	// proofs the commit for height n is contained in the header of height n + 1
	cons, err := QueryClientConsensusStatePair(c, dst, hs[scid].Header.Height-1, hs[dcid].Header.Height-1, srcConsH, dstConsH)
	if err != nil {
		return nil, err
	}

	switch {
	// Handshake hasn't been started on src or dst, relay `connOpenInit` to src
	case conn[scid].Connection.State == ibctypes.UNINITIALIZED && conn[dcid].Connection.State == ibctypes.UNINITIALIZED:
		if c.debug {
			logConnectionStates(c, dst, conn)
		}
		out.Src = append(out.Src, c.PathEnd.ConnInit(dst.PathEnd, c.MustGetAddress()))

	// Handshake has started on dst (1 stepdone), relay `connOpenTry` and `updateClient` on src
	case conn[scid].Connection.State == ibctypes.UNINITIALIZED && conn[dcid].Connection.State == ibctypes.INIT:
		if c.debug {
			logConnectionStates(c, dst, conn)
		}
		out.Src = append(out.Src,
			c.PathEnd.UpdateClient(dstUpdateHeader, c.MustGetAddress()),
			c.PathEnd.ConnTry(dst.PathEnd, cs[dcid], conn[dcid], cons[dcid], c.MustGetAddress()),
		)

	// Handshake has started on src (1 step done), relay `connOpenTry` and `updateClient` on dst
	case conn[scid].Connection.State == ibctypes.INIT && conn[dcid].Connection.State == ibctypes.UNINITIALIZED:
		if dst.debug {
			logConnectionStates(dst, c, conn)
		}
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
			dst.PathEnd.ConnTry(c.PathEnd, cs[scid], conn[scid], cons[scid], dst.MustGetAddress()),
		)

	// Handshake has started on src end (2 steps done), relay `connOpenAck` and `updateClient` to dst end
	case conn[scid].Connection.State == ibctypes.TRYOPEN && conn[dcid].Connection.State == ibctypes.INIT:
		if dst.debug {
			logConnectionStates(dst, c, conn)
		}
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
			dst.PathEnd.ConnAck(c.PathEnd, cs[scid], conn[scid], cons[scid], dst.MustGetAddress()),
		)

	// Handshake has started on dst end (2 steps done), relay `connOpenAck` and `updateClient` to src end
	case conn[scid].Connection.State == ibctypes.INIT && conn[dcid].Connection.State == ibctypes.TRYOPEN:
		if c.debug {
			logConnectionStates(c, dst, conn)
		}
		out.Src = append(out.Src,
			c.PathEnd.UpdateClient(dstUpdateHeader, c.MustGetAddress()),
			c.PathEnd.ConnAck(dst.PathEnd, cs[dcid], conn[dcid], cons[dcid], c.MustGetAddress()),
		)

	// Handshake has confirmed on dst (3 steps done), relay `connOpenConfirm` and `updateClient` to src end
	case conn[scid].Connection.State == ibctypes.TRYOPEN && conn[dcid].Connection.State == ibctypes.OPEN:
		if c.debug {
			logConnectionStates(c, dst, conn)
		}
		out.Src = append(out.Src,
			c.PathEnd.UpdateClient(dstUpdateHeader, c.MustGetAddress()),
			c.PathEnd.ConnConfirm(conn[dcid], c.MustGetAddress()),
		)
		out.last = true

	// Handshake has confirmed on src (3 steps done), relay `connOpenConfirm` and `updateClient` to dst end
	case conn[scid].Connection.State == ibctypes.OPEN && conn[dcid].Connection.State == ibctypes.TRYOPEN:
		if dst.debug {
			logConnectionStates(dst, c, conn)
		}
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
			dst.PathEnd.ConnConfirm(conn[scid], dst.MustGetAddress()),
		)
		out.last = true
	}

	return out, nil
}
