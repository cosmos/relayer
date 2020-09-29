package relayer

import (
	"fmt"
	"time"

	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint/types"
	"golang.org/x/sync/errgroup"
)

// CreateChannel runs the channel creation messages on timeout until they pass
// TODO: add max retries or something to this function
func (c *Chain) CreateChannel(dst *Chain, ordered bool, to time.Duration) error {
	var order chanTypes.Order
	if ordered {
		order = chanTypes.ORDERED
	} else {
		order = chanTypes.UNORDERED
	}

	ticker := time.NewTicker(to)
	failures := 0
	for ; true; <-ticker.C {
		chanSteps, err := c.CreateChannelStep(dst, order)
		if err != nil {
			return err
		}

		if !chanSteps.Ready() {
			break
		}

		chanSteps.Send(c, dst)

		switch {
		// In the case of success and this being the last transaction
		// debug logging, log created connection and break
		case chanSteps.success && chanSteps.last:
			srch, dsth, err := GetLatestLightHeights(c, dst)
			if err != nil {
				return err
			}
			srcChan, dstChan, err := QueryChannelPair(c, dst, srch, dsth)
			if err != nil {
				return err
			}
			if c.debug {
				logChannelStates(c, dst, srcChan, dstChan)
			}
			c.Log(fmt.Sprintf("★ Channel created: [%s]chan{%s}port{%s} -> [%s]chan{%s}port{%s}",
				c.ChainID, c.PathEnd.ChannelID, c.PathEnd.PortID,
				dst.ChainID, dst.PathEnd.ChannelID, dst.PathEnd.PortID))
			return nil
		// In the case of success, reset the failures counter
		case chanSteps.success:
			failures = 0
			continue
		// In the case of failure, increment the failures counter and exit if this is the 3rd failure
		case !chanSteps.success:
			failures++
			if failures > 2 {
				return fmt.Errorf("! Channel failed: [%s]chan{%s}port{%s} -> [%s]chan{%s}port{%s}",
					c.ChainID, c.PathEnd.ChannelID, c.PathEnd.PortID,
					dst.ChainID, dst.PathEnd.ChannelID, dst.PathEnd.PortID)
			}
		}
	}

	return nil
}

// CreateChannelStep returns the next set of messages for creating a channel with given
// identifiers between chains src and dst. If the handshake hasn't started, then CreateChannelStep
// will begin the handshake on the src chain
func (c *Chain) CreateChannelStep(dst *Chain, ordering chanTypes.Order) (*RelayMsgs, error) {
	out := NewRelayMsgs()
	if err := ValidatePaths(c, dst); err != nil {
		return nil, err
	}

	sh, err := NewSyncHeaders(c, dst)
	if err != nil {
		return nil, err
	}

	// Query a number of things all at once
	var (
		eg                               = new(errgroup.Group)
		srcUpdateHeader, dstUpdateHeader *tmclient.Header
		srcChan, dstChan                 *chanTypes.QueryChannelResponse
	)

	eg.Go(func() error {
		srcUpdateHeader, dstUpdateHeader, err = sh.GetTrustedHeaders(c, dst)
		return err
	})

	eg.Go(func() error {
		srcChan, dstChan, err = QueryChannelPair(c, dst, int64(sh.GetHeight(c.ChainID))-1, int64(sh.GetHeight(dst.ChainID))-1)
		return err
	})

	if err = eg.Wait(); err != nil {
		return nil, err
	}

	switch {
	// Handshake hasn't been started on src or dst, relay `chanOpenInit` to src
	case srcChan.Channel.State == chanTypes.UNINITIALIZED && dstChan.Channel.State == chanTypes.UNINITIALIZED:
		if c.debug {
			logChannelStates(c, dst, srcChan, dstChan)
		}
		out.Src = append(out.Src,
			c.PathEnd.ChanInit(dst.PathEnd, c.MustGetAddress()),
		)

	// Handshake has started on dst (1 step done), relay `chanOpenTry` and `updateClient` to src
	case srcChan.Channel.State == chanTypes.UNINITIALIZED && dstChan.Channel.State == chanTypes.INIT:
		if c.debug {
			logChannelStates(c, dst, srcChan, dstChan)
		}
		out.Src = append(out.Src,
			c.PathEnd.UpdateClient(dstUpdateHeader, c.MustGetAddress()),
			c.PathEnd.ChanTry(dst.PathEnd, dstChan, c.MustGetAddress()),
		)

	// Handshake has started on src (1 step done), relay `chanOpenTry` and `updateClient` to dst
	case srcChan.Channel.State == chanTypes.INIT && dstChan.Channel.State == chanTypes.UNINITIALIZED:
		if dst.debug {
			logChannelStates(dst, c, dstChan, srcChan)
		}
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
			dst.PathEnd.ChanTry(c.PathEnd, srcChan, dst.MustGetAddress()),
		)

	// Handshake has started on src (2 steps done), relay `chanOpenAck` and `updateClient` to dst
	case srcChan.Channel.State == chanTypes.TRYOPEN && dstChan.Channel.State == chanTypes.INIT:
		if dst.debug {
			logChannelStates(dst, c, dstChan, srcChan)
		}
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
			dst.PathEnd.ChanAck(srcChan, dst.MustGetAddress()),
		)

	// Handshake has started on dst (2 steps done), relay `chanOpenAck` and `updateClient` to src
	case srcChan.Channel.State == chanTypes.INIT && dstChan.Channel.State == chanTypes.TRYOPEN:
		if c.debug {
			logChannelStates(c, dst, srcChan, dstChan)
		}
		out.Src = append(out.Src,
			c.PathEnd.UpdateClient(dstUpdateHeader, c.MustGetAddress()),
			c.PathEnd.ChanAck(dstChan, c.MustGetAddress()),
		)

	// Handshake has confirmed on dst (3 steps done), relay `chanOpenConfirm` and `updateClient` to src
	case srcChan.Channel.State == chanTypes.TRYOPEN && dstChan.Channel.State == chanTypes.OPEN:
		if c.debug {
			logChannelStates(c, dst, srcChan, dstChan)
		}
		out.Src = append(out.Src,
			c.PathEnd.UpdateClient(dstUpdateHeader, c.MustGetAddress()),
			c.PathEnd.ChanConfirm(dstChan, c.MustGetAddress()),
		)
		out.last = true

	// Handshake has confirmed on src (3 steps done), relay `chanOpenConfirm` and `updateClient` to dst
	case srcChan.Channel.State == chanTypes.OPEN && dstChan.Channel.State == chanTypes.TRYOPEN:
		if dst.debug {
			logChannelStates(dst, c, dstChan, srcChan)
		}
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
			dst.PathEnd.ChanConfirm(srcChan, dst.MustGetAddress()),
		)
		out.last = true
	}

	return out, nil
}

// CloseChannel runs the channel closing messages on timeout until they pass
// TODO: add max retries or something to this function
func (c *Chain) CloseChannel(dst *Chain, to time.Duration) error {

	ticker := time.NewTicker(to)
	for ; true; <-ticker.C {
		closeSteps, err := c.CloseChannelStep(dst)
		if err != nil {
			return err
		}

		if !closeSteps.Ready() {
			break
		}

		if closeSteps.Send(c, dst); closeSteps.success && closeSteps.last {
			srcChan, dstChan, err := QueryChannelPair(c, dst, 0, 0)
			if err != nil {
				return err
			}
			if c.debug {
				logChannelStates(c, dst, srcChan, dstChan)
			}
			c.Log(fmt.Sprintf("★ Closed channel between [%s]chan{%s}port{%s} -> [%s]chan{%s}port{%s}",
				c.ChainID, c.PathEnd.ChannelID, c.PathEnd.PortID,
				dst.ChainID, dst.PathEnd.ChannelID, dst.PathEnd.PortID))
			break
		}
	}
	return nil
}

// CloseChannelStep returns the next set of messages for closing a channel with given
// identifiers between chains src and dst. If the closing handshake hasn't started, then CloseChannelStep
// will begin the handshake on the src chain
func (c *Chain) CloseChannelStep(dst *Chain) (*RelayMsgs, error) {
	out := NewRelayMsgs()
	if err := ValidatePaths(c, dst); err != nil {
		return nil, err
	}

	sh, err := NewSyncHeaders(c, dst)
	if err != nil {
		return nil, err
	}

	// Query a number of things all at once
	var (
		eg                               = new(errgroup.Group)
		srcUpdateHeader, dstUpdateHeader *tmclient.Header
		srcChan, dstChan                 *chanTypes.QueryChannelResponse
	)

	eg.Go(func() error {
		// create the UpdateHeaders for src and dest Chains
		srcUpdateHeader, dstUpdateHeader, err = sh.GetTrustedHeaders(c, dst)
		return err
	})

	eg.Go(func() error {
		srcChan, dstChan, err = QueryChannelPair(c, dst, int64(sh.GetHeight(c.ChainID)), int64(sh.GetHeight(dst.ChainID)))
		return err
	})

	if err = eg.Wait(); err != nil {
		return nil, err
	}

	logChannelStates(c, dst, srcChan, dstChan)

	switch {
	// Closing handshake has not started, relay `updateClient` and `chanCloseInit` to src or dst according
	// to the channel state
	case srcChan.Channel.State != chanTypes.CLOSED && dstChan.Channel.State != chanTypes.CLOSED:
		if srcChan.Channel.State != chanTypes.UNINITIALIZED {
			if c.debug {
				logChannelStates(c, dst, srcChan, dstChan)
			}
			out.Src = append(out.Src,
				c.PathEnd.UpdateClient(dstUpdateHeader, c.MustGetAddress()),
				c.PathEnd.ChanCloseInit(c.MustGetAddress()),
			)
		} else if dstChan.Channel.State != chanTypes.UNINITIALIZED {
			if dst.debug {
				logChannelStates(dst, c, dstChan, srcChan)
			}
			out.Dst = append(out.Dst,
				dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
				dst.PathEnd.ChanCloseInit(dst.MustGetAddress()),
			)
		}

	// Closing handshake has started on src, relay `updateClient` and `chanCloseConfirm` to dst
	case srcChan.Channel.State == chanTypes.CLOSED && dstChan.Channel.State != chanTypes.CLOSED:
		if dstChan.Channel.State != chanTypes.UNINITIALIZED {
			if dst.debug {
				logChannelStates(dst, c, dstChan, srcChan)
			}
			out.Dst = append(out.Dst,
				dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
				dst.PathEnd.ChanCloseConfirm(srcChan, dst.MustGetAddress()),
			)
			out.last = true
		}

	// Closing handshake has started on dst, relay `updateClient` and `chanCloseConfirm` to src
	case dstChan.Channel.State == chanTypes.CLOSED && srcChan.Channel.State != chanTypes.CLOSED:
		if srcChan.Channel.State != chanTypes.UNINITIALIZED {
			if c.debug {
				logChannelStates(c, dst, srcChan, dstChan)
			}
			out.Src = append(out.Src,
				c.PathEnd.UpdateClient(dstUpdateHeader, c.MustGetAddress()),
				c.PathEnd.ChanCloseConfirm(dstChan, c.MustGetAddress()),
			)
			out.last = true
		}
	}
	return out, nil
}
