package relayer

import (
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	chantypes "github.com/cosmos/cosmos-sdk/x/ibc/core/04-channel/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
	"golang.org/x/sync/errgroup"
)

// CreateChannel runs the channel creation messages on timeout until they pass
func (c *Chain) CreateOpenChannels(dst *Chain, maxRetries uint64, to time.Duration) (modified bool, err error) {
	// client and connection identifiers must be filled in
	if err := ValidateConnectionPaths(c, dst); err != nil {
		return modified, err
	}
	// ports must be valid and channel ORDER must be the same
	if err := ValidateChannelParams(c, dst); err != nil {
		return modified, err
	}

	ticker := time.NewTicker(to)
	failures := uint64(0)
	for ; true; <-ticker.C {
		success, lastStep, recentlyModified, err := ExecuteChannelStep(c, dst)
		if err != nil {
			c.Log(err.Error())
		}
		if recentlyModified {
			modified = true
		}

		switch {
		// In the case of success and this being the last transaction
		// debug logging, log created channel and break
		case success && lastStep:

			if c.debug {
				srch, dsth, err := GetLatestLightHeights(c, dst)
				if err != nil {
					return modified, err
				}
				srcChan, dstChan, err := QueryChannelPair(c, dst, srch, dsth)
				if err != nil {
					return modified, err
				}
				logChannelStates(c, dst, srcChan, dstChan)
			}

			c.Log(fmt.Sprintf("★ Channel created: [%s]chan{%s}port{%s} -> [%s]chan{%s}port{%s}",
				c.ChainID, c.PathEnd.ChannelID, c.PathEnd.PortID,
				dst.ChainID, dst.PathEnd.ChannelID, dst.PathEnd.PortID))
			return modified, nil

		// In the case of success, reset the failures counter
		case success:
			failures = 0
			continue

		// In the case of failure, increment the failures counter and exit if this is the 3rd failure
		case !success:
			failures++
			c.Log(fmt.Sprintf("retrying transaction..."))
			time.Sleep(5 * time.Second)

			if failures > maxRetries {
				return modified, fmt.Errorf("! Channel failed: [%s]chan{%s}port{%s} -> [%s]chan{%s}port{%s}",
					c.ChainID, c.PathEnd.ChannelID, c.PathEnd.PortID,
					dst.ChainID, dst.PathEnd.ChannelID, dst.PathEnd.PortID)
			}
		}
	}

	return modified, nil // lgtm [go/unreachable-statement]
}

// ExecuteChannelStep executes the next channel step based on the
// states of two channel ends specified by the relayer configuration
// file. The booleans return indicate if the message was successfully
// executed and if this was the last handshake step.
func ExecuteChannelStep(src, dst *Chain) (success, last, modified bool, err error) {
	// update the off chain light clients to the latest header and return the header
	sh, err := NewSyncHeaders(src, dst)
	if err != nil {
		return false, false, false, err
	}

	// variables needed to determine the current handshake step
	var (
		srcUpdateHeader, dstUpdateHeader *tmclient.Header
		srcChan, dstChan                 *chantypes.QueryChannelResponse
		msgs                             []sdk.Msg
	)

	// get headers to update light clients on chain
	srcUpdateHeader, dstUpdateHeader, err = sh.GetTrustedHeaders(src, dst)
	if err != nil {
		return false, false, false, err
	}

	// if either identifier is missing, an existing channel that matches the required fields
	// is chosen or a new channel is created.
	if src.PathEnd.ChannelID == "" || dst.PathEnd.ChannelID == "" {
		// TODO: Query for existing identifier and fill config, if possible
		success, modified, err := InitializeChannel(src, dst, srcUpdateHeader, dstUpdateHeader, sh)
		if err != nil {
			return false, false, false, err
		}

		return success, false, modified, nil
	}

	// Query Channel data from src and dst
	srcChan, dstChan, err = QueryChannelPair(src, dst, int64(sh.GetHeight(src.ChainID))-1, int64(sh.GetHeight(dst.ChainID))-1)
	if err != nil {
		return false, false, false, err
	}

	switch {

	// OpenTry on source in case of crossing hellos (both channels are on INIT)
	// obtain proof of counterparty in TRYOPEN state and submit to source chain to update state
	// from INIT to TRYOPEN.
	case srcChan.Channel.State == chantypes.INIT && dstChan.Channel.State == chantypes.INIT:
		if src.debug {
			logChannelStates(src, dst, srcChan, dstChan)
		}

		openTry, err := src.PathEnd.ChanTry(dst, dstUpdateHeader.GetHeight().GetRevisionHeight()-1, src.MustGetAddress())
		if err != nil {
			return false, false, false, err
		}

		msgs = []sdk.Msg{
			src.PathEnd.UpdateClient(dstUpdateHeader, src.MustGetAddress()),
			openTry,
		}

		_, success, err = src.SendMsgs(msgs)
		if !success {
			return false, false, false, err
		}

	// OpenAck on source if dst is at TRYOPEN and src is at INIT or TRYOPEN (crossing hellos)
	// obtain proof of counterparty in TRYOPEN state and submit to source chain to update state
	// from INIT/TRYOPEN to OPEN.
	case (srcChan.Channel.State == chantypes.INIT || srcChan.Channel.State == chantypes.TRYOPEN) && dstChan.Channel.State == chantypes.TRYOPEN:
		if src.debug {
			logChannelStates(src, dst, srcChan, dstChan)
		}

		openAck, err := src.PathEnd.ChanAck(dst, dstUpdateHeader.GetHeight().GetRevisionHeight()-1, src.MustGetAddress())
		if err != nil {
			return false, false, false, err
		}

		msgs = []sdk.Msg{
			src.PathEnd.UpdateClient(dstUpdateHeader, src.MustGetAddress()),
			openAck,
		}

		_, success, err = src.SendMsgs(msgs)
		if !success {
			return false, false, false, err
		}

	// OpenAck on counterparty
	// obtain proof of source in TRYOPEN state and submit to counterparty chain to update state
	// from INIT to OPEN.
	case srcChan.Channel.State == chantypes.TRYOPEN && dstChan.Channel.State == chantypes.INIT:
		if dst.debug {
			logChannelStates(dst, src, dstChan, srcChan)
		}

		openAck, err := dst.PathEnd.ChanAck(src, srcUpdateHeader.GetHeight().GetRevisionHeight()-1, src.MustGetAddress())
		if err != nil {
			return false, false, false, err
		}

		msgs = []sdk.Msg{
			dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
			openAck,
		}

		_, success, err = dst.SendMsgs(msgs)
		if !success {
			return false, false, false, err
		}

	// OpenConfirm on source
	case srcChan.Channel.State == chantypes.TRYOPEN && dstChan.Channel.State == chantypes.OPEN:
		if src.debug {
			logChannelStates(src, dst, srcChan, dstChan)
		}
		msgs = []sdk.Msg{
			src.PathEnd.UpdateClient(dstUpdateHeader, src.MustGetAddress()),
			src.PathEnd.ChanConfirm(dstChan, src.MustGetAddress()),
		}
		last = true

		_, success, err = src.SendMsgs(msgs)
		if !success {
			return false, false, false, err
		}

	// OpenConfrim on counterparty
	case srcChan.Channel.State == chantypes.OPEN && dstChan.Channel.State == chantypes.TRYOPEN:
		if dst.debug {
			logChannelStates(dst, src, dstChan, srcChan)
		}
		msgs = []sdk.Msg{
			dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
			dst.PathEnd.ChanConfirm(srcChan, dst.MustGetAddress()),
		}
		last = true

		_, success, err = dst.SendMsgs(msgs)
		if !success {
			return false, false, false, err
		}

	}

	return true, last, false, nil
}

// InitializeChannel creates a new channel on either the source or destination chain .
// The identifiers set in the PathEnd's are used to determine which channel ends need to be
// initialized. The PathEnds are updated upon a successful transaction.
// NOTE: This function may need to be called twice if neither channel exists.
func InitializeChannel(src, dst *Chain, srcUpdateHeader, dstUpdateHeader *tmclient.Header, sh *SyncHeaders) (success, modified bool, err error) {
	switch {

	// OpenInit on source
	// Neither channel has been initialized
	case src.PathEnd.ChannelID == "" && dst.PathEnd.ChannelID == "":
		if src.debug {
			// TODO: log that we are attempting to create new channel ends
		}

		// cosntruct OpenInit message to be submitted on source chain
		msgs := []sdk.Msg{
			src.PathEnd.UpdateClient(dstUpdateHeader, src.MustGetAddress()),
			src.PathEnd.ChanInit(dst.PathEnd, src.MustGetAddress()),
		}

		res, success, err := src.SendMsgs(msgs)
		if !success {
			return false, false, err
		}

		// update channel identifier in PathEnd
		// use index 1, channel open init is the second message in the transaction
		channelID, err := ParseChannelIDFromEvents(res.Logs[1].Events)
		if err != nil {
			return false, false, err
		}
		src.PathEnd.ChannelID = channelID

		return true, true, nil

	// OpenTry on source
	// source channel does not exist, but counterparty channel exists
	case src.PathEnd.ChannelID == "" && dst.PathEnd.ChannelID != "":
		if src.debug {
			// TODO: update logging
		}

		// open try on source chain
		openTry, err := src.PathEnd.ChanTry(dst, dstUpdateHeader.GetHeight().GetRevisionHeight()-1, src.MustGetAddress())
		if err != nil {
			return false, false, err
		}

		msgs := []sdk.Msg{
			src.PathEnd.UpdateClient(dstUpdateHeader, src.MustGetAddress()),
			openTry,
		}
		res, success, err := src.SendMsgs(msgs)
		if !success {
			return false, false, err
		}

		// update channel identifier in PathEnd
		// use index 1, channel open try is the second message in the transaction
		channelID, err := ParseChannelIDFromEvents(res.Logs[1].Events)
		if err != nil {
			return false, false, err
		}
		src.PathEnd.ChannelID = channelID

		return true, true, nil

	// OpenTry on counterparty
	// source channel exists, but counterparty channel does not exist
	case src.PathEnd.ChannelID != "" && dst.PathEnd.ChannelID == "":
		if dst.debug {
			// TODO: update logging
		}

		// open try on destination chain
		openTry, err := dst.PathEnd.ChanTry(src, srcUpdateHeader.GetHeight().GetRevisionHeight()-1, dst.MustGetAddress())
		if err != nil {
			return false, false, err
		}

		msgs := []sdk.Msg{
			dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
			openTry,
		}
		res, success, err := dst.SendMsgs(msgs)
		if !success {
			return false, false, err
		}

		// update channel identifier in PathEnd
		// use index 1, channel open try is the second message in the transaction
		channelID, err := ParseChannelIDFromEvents(res.Logs[1].Events)
		if err != nil {
			return false, false, err
		}
		dst.PathEnd.ChannelID = channelID

		return true, true, nil

	default:
		return false, false, fmt.Errorf("channel ends already created")
	}
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

		if closeSteps.Send(c, dst); closeSteps.Success() && closeSteps.Last {
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
	if err := sh.Updates(c, dst); err != nil {
		return nil, err
	}

	// Query a number of things all at once
	var (
		eg                               = new(errgroup.Group)
		srcUpdateHeader, dstUpdateHeader *tmclient.Header
		srcChan, dstChan                 *chantypes.QueryChannelResponse
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
	case srcChan.Channel.State != chantypes.CLOSED && dstChan.Channel.State != chantypes.CLOSED:
		if srcChan.Channel.State != chantypes.UNINITIALIZED {
			if c.debug {
				logChannelStates(c, dst, srcChan, dstChan)
			}
			out.Src = append(out.Src,
				c.PathEnd.UpdateClient(dstUpdateHeader, c.MustGetAddress()),
				c.PathEnd.ChanCloseInit(c.MustGetAddress()),
			)
		} else if dstChan.Channel.State != chantypes.UNINITIALIZED {
			if dst.debug {
				logChannelStates(dst, c, dstChan, srcChan)
			}
			out.Dst = append(out.Dst,
				dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
				dst.PathEnd.ChanCloseInit(dst.MustGetAddress()),
			)
		}

	// Closing handshake has started on src, relay `updateClient` and `chanCloseConfirm` to dst
	case srcChan.Channel.State == chantypes.CLOSED && dstChan.Channel.State != chantypes.CLOSED:
		if dstChan.Channel.State != chantypes.UNINITIALIZED {
			if dst.debug {
				logChannelStates(dst, c, dstChan, srcChan)
			}
			out.Dst = append(out.Dst,
				dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
				dst.PathEnd.ChanCloseConfirm(srcChan, dst.MustGetAddress()),
			)
			out.Last = true
		}

	// Closing handshake has started on dst, relay `updateClient` and `chanCloseConfirm` to src
	case dstChan.Channel.State == chantypes.CLOSED && srcChan.Channel.State != chantypes.CLOSED:
		if srcChan.Channel.State != chantypes.UNINITIALIZED {
			if c.debug {
				logChannelStates(c, dst, srcChan, dstChan)
			}
			out.Src = append(out.Src,
				c.PathEnd.UpdateClient(dstUpdateHeader, c.MustGetAddress()),
				c.PathEnd.ChanCloseConfirm(dstChan, c.MustGetAddress()),
			)
			out.Last = true
		}
	}
	return out, nil
}
