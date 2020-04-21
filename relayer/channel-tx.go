package relayer

import (
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	chanState "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/exported"
)

// CreateChannel runs the channel creation messages on timeout until they pass
// TODO: add max retries or something to this function
func (src *Chain) CreateChannel(dst *Chain, ordered bool, to time.Duration) error {
	var order chanState.Order
	if ordered {
		order = chanState.ORDERED
	} else {
		order = chanState.UNORDERED
	}

	ticker := time.NewTicker(to)
	failures := 0
	for ; true; <-ticker.C {
		chanSteps, err := src.CreateChannelStep(dst, order)
		if err != nil {
			return err
		}

		if !chanSteps.Ready() {
			break
		}

		chanSteps.Send(src, dst)

		switch {
		// In the case of success and this being the last transaction
		// debug logging, log created connection and break
		case chanSteps.success && chanSteps.last:
			chans, err := QueryChannelPair(src, dst, 0, 0)
			if err != nil {
				return err
			}
			if src.debug {
				logChannelStates(src, dst, chans)
			}
			src.Log(fmt.Sprintf("★ Channel created: [%s]chan{%s}port{%s} -> [%s]chan{%s}port{%s}",
				src.ChainID, src.PathEnd.ChannelID, src.PathEnd.PortID,
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
					src.ChainID, src.PathEnd.ChannelID, src.PathEnd.PortID,
					dst.ChainID, dst.PathEnd.ChannelID, dst.PathEnd.PortID)
			}
		}
	}

	return nil
}

// CreateChannelStep returns the next set of messages for creating a channel with given
// identifiers between chains src and dst. If the handshake hasn't started, then CreateChannelStep
// will begin the handshake on the src chain
func (src *Chain) CreateChannelStep(dst *Chain, ordering chanState.Order) (*RelayMsgs, error) {
	var (
		out        = &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}, last: false}
		scid, dcid = src.ChainID, dst.ChainID
	)

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

	chans, err := QueryChannelPair(src, dst, hs[scid].Height-1, hs[dcid].Height-1)
	if err != nil {
		return nil, err
	}

	switch {
	// Handshake hasn't been started on src or dst, relay `chanOpenInit` to src
	case chans[scid].Channel.Channel.State == chanState.UNINITIALIZED && chans[dcid].Channel.Channel.State == chanState.UNINITIALIZED:
		if src.debug {
			logChannelStates(src, dst, chans)
		}
		out.Src = append(out.Src,
			src.PathEnd.ChanInit(dst.PathEnd, src.MustGetAddress()),
		)

	// Handshake has started on dst (1 step done), relay `chanOpenTry` and `updateClient` to src
	case chans[scid].Channel.Channel.State == chanState.UNINITIALIZED && chans[dcid].Channel.Channel.State == chanState.INIT:
		if src.debug {
			logChannelStates(src, dst, chans)
		}
		out.Src = append(out.Src,
			src.PathEnd.UpdateClient(hs[dcid], src.MustGetAddress()),
			src.PathEnd.ChanTry(dst.PathEnd, chans[dcid], src.MustGetAddress()),
		)

	// Handshake has started on src (1 step done), relay `chanOpenTry` and `updateClient` to dst
	case chans[scid].Channel.Channel.State == chanState.INIT && chans[dcid].Channel.Channel.State == chanState.UNINITIALIZED:
		if dst.debug {
			logChannelStates(dst, src, chans)
		}
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(hs[scid], dst.MustGetAddress()),
			dst.PathEnd.ChanTry(src.PathEnd, chans[scid], dst.MustGetAddress()),
		)

	// Handshake has started on src (2 steps done), relay `chanOpenAck` and `updateClient` to dst
	case chans[scid].Channel.Channel.State == chanState.TRYOPEN && chans[dcid].Channel.Channel.State == chanState.INIT:
		if dst.debug {
			logChannelStates(dst, src, chans)
		}
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(hs[scid], dst.MustGetAddress()),
			dst.PathEnd.ChanAck(chans[scid], dst.MustGetAddress()),
		)

	// Handshake has started on dst (2 steps done), relay `chanOpenAck` and `updateClient` to src
	case chans[scid].Channel.Channel.State == chanState.INIT && chans[dcid].Channel.Channel.State == chanState.TRYOPEN:
		if src.debug {
			logChannelStates(src, dst, chans)
		}
		out.Src = append(out.Src,
			src.PathEnd.UpdateClient(hs[dcid], src.MustGetAddress()),
			src.PathEnd.ChanAck(chans[dcid], src.MustGetAddress()),
		)

	// Handshake has confirmed on dst (3 steps done), relay `chanOpenConfirm` and `updateClient` to src
	case chans[scid].Channel.Channel.State == chanState.TRYOPEN && chans[dcid].Channel.Channel.State == chanState.OPEN:
		if src.debug {
			logChannelStates(src, dst, chans)
		}
		out.Src = append(out.Src,
			src.PathEnd.UpdateClient(hs[dcid], src.MustGetAddress()),
			src.PathEnd.ChanConfirm(chans[dcid], src.MustGetAddress()),
		)
		out.last = true

	// Handshake has confirmed on src (3 steps done), relay `chanOpenConfirm` and `updateClient` to dst
	case chans[scid].Channel.Channel.State == chanState.OPEN && chans[dcid].Channel.Channel.State == chanState.TRYOPEN:
		if dst.debug {
			logChannelStates(dst, src, chans)
		}
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(hs[scid], dst.MustGetAddress()),
			dst.PathEnd.ChanConfirm(chans[scid], dst.MustGetAddress()),
		)
		out.last = true
	}

	return out, nil
}

// CloseChannel runs the channel closing messages on timeout until they pass
// TODO: add max retries or something to this function
func (src *Chain) CloseChannel(dst *Chain, to time.Duration) error {

	ticker := time.NewTicker(to)
	for ; true; <-ticker.C {
		closeSteps, err := src.CloseChannelStep(dst)
		if err != nil {
			return err
		}

		if !closeSteps.Ready() {
			break
		}

		if closeSteps.Send(src, dst); closeSteps.success && closeSteps.last {
			chans, err := QueryChannelPair(src, dst, 0, 0)
			if err != nil {
				return err
			}
			if src.debug {
				logChannelStates(src, dst, chans)
			}
			src.Log(fmt.Sprintf("★ Closed channel between [%s]chan{%s}port{%s} -> [%s]chan{%s}port{%s}",
				src.ChainID, src.PathEnd.ChannelID, src.PathEnd.PortID,
				dst.ChainID, dst.PathEnd.ChannelID, dst.PathEnd.PortID))
			break
		}
	}
	return nil
}

// CloseChannelStep returns the next set of messages for closing a channel with given
// identifiers between chains src and dst. If the closing handshake hasn't started, then CloseChannelStep
// will begin the handshake on the src chain
func (src *Chain) CloseChannelStep(dst *Chain) (*RelayMsgs, error) {
	var (
		out        = &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}, last: false}
		scid, dcid = src.ChainID, dst.ChainID
	)

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

	chans, err := QueryChannelPair(src, dst, hs[scid].Height-1, hs[dcid].Height-1)
	if err != nil {
		return nil, err
	}
	logChannelStates(src, dst, chans)

	switch {
	// Closing handshake has not started, relay `updateClient` and `chanCloseInit` to src or dst according
	// to the channel state
	case chans[scid].Channel.Channel.State != chanState.CLOSED && chans[dcid].Channel.Channel.State != chanState.CLOSED:
		if chans[scid].Channel.Channel.State != chanState.UNINITIALIZED {
			if src.debug {
				logChannelStates(src, dst, chans)
			}
			out.Src = append(out.Src,
				src.PathEnd.UpdateClient(hs[dcid], src.MustGetAddress()),
				src.PathEnd.ChanCloseInit(src.MustGetAddress()),
			)
		} else if chans[dcid].Channel.Channel.State != chanState.UNINITIALIZED {
			if dst.debug {
				logChannelStates(dst, src, chans)
			}
			out.Dst = append(out.Dst,
				dst.PathEnd.UpdateClient(hs[scid], dst.MustGetAddress()),
				dst.PathEnd.ChanCloseInit(dst.MustGetAddress()),
			)
		}

	// Closing handshake has started on src, relay `updateClient` and `chanCloseConfirm` to dst
	case chans[scid].Channel.Channel.State == chanState.CLOSED && chans[dcid].Channel.Channel.State != chanState.CLOSED:
		if chans[dcid].Channel.Channel.State != chanState.UNINITIALIZED {
			if dst.debug {
				logChannelStates(dst, src, chans)
			}
			out.Dst = append(out.Dst,
				dst.PathEnd.UpdateClient(hs[scid], dst.MustGetAddress()),
				dst.PathEnd.ChanCloseConfirm(chans[scid], dst.MustGetAddress()),
			)
			out.last = true
		}

	// Closing handshake has started on dst, relay `updateClient` and `chanCloseConfirm` to src
	case chans[dcid].Channel.Channel.State == chanState.CLOSED && chans[scid].Channel.Channel.State != chanState.CLOSED:
		if chans[scid].Channel.Channel.State != chanState.UNINITIALIZED {
			if src.debug {
				logChannelStates(src, dst, chans)
			}
			out.Src = append(out.Src,
				src.PathEnd.UpdateClient(hs[dcid], src.MustGetAddress()),
				src.PathEnd.ChanCloseConfirm(chans[dcid], src.MustGetAddress()),
			)
			out.last = true
		}
	}
	return out, nil
}
