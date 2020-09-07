package relayer

import (
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	chantypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
)

// CreateChannel runs the channel creation messages on timeout until they pass
// TODO: add max retries or something to this function
func (c *Chain) CreateChannel(dst *Chain, ordered bool, to time.Duration) error {
	var order chantypes.Order
	if ordered {
		order = chantypes.ORDERED
	} else {
		order = chantypes.UNORDERED
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
			chans, err := QueryChannelPair(c, dst, 0, 0)
			if err != nil {
				return err
			}
			if c.debug {
				logChannelStates(c, dst, chans)
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
func (c *Chain) CreateChannelStep(dst *Chain, ordering chantypes.Order) (*RelayMsgs, error) {
	var (
		out        = &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}, last: false}
		scid, dcid = c.ChainID, dst.ChainID
	)

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
	// create the UpdateHeaders for src and dest Chains
	srcUpdateHeader, err := InjectTrustedFields(c, dst, hs[scid])
	if err != nil {
		return nil, err
	}
	dstUpdateHeader, err := InjectTrustedFields(dst, c, hs[dcid])
	if err != nil {
		return nil, err
	}

	chans, err := QueryChannelPair(c, dst, hs[scid].Header.Height-1, hs[dcid].Header.Height-1)
	if err != nil {
		return nil, err
	}

	switch {
	// Handshake hasn't been started on src or dst, relay `chanOpenInit` to src
	case chans[scid].Channel.State == chantypes.UNINITIALIZED && chans[dcid].Channel.State == chantypes.UNINITIALIZED:
		if c.debug {
			logChannelStates(c, dst, chans)
		}
		out.Src = append(out.Src,
			c.PathEnd.ChanInit(dst.PathEnd, c.MustGetAddress()),
		)

	// Handshake has started on dst (1 step done), relay `chanOpenTry` and `updateClient` to src
	case chans[scid].Channel.State == chantypes.UNINITIALIZED && chans[dcid].Channel.State == chantypes.INIT:
		if c.debug {
			logChannelStates(c, dst, chans)
		}
		out.Src = append(out.Src,
			c.PathEnd.UpdateClient(dstUpdateHeader, c.MustGetAddress()),
			c.PathEnd.ChanTry(dst.PathEnd, chans[dcid], c.MustGetAddress()),
		)

	// Handshake has started on src (1 step done), relay `chanOpenTry` and `updateClient` to dst
	case chans[scid].Channel.State == chantypes.INIT && chans[dcid].Channel.State == chantypes.UNINITIALIZED:
		if dst.debug {
			logChannelStates(dst, c, chans)
		}
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
			dst.PathEnd.ChanTry(c.PathEnd, chans[scid], dst.MustGetAddress()),
		)

	// Handshake has started on src (2 steps done), relay `chanOpenAck` and `updateClient` to dst
	case chans[scid].Channel.State == chantypes.TRYOPEN && chans[dcid].Channel.State == chantypes.INIT:
		if dst.debug {
			logChannelStates(dst, c, chans)
		}
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
			dst.PathEnd.ChanAck(chans[scid], dst.MustGetAddress()),
		)

	// Handshake has started on dst (2 steps done), relay `chanOpenAck` and `updateClient` to src
	case chans[scid].Channel.State == chantypes.INIT && chans[dcid].Channel.State == chantypes.TRYOPEN:
		if c.debug {
			logChannelStates(c, dst, chans)
		}
		out.Src = append(out.Src,
			c.PathEnd.UpdateClient(dstUpdateHeader, c.MustGetAddress()),
			c.PathEnd.ChanAck(chans[dcid], c.MustGetAddress()),
		)

	// Handshake has confirmed on dst (3 steps done), relay `chanOpenConfirm` and `updateClient` to src
	case chans[scid].Channel.State == chantypes.TRYOPEN && chans[dcid].Channel.State == chantypes.OPEN:
		if c.debug {
			logChannelStates(c, dst, chans)
		}
		out.Src = append(out.Src,
			c.PathEnd.UpdateClient(dstUpdateHeader, c.MustGetAddress()),
			c.PathEnd.ChanConfirm(chans[dcid], c.MustGetAddress()),
		)
		out.last = true

	// Handshake has confirmed on src (3 steps done), relay `chanOpenConfirm` and `updateClient` to dst
	case chans[scid].Channel.State == chantypes.OPEN && chans[dcid].Channel.State == chantypes.TRYOPEN:
		if dst.debug {
			logChannelStates(dst, c, chans)
		}
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
			dst.PathEnd.ChanConfirm(chans[scid], dst.MustGetAddress()),
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
			chans, err := QueryChannelPair(c, dst, 0, 0)
			if err != nil {
				return err
			}
			if c.debug {
				logChannelStates(c, dst, chans)
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
	var (
		out        = &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}, last: false}
		scid, dcid = c.ChainID, dst.ChainID
	)

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
	// create the UpdateHeaders for src and dest Chains
	srcUpdateHeader, err := InjectTrustedFields(c, dst, hs[scid])
	if err != nil {
		return nil, err
	}
	dstUpdateHeader, err := InjectTrustedFields(dst, c, hs[dcid])
	if err != nil {
		return nil, err
	}

	chans, err := QueryChannelPair(c, dst, hs[scid].Header.Height-1, hs[dcid].Header.Height-1)
	if err != nil {
		return nil, err
	}
	logChannelStates(c, dst, chans)

	switch {
	// Closing handshake has not started, relay `updateClient` and `chanCloseInit` to src or dst according
	// to the channel state
	case chans[scid].Channel.State != chantypes.CLOSED && chans[dcid].Channel.State != chantypes.CLOSED:
		if chans[scid].Channel.State != chantypes.UNINITIALIZED {
			if c.debug {
				logChannelStates(c, dst, chans)
			}
			out.Src = append(out.Src,
				c.PathEnd.UpdateClient(dstUpdateHeader, c.MustGetAddress()),
				c.PathEnd.ChanCloseInit(c.MustGetAddress()),
			)
		} else if chans[dcid].Channel.State != chantypes.UNINITIALIZED {
			if dst.debug {
				logChannelStates(dst, c, chans)
			}
			out.Dst = append(out.Dst,
				dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
				dst.PathEnd.ChanCloseInit(dst.MustGetAddress()),
			)
		}

	// Closing handshake has started on src, relay `updateClient` and `chanCloseConfirm` to dst
	case chans[scid].Channel.State == chantypes.CLOSED && chans[dcid].Channel.State != chantypes.CLOSED:
		if chans[dcid].Channel.State != chantypes.UNINITIALIZED {
			if dst.debug {
				logChannelStates(dst, c, chans)
			}
			out.Dst = append(out.Dst,
				dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
				dst.PathEnd.ChanCloseConfirm(chans[scid], dst.MustGetAddress()),
			)
			out.last = true
		}

	// Closing handshake has started on dst, relay `updateClient` and `chanCloseConfirm` to src
	case chans[dcid].Channel.State == chantypes.CLOSED && chans[scid].Channel.State != chantypes.CLOSED:
		if chans[scid].Channel.State != chantypes.UNINITIALIZED {
			if c.debug {
				logChannelStates(c, dst, chans)
			}
			out.Src = append(out.Src,
				c.PathEnd.UpdateClient(dstUpdateHeader, c.MustGetAddress()),
				c.PathEnd.ChanCloseConfirm(chans[dcid], c.MustGetAddress()),
			)
			out.last = true
		}
	}
	return out, nil
}
