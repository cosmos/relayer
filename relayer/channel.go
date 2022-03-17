package relayer

import (
	"fmt"
	"strings"
	"time"

	"github.com/avast/retry-go"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	host "github.com/cosmos/ibc-go/v3/modules/core/24-host"
	"github.com/cosmos/ibc-go/v3/modules/core/exported"
	"github.com/cosmos/relayer/relayer/provider"
)

// CreateOpenChannels runs the channel creation messages on timeout until they pass,
func (c *Chain) CreateOpenChannels(dst *Chain, maxRetries uint64, to time.Duration, srcPortID, dstPortID, order, version string, override bool) (modified bool, err error) {
	// client and connection identifiers must be filled in
	if err := ValidateConnectionPaths(c, dst); err != nil {
		return modified, err
	}

	// port identifiers and channel ORDER must be valid
	if err := ValidateChannelParams(srcPortID, dstPortID, order); err != nil {
		return modified, err
	}

	var (
		srcChannelID, dstChannelID          string
		success, lastStep, recentlyModified bool
	)

	ticker := time.NewTicker(to)
	defer ticker.Stop()

	failures := uint64(0)
	for ; true; <-ticker.C {
		var err error
		srcChannelID, dstChannelID, success, lastStep, recentlyModified, err = ExecuteChannelStep(c, dst, srcChannelID,
			dstChannelID, srcPortID, dstPortID, order, version, override)

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
				srch, dsth, err := QueryLatestHeights(c, dst)
				if err != nil {
					return modified, err
				}
				srcChan, dstChan, err := QueryChannelPair(c, dst, srch, dsth, srcChannelID, dstChannelID, srcPortID, dstPortID)
				if err != nil {
					return modified, err
				}
				logChannelStates(c, dst, srcChan, dstChan)
			}

			c.Log(fmt.Sprintf("★ Channel created: [%s]chan{%s}port{%s} -> [%s]chan{%s}port{%s}",
				c.ChainID(), srcChannelID, srcPortID, dst.ChainID(), dstChannelID, dstPortID))

			return modified, nil

		// In the case of success, reset the failures counter
		case success:
			failures = 0
			continue

		// In the case of failure, increment the failures counter and exit if this is the 3rd failure
		case !success:
			failures++
			c.Log("retrying transaction...")
			time.Sleep(5 * time.Second)

			if failures > maxRetries {
				return modified, fmt.Errorf("! Channel failed: [%s]chan{%s}port{%s} -> [%s]chan{%s}port{%s}",
					c.ChainID(), srcChannelID, srcPortID,
					dst.ChainID(), dstChannelID, dstPortID)
			}
		}
	}

	return modified, nil // lgtm [go/unreachable-statement]
}

// ExecuteChannelStep executes the next channel step based on the
// states of two channel ends specified by the relayer configuration
// file. The booleans return indicate if the message was successfully
// executed and if this was the last handshake step.
func ExecuteChannelStep(src, dst *Chain, srcChanID, dstChanID, srcPortID, dstPortID, order, version string, override bool) (
	newSrcChanID, newDstChanID string, success, last, modified bool, err error) {

	var (
		srch, dsth           int64
		srcHeader, dstHeader exported.Header
		msgs                 []provider.RelayerMessage
		res                  *provider.RelayerTxResponse
	)

	if err = retry.Do(func() error {
		srch, dsth, err = QueryLatestHeights(src, dst)
		if err != nil || srch == 0 || dsth == 0 {
			return fmt.Errorf("failed to query latest heights. Err: %w", err)
		}
		return err
	}, RtyAtt, RtyDel, RtyErr); err != nil {
		return srcChanID, dstChanID, success, last, modified, err
	}

	// if either identifier is missing, an existing channel that matches the required fields is chosen
	// or a new channel is created.
	if srcChanID == "" || dstChanID == "" {
		newSrcChanID, newDstChanID, success, modified, err = InitializeChannel(src, dst, srcChanID, dstChanID, srcPortID,
			dstPortID, order, version, override)

		if err != nil {
			return srcChanID, dstChanID, false, false, false, err
		}

		return newSrcChanID, newDstChanID, success, false, modified, nil
	}

	// Query Channel data from src and dst
	srcChan, dstChan, err := QueryChannelPair(src, dst, srch-1, dsth-1, srcChanID, dstChanID, srcPortID, dstPortID)
	if err != nil {
		return srcChanID, dstChanID, false, false, false, err
	}

	switch {

	// OpenTry on source in case of crossing hellos (both channels are on INIT)
	// obtain proof of counterparty in TRYOPEN state and submit to source chain to update state
	// from INIT to TRYOPEN.
	case srcChan.Channel.State == chantypes.INIT && dstChan.Channel.State == chantypes.INIT:
		if src.debug {
			logChannelStates(src, dst, srcChan, dstChan)
		}

		if err = retry.Do(func() error {
			dsth, err = dst.ChainProvider.QueryLatestHeight()
			if err != nil || dsth == 0 {
				return fmt.Errorf("failed to query latest heights. Err: %w", err)
			}
			return err
		}, RtyAtt, RtyDel, RtyErr); err != nil {
			return srcChanID, dstChanID, success, last, modified, err
		}

		if err = retry.Do(func() error {
			dstHeader, err = dst.ChainProvider.GetIBCUpdateHeader(dsth, src.ChainProvider, src.ClientID())
			if err != nil || dsth == 0 {
				return fmt.Errorf("failed to get IBC update header. Err: %w", err)
			}
			return err
		}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
			dst.LogRetryGetIBCUpdateHeader(n, err)
			dsth, _ = dst.ChainProvider.QueryLatestHeight()
		})); err != nil {
			return srcChanID, dstChanID, success, last, modified, err
		}

		msgs, err = src.ChainProvider.ChannelOpenTry(dst.ChainProvider, dstHeader, srcPortID, dstPortID,
			srcChanID, dstChanID, version, src.ConnectionID(), src.ClientID())

		if err != nil {
			return srcChanID, dstChanID, false, false, false, err
		}

		res, success, err = src.ChainProvider.SendMessages(msgs)
		if err != nil {
			src.LogFailedTx(res, err, msgs)
		}
		if !success {
			return srcChanID, dstChanID, false, false, false, err
		}

	// OpenAck on source if dst is at TRYOPEN and src is at INIT or TRYOPEN (crossing hellos)
	// obtain proof of counterparty in TRYOPEN state and submit to source chain to update state
	// from INIT/TRYOPEN to OPEN.
	case (srcChan.Channel.State == chantypes.INIT || srcChan.Channel.State == chantypes.TRYOPEN) &&
		dstChan.Channel.State == chantypes.TRYOPEN:

		if src.debug {
			logChannelStates(src, dst, srcChan, dstChan)
		}

		if err = retry.Do(func() error {
			dsth, err = dst.ChainProvider.QueryLatestHeight()
			if err != nil || dsth == 0 {
				return fmt.Errorf("failed to query latest heights. Err: %w", err)
			}
			return err
		}, RtyAtt, RtyDel, RtyErr); err != nil {
			return srcChanID, dstChanID, success, last, modified, err
		}

		if err = retry.Do(func() error {
			dstHeader, err = dst.ChainProvider.GetIBCUpdateHeader(dsth, src.ChainProvider, src.ClientID())
			if err != nil || dsth == 0 {
				return fmt.Errorf("failed to get IBC update header. Err: %w", err)
			}
			return err
		}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
			dst.LogRetryGetIBCUpdateHeader(n, err)
			dsth, _ = dst.ChainProvider.QueryLatestHeight()
		})); err != nil {
			return srcChanID, dstChanID, success, last, modified, err
		}

		msgs, err = src.ChainProvider.ChannelOpenAck(dst.ChainProvider, dstHeader, src.ClientID(), srcPortID, srcChanID, dstChanID, dstPortID)
		if err != nil {
			return srcChanID, dstChanID, false, false, false, err
		}

		res, success, err = src.ChainProvider.SendMessages(msgs)
		if err != nil {
			src.LogFailedTx(res, err, msgs)
		}
		if !success {
			return srcChanID, dstChanID, false, false, false, err
		}

	// OpenAck on counterparty
	// obtain proof of source in TRYOPEN state and submit to counterparty chain to update state
	// from INIT to OPEN.
	case srcChan.Channel.State == chantypes.TRYOPEN && dstChan.Channel.State == chantypes.INIT:
		if dst.debug {
			logChannelStates(dst, src, dstChan, srcChan)
		}

		if err = retry.Do(func() error {
			srch, err = src.ChainProvider.QueryLatestHeight()
			if err != nil || srch == 0 {
				return fmt.Errorf("failed to query latest heights. Err: %w", err)
			}
			return err
		}, RtyAtt, RtyDel, RtyErr); err != nil {
			return srcChanID, dstChanID, success, last, modified, err
		}

		if err = retry.Do(func() error {
			srcHeader, err = src.ChainProvider.GetIBCUpdateHeader(srch, dst.ChainProvider, dst.ClientID())
			if err != nil || srch == 0 {
				return fmt.Errorf("failed to get IBC update header. Err: %w", err)
			}
			return err
		}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
			src.LogRetryGetIBCUpdateHeader(n, err)
			srch, _ = src.ChainProvider.QueryLatestHeight()
		})); err != nil {
			return srcChanID, dstChanID, success, last, modified, err
		}

		msgs, err = dst.ChainProvider.ChannelOpenAck(src.ChainProvider, srcHeader, dst.ClientID(), dstPortID, dstChanID, srcChanID, srcPortID)
		if err != nil {
			return srcChanID, dstChanID, false, false, false, err
		}

		res, success, err = dst.ChainProvider.SendMessages(msgs)
		if err != nil {
			dst.LogFailedTx(res, err, msgs)
		}
		if !success {
			return srcChanID, dstChanID, false, false, false, err
		}

	// OpenConfirm on source
	case srcChan.Channel.State == chantypes.TRYOPEN && dstChan.Channel.State == chantypes.OPEN:
		if src.debug {
			logChannelStates(src, dst, srcChan, dstChan)
		}

		if err = retry.Do(func() error {
			dsth, err = dst.ChainProvider.QueryLatestHeight()
			if err != nil || dsth == 0 {
				return fmt.Errorf("failed to query latest heights. Err: %w", err)
			}
			return err
		}, RtyAtt, RtyDel, RtyErr); err != nil {
			return srcChanID, dstChanID, success, last, modified, err
		}

		if err = retry.Do(func() error {
			dstHeader, err = dst.ChainProvider.GetIBCUpdateHeader(dsth, src.ChainProvider, src.ClientID())
			if err != nil || dsth == 0 {
				return fmt.Errorf("failed to get IBC update header. Err: %w", err)
			}
			return err
		}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
			dst.LogRetryGetIBCUpdateHeader(n, err)
			dsth, _ = dst.ChainProvider.QueryLatestHeight()
		})); err != nil {
			return srcChanID, dstChanID, success, last, modified, err
		}

		msgs, err = src.ChainProvider.ChannelOpenConfirm(dst.ChainProvider, dstHeader, src.ClientID(), srcPortID, srcChanID, dstPortID, dstChanID)
		if err != nil {
			return srcChanID, dstChanID, false, false, false, err
		}

		last = true

		res, success, err = src.ChainProvider.SendMessages(msgs)
		if err != nil {
			src.LogFailedTx(res, err, msgs)
		}
		if !success {
			return srcChanID, dstChanID, false, false, false, err
		}

	// OpenConfirm on counterparty
	case srcChan.Channel.State == chantypes.OPEN && dstChan.Channel.State == chantypes.TRYOPEN:
		if dst.debug {
			logChannelStates(dst, src, dstChan, srcChan)
		}

		if err = retry.Do(func() error {
			srch, err = src.ChainProvider.QueryLatestHeight()
			if err != nil || srch == 0 {
				return fmt.Errorf("failed to query latest heights. Err: %w", err)
			}
			return err
		}, RtyAtt, RtyDel, RtyErr); err != nil {
			return srcChanID, dstChanID, success, last, modified, err
		}

		if err = retry.Do(func() error {
			srcHeader, err = src.ChainProvider.GetIBCUpdateHeader(srch, dst.ChainProvider, dst.ClientID())
			if err != nil || srch == 0 {
				return fmt.Errorf("failed to get IBC update header. Err: %w", err)
			}
			return err
		}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
			src.LogRetryGetIBCUpdateHeader(n, err)
			srch, _ = src.ChainProvider.QueryLatestHeight()
		})); err != nil {
			return srcChanID, dstChanID, success, last, modified, err
		}

		msgs, err = dst.ChainProvider.ChannelOpenConfirm(src.ChainProvider, srcHeader, dst.ClientID(), dstPortID, dstChanID, srcPortID, srcChanID)
		if err != nil {
			return srcChanID, dstChanID, false, false, false, err
		}

		res, success, err = dst.ChainProvider.SendMessages(msgs)
		if err != nil {
			dst.LogFailedTx(res, err, msgs)
		}
		if !success {
			return srcChanID, dstChanID, false, false, false, err
		}

		last = true

	case srcChan.Channel.State == chantypes.OPEN && dstChan.Channel.State == chantypes.OPEN:
		last = true

	}

	return srcChanID, dstChanID, true, last, false, nil
}

// InitializeChannel creates a new channel on either the source or destination chain.
// NOTE: This function may need to be called twice if neither channel exists.
func InitializeChannel(src, dst *Chain, srcChanID, dstChanID, srcPortID, dstPortID, order, version string, override bool) (
	newSrcChanID, newDstChanID string, success, modified bool, err error) {

	var (
		srch, dsth           int64
		srcHeader, dstHeader exported.Header
		msgs                 []provider.RelayerMessage
		res                  *provider.RelayerTxResponse
		existingChanID       string
		found                bool
	)

	switch {

	// OpenInit on source
	// Neither channel has been initialized
	case srcChanID == "" && dstChanID == "":
		if src.debug {
			src.logOpenInit(dst, "channel")
		}

		if !override {
			existingChanID, found = FindMatchingChannel(src, srcPortID, dstPortID, order, version)
		}

		if !found || override {

			if err = retry.Do(func() error {
				dsth, err = dst.ChainProvider.QueryLatestHeight()
				if err != nil || dsth == 0 {
					return fmt.Errorf("failed to query latest heights. Err: %w", err)
				}
				return err
			}, RtyAtt, RtyDel, RtyErr); err != nil {
				return srcChanID, dstChanID, false, false, err
			}

			if err = retry.Do(func() error {
				dstHeader, err = dst.ChainProvider.GetIBCUpdateHeader(dsth, src.ChainProvider, src.ClientID())
				if err != nil || dsth == 0 {
					return fmt.Errorf("failed to get IBC update header. Err: %w", err)
				}
				return err
			}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
				dst.LogRetryGetIBCUpdateHeader(n, err)
				dsth, _ = dst.ChainProvider.QueryLatestHeight()
			})); err != nil {
				return srcChanID, dstChanID, false, false, err
			}

			msgs, err = src.ChainProvider.ChannelOpenInit(src.ClientID(), src.ConnectionID(), srcPortID, version, dstPortID, OrderFromString(strings.ToUpper(order)), dstHeader)
			if err != nil {
				return srcChanID, dstChanID, false, false, err
			}

			res, success, err = src.ChainProvider.SendMessages(msgs)
			if err != nil {
				src.LogFailedTx(res, err, msgs)
			}
			if !success {
				return srcChanID, dstChanID, false, false, err
			}

			// use index 1, channel open init is the second message in the transaction
			existingChanID, err = ParseChannelIDFromEvents(res.Events)
			if err != nil {
				return srcChanID, dstChanID, false, false, err
			}
		} else if src.debug {
			src.logIdentifierExists(dst, "channel end", existingChanID)
		}

		return existingChanID, dstChanID, true, true, nil

	// OpenTry on source
	// source channel does not exist, but counterparty channel exists
	case srcChanID == "" && dstChanID != "":
		if src.debug {
			src.logOpenTry(dst, "channel")
		}

		if !override {
			existingChanID, found = FindMatchingChannel(src, srcPortID, dstPortID, order, version)
		}

		if !found || override {

			if err = retry.Do(func() error {
				dsth, err = dst.ChainProvider.QueryLatestHeight()
				if err != nil || dsth == 0 {
					return fmt.Errorf("failed to query latest heights. Err: %w", err)
				}
				return err
			}, RtyAtt, RtyDel, RtyErr); err != nil {
				return srcChanID, dstChanID, false, false, err
			}

			if err = retry.Do(func() error {
				dstHeader, err = dst.ChainProvider.GetIBCUpdateHeader(dsth, src.ChainProvider, src.ClientID())
				if err != nil || dsth == 0 {
					return fmt.Errorf("failed to get IBC update header. Err: %w", err)
				}
				return err
			}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
				dst.LogRetryGetIBCUpdateHeader(n, err)
				dsth, _ = dst.ChainProvider.QueryLatestHeight()
			})); err != nil {
				return srcChanID, dstChanID, false, false, err
			}

			// open try on source chain
			msgs, err = src.ChainProvider.ChannelOpenTry(dst.ChainProvider, dstHeader, srcPortID, dstPortID, srcChanID, dstChanID, version, src.ConnectionID(), src.ClientID())
			if err != nil {
				return srcChanID, dstChanID, false, false, err
			}

			res, success, err = src.ChainProvider.SendMessages(msgs)
			if err != nil {
				src.LogFailedTx(res, err, msgs)
			}
			if !success {
				return srcChanID, dstChanID, false, false, err
			}

			// use index 1, channel open try is the second message in the transaction
			existingChanID, err = ParseChannelIDFromEvents(res.Events)
			if err != nil {
				return srcChanID, dstChanID, false, false, err
			}
		} else if src.debug {
			src.logIdentifierExists(dst, "channel end", existingChanID)
		}

		return existingChanID, dstChanID, true, true, nil

	// OpenTry on counterparty
	// source channel exists, but counterparty channel does not exist
	case srcChanID != "" && dstChanID == "":
		if dst.debug {
			dst.logOpenTry(src, "channel")
		}

		if !override {
			existingChanID, found = FindMatchingChannel(dst, dstPortID, srcPortID, order, version)
		}

		if !found || override {

			if err = retry.Do(func() error {
				srch, err = src.ChainProvider.QueryLatestHeight()
				if err != nil || srch == 0 {
					return fmt.Errorf("failed to query latest heights. Err: %w", err)
				}
				return err
			}, RtyAtt, RtyDel, RtyErr); err != nil {
				return srcChanID, dstChanID, false, false, err
			}

			if err = retry.Do(func() error {
				srcHeader, err = src.ChainProvider.GetIBCUpdateHeader(srch, dst.ChainProvider, dst.ClientID())
				if err != nil || srch == 0 {
					return fmt.Errorf("failed to get IBC update header. Err: %w", err)
				}
				return err
			}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
				src.LogRetryGetIBCUpdateHeader(n, err)
				srch, _ = src.ChainProvider.QueryLatestHeight()
			})); err != nil {
				return srcChanID, dstChanID, false, false, err
			}

			// open try on destination chain
			msgs, err = dst.ChainProvider.ChannelOpenTry(src.ChainProvider, srcHeader, dstPortID, srcPortID, dstChanID, srcChanID, version, dst.ConnectionID(), dst.ClientID())
			if err != nil {
				return srcChanID, dstChanID, false, false, err
			}

			res, success, err = dst.ChainProvider.SendMessages(msgs)
			if err != nil {
				dst.LogFailedTx(res, err, msgs)
			}
			if !success {
				return srcChanID, dstChanID, false, false, err
			}

			// use index 1, channel open try is the second message in the transaction
			existingChanID, err = ParseChannelIDFromEvents(res.Events)
			if err != nil {
				return srcChanID, dstChanID, false, false, err
			}
		} else if dst.debug {
			dst.logIdentifierExists(src, "channel end", existingChanID)
		}

		return srcChanID, existingChanID, true, true, nil

	default:
		return srcChanID, dstChanID, false, false, fmt.Errorf("channel ends already created")
	}
}

// CloseChannel runs the channel closing messages on timeout until they pass
// TODO: add max retries or something to this function
func (c *Chain) CloseChannel(dst *Chain, to time.Duration, srcChanID, srcPortID string, srcChan *chantypes.QueryChannelResponse) error {
	ticker := time.NewTicker(to)
	defer ticker.Stop()

	dstChanID := srcChan.Channel.Counterparty.ChannelId
	dstPortID := srcChan.Channel.Counterparty.PortId

	for ; true; <-ticker.C {
		closeSteps, err := c.CloseChannelStep(dst, srcChanID, srcPortID, srcChan)
		if err != nil {
			return err
		}

		if !closeSteps.Ready() {
			break
		}

		if closeSteps.Send(c, dst); closeSteps.Success() && closeSteps.Last {
			srch, dsth, err := QueryLatestHeights(c, dst)
			if err != nil {
				return err
			}

			srcChan, dstChan, err := QueryChannelPair(c, dst, srch, dsth, srcChanID, dstChanID, srcPortID, dstPortID)
			if err != nil {
				return err
			}

			if c.debug {
				logChannelStates(c, dst, srcChan, dstChan)
			}
			c.Log(fmt.Sprintf("★ Closed srcChan between [%s]chan{%s}port{%s} -> [%s]chan{%s}port{%s}",
				c.ChainID(), srcChanID, srcPortID,
				dst.ChainID(), dstChanID, dstPortID))
			break
		}
	}
	return nil
}

// CloseChannelStep returns the next set of messages for closing a channel with given
// identifiers between chains src and dst. If the closing handshake hasn't started, then CloseChannelStep
// will begin the handshake on the src chain
func (c *Chain) CloseChannelStep(dst *Chain, srcChanID, srcPortID string, srcChan *chantypes.QueryChannelResponse) (*RelayMsgs, error) {
	dsth, err := dst.ChainProvider.QueryLatestHeight()
	if err != nil {
		return nil, err
	}

	out := NewRelayMsgs()
	if err := ValidatePaths(c, dst); err != nil {
		return nil, err
	}

	dstChanID := srcChan.Channel.Counterparty.ChannelId
	dstPortID := srcChan.Channel.Counterparty.PortId

	dstChan, err := dst.ChainProvider.QueryChannel(dsth, dstChanID, dstPortID)
	if err != nil {
		return nil, err
	}

	logChannelStates(c, dst, srcChan, dstChan)

	switch {
	// Closing handshake has not started, relay `updateClient` and `chanCloseInit` to src or dst according
	// to the srcChan state
	case srcChan.Channel.State != chantypes.CLOSED && dstChan.Channel.State != chantypes.CLOSED:
		if srcChan.Channel.State != chantypes.UNINITIALIZED {
			if c.debug {
				logChannelStates(c, dst, srcChan, dstChan)
			}

			dstHeader, err := dst.ChainProvider.GetIBCUpdateHeader(dsth, c.ChainProvider, c.ClientID())
			if err != nil {
				return nil, err
			}

			updateMsg, err := c.ChainProvider.UpdateClient(c.ClientID(), dstHeader)
			if err != nil {
				return nil, err
			}

			msg, err := c.ChainProvider.ChannelCloseInit(srcPortID, srcChanID)
			if err != nil {
				return nil, err
			}
			out.Src = append(out.Src, updateMsg, msg)
		} else if dstChan.Channel.State != chantypes.UNINITIALIZED {
			if dst.debug {
				logChannelStates(dst, c, dstChan, srcChan)
			}

			srch, err := c.ChainProvider.QueryLatestHeight()
			if err != nil {
				return nil, err
			}

			srcHeader, err := c.ChainProvider.GetIBCUpdateHeader(srch, dst.ChainProvider, dst.ClientID())
			if err != nil {
				return nil, err
			}

			updateMsg, err := dst.ChainProvider.UpdateClient(dst.ClientID(), srcHeader)
			if err != nil {
				return nil, err
			}

			msg, err := dst.ChainProvider.ChannelCloseInit(dstPortID, dstChanID)
			if err != nil {
				return nil, err
			}
			out.Dst = append(out.Dst, updateMsg, msg)
		}

	// Closing handshake has started on src, relay `updateClient` and `chanCloseConfirm` to dst
	case srcChan.Channel.State == chantypes.CLOSED && dstChan.Channel.State != chantypes.CLOSED:
		if dstChan.Channel.State != chantypes.UNINITIALIZED {
			if dst.debug {
				logChannelStates(dst, c, dstChan, srcChan)
			}

			srch, err := c.ChainProvider.QueryLatestHeight()
			if err != nil {
				return nil, err
			}

			srcHeader, err := c.ChainProvider.GetIBCUpdateHeader(srch, dst.ChainProvider, dst.ClientID())
			if err != nil {
				return nil, err
			}

			updateMsg, err := dst.ChainProvider.UpdateClient(dst.ClientID(), srcHeader)
			if err != nil {
				return nil, err
			}

			chanCloseConfirm, err := dst.ChainProvider.ChannelCloseConfirm(c.ChainProvider, srch, srcChanID, srcPortID, dstPortID, dstChanID)
			if err != nil {
				return nil, err
			}

			out.Dst = append(out.Dst,
				updateMsg,
				chanCloseConfirm,
			)
			out.Last = true
		}

	// Closing handshake has started on dst, relay `updateClient` and `chanCloseConfirm` to src
	case dstChan.Channel.State == chantypes.CLOSED && srcChan.Channel.State != chantypes.CLOSED:
		if srcChan.Channel.State != chantypes.UNINITIALIZED {
			if c.debug {
				logChannelStates(c, dst, srcChan, dstChan)
			}

			dsth, err := dst.ChainProvider.QueryLatestHeight()
			if err != nil {
				return nil, err
			}

			dstHeader, err := dst.ChainProvider.GetIBCUpdateHeader(dsth, c.ChainProvider, c.ClientID())
			if err != nil {
				return nil, err
			}

			updateMsg, err := c.ChainProvider.UpdateClient(c.ClientID(), dstHeader)
			if err != nil {
				return nil, err
			}

			chanCloseConfirm, err := c.ChainProvider.ChannelCloseConfirm(dst.ChainProvider, dsth, dstChanID, dstPortID, srcPortID, srcChanID)
			if err != nil {
				return nil, err
			}

			out.Src = append(out.Src,
				updateMsg,
				chanCloseConfirm,
			)
			out.Last = true
		}
	}
	return out, nil
}

// FindMatchingChannel will determine if there already exists a channel between source and counterparty
// that matches the parameters set in the relayer config.
func FindMatchingChannel(source *Chain, srcPortID, dstPortID, order, version string) (string, bool) {
	// TODO: add appropriate offset and limits, along with retries
	channelsResp, err := source.ChainProvider.QueryChannels()
	if err != nil {
		if source.debug {
			source.Log(fmt.Sprintf("Error: querying channels on %s failed: %v", source.ChainID(), err))
		}
		return "", false
	}

	for _, channel := range channelsResp {
		if IsMatchingChannel(source, channel, srcPortID, dstPortID, order, version) {
			// unused channel found
			return channel.ChannelId, true
		}
	}

	return "", false
}

// IsMatchingChannel determines if given channel matches required conditions
func IsMatchingChannel(source *Chain, channel *chantypes.IdentifiedChannel, srcPortID, dstPortID, order, version string) bool {
	return channel.Ordering == OrderFromString(order) &&
		IsConnectionFound(channel.ConnectionHops, source.ConnectionID()) &&
		channel.Version == version &&
		channel.PortId == srcPortID &&
		channel.Counterparty.PortId == dstPortID &&
		(((channel.State == chantypes.INIT || channel.State == chantypes.TRYOPEN) && channel.Counterparty.ChannelId == "") ||
			(channel.State == chantypes.OPEN && (channel.Counterparty.ChannelId == "" || channel.Counterparty.PortId == dstPortID)))
}

// IsConnectionFound determines if given connectionId is present in channel connectionHops list
func IsConnectionFound(connectionHops []string, connectionID string) bool {
	for _, id := range connectionHops {
		if id == connectionID {
			return true
		}
	}
	return false
}

// ValidateChannelParams validates a set of port-ids as well as the order
func ValidateChannelParams(srcPortID, dstPortID, order string) error {
	if err := host.PortIdentifierValidator(srcPortID); err != nil {
		return err
	}
	if err := host.PortIdentifierValidator(dstPortID); err != nil {
		return err
	}
	if (OrderFromString(order) == chantypes.ORDERED) || (OrderFromString(order) == chantypes.UNORDERED) {
		return nil
	}
	return fmt.Errorf("invalid order input (%s), order must be 'ordered' or 'unordered'", order)
}
