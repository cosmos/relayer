package relayer

import (
	"context"
	"fmt"
	"sync"

	"github.com/avast/retry-go/v4"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// UnrelayedSequences returns the unrelayed sequence numbers between two chains
func UnrelayedSequences(ctx context.Context, src, dst *Chain, srcChannel *chantypes.IdentifiedChannel) RelaySequences {
	var (
		srcPacketSeq = []uint64{}
		dstPacketSeq = []uint64{}
		rs           = RelaySequences{Src: []uint64{}, Dst: []uint64{}}
	)

	srch, dsth, err := QueryLatestHeights(ctx, src, dst)
	if err != nil {
		src.log.Error("Error querying latest heights", zap.Error(err))
		return rs
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		var (
			res *chantypes.QueryPacketCommitmentsResponse
			err error
		)
		if err = retry.Do(func() error {
			// Query the packet commitment
			res, err = src.ChainProvider.QueryPacketCommitments(ctx, uint64(srch), srcChannel.ChannelId, srcChannel.PortId)
			switch {
			case err != nil:
				return err
			case res == nil:
				return fmt.Errorf("no error on QueryPacketCommitments for %s, however response is nil", src.ChainID())
			default:
				return nil
			}
		}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
			src.log.Info(
				"Failed to query packet commitments",
				zap.String("channel_id", srcChannel.ChannelId),
				zap.String("port_id", srcChannel.PortId),
				zap.Uint("attempt", n+1),
				zap.Uint("max_attempts", RtyAttNum),
				zap.Error(err),
			)
		})); err != nil {
			src.log.Error(
				"Failed to query packet commitments after max retries",
				zap.String("channel_id", srcChannel.ChannelId),
				zap.String("port_id", srcChannel.PortId),
				zap.Uint("attempts", RtyAttNum),
				zap.Error(err),
			)
			return
		}

		for _, pc := range res.Commitments {
			srcPacketSeq = append(srcPacketSeq, pc.Sequence)
		}
	}()

	go func() {
		defer wg.Done()
		var (
			res *chantypes.QueryPacketCommitmentsResponse
			err error
		)
		if err = retry.Do(func() error {
			res, err = dst.ChainProvider.QueryPacketCommitments(ctx, uint64(dsth), srcChannel.Counterparty.ChannelId, srcChannel.Counterparty.PortId)
			switch {
			case err != nil:
				return err
			case res == nil:
				return fmt.Errorf("no error on QueryPacketCommitments for %s, however response is nil", dst.ChainID())
			default:
				return nil
			}
		}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
			dst.log.Info(
				"Failed to query packet commitments",
				zap.String("channel_id", srcChannel.Counterparty.ChannelId),
				zap.String("port_id", srcChannel.Counterparty.PortId),
				zap.Uint("attempt", n+1),
				zap.Uint("max_attempts", RtyAttNum),
				zap.Error(err),
			)
		})); err != nil {
			dst.log.Error(
				"Failed to query packet commitments after max retries",
				zap.String("channel_id", srcChannel.Counterparty.ChannelId),
				zap.String("port_id", srcChannel.Counterparty.PortId),
				zap.Uint("attempts", RtyAttNum),
				zap.Error(err),
			)
			return
		}

		for _, pc := range res.Commitments {
			dstPacketSeq = append(dstPacketSeq, pc.Sequence)
		}
	}()

	wg.Wait()

	var (
		srcUnreceivedPackets, dstUnreceivedPackets []uint64
	)

	if len(srcPacketSeq) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Query all packets sent by src that have not been received by dst.
			if err := retry.Do(func() error {
				var err error
				srcUnreceivedPackets, err = dst.ChainProvider.QueryUnreceivedPackets(ctx, uint64(dsth), srcChannel.Counterparty.ChannelId, srcChannel.Counterparty.PortId, srcPacketSeq)
				return err
			}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
				dst.log.Info(
					"Failed to query unreceived packets",
					zap.String("channel_id", srcChannel.Counterparty.ChannelId),
					zap.String("port_id", srcChannel.Counterparty.PortId),
					zap.Uint("attempt", n+1),
					zap.Uint("max_attempts", RtyAttNum),
					zap.Error(err),
				)
			})); err != nil {
				dst.log.Error(
					"Failed to query unreceived packets after max retries",
					zap.String("channel_id", srcChannel.Counterparty.ChannelId),
					zap.String("port_id", srcChannel.Counterparty.PortId),
					zap.Uint("attempts", RtyAttNum),
					zap.Error(err),
				)
			}
		}()
	}

	if len(dstPacketSeq) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Query all packets sent by dst that have not been received by src.
			if err := retry.Do(func() error {
				var err error
				dstUnreceivedPackets, err = src.ChainProvider.QueryUnreceivedPackets(ctx, uint64(srch), srcChannel.ChannelId, srcChannel.PortId, dstPacketSeq)
				return err
			}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
				src.log.Info(
					"Failed to query unreceived packets",
					zap.String("channel_id", srcChannel.ChannelId),
					zap.String("port_id", srcChannel.PortId),
					zap.Uint("attempt", n+1),
					zap.Uint("max_attempts", RtyAttNum),
					zap.Error(err),
				)
			})); err != nil {
				src.log.Error(
					"Failed to query unreceived packets after max retries",
					zap.String("channel_id", srcChannel.ChannelId),
					zap.String("port_id", srcChannel.PortId),
					zap.Uint("attempts", RtyAttNum),
					zap.Error(err),
				)
				return
			}
		}()
	}
	wg.Wait()

	// If this is an UNORDERED channel we can return at this point.
	if srcChannel.Ordering != chantypes.ORDERED {
		rs.Src = srcUnreceivedPackets
		rs.Dst = dstUnreceivedPackets
		return rs
	}

	// For ordered channels we want to only relay the packet whose sequence number is equal to
	// the expected next packet receive sequence from the counterparty.
	if len(srcUnreceivedPackets) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			nextSeqResp, err := dst.ChainProvider.QueryNextSeqRecv(ctx, dsth, srcChannel.Counterparty.ChannelId, srcChannel.Counterparty.PortId)
			if err != nil {
				dst.log.Error(
					"Failed to query next packet receive sequence",
					zap.String("channel_id", srcChannel.Counterparty.ChannelId),
					zap.String("port_id", srcChannel.Counterparty.PortId),
					zap.Error(err),
				)
				return
			}

			for _, seq := range srcUnreceivedPackets {
				if seq == nextSeqResp.NextSequenceReceive {
					rs.Src = append(rs.Src, seq)
					break
				}
			}
		}()
	}

	if len(dstUnreceivedPackets) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			nextSeqResp, err := src.ChainProvider.QueryNextSeqRecv(ctx, srch, srcChannel.ChannelId, srcChannel.PortId)
			if err != nil {
				src.log.Error(
					"Failed to query next packet receive sequence",
					zap.String("channel_id", srcChannel.ChannelId),
					zap.String("port_id", srcChannel.PortId),
					zap.Error(err),
				)
				return
			}

			for _, seq := range dstUnreceivedPackets {
				if seq == nextSeqResp.NextSequenceReceive {
					rs.Dst = append(rs.Dst, seq)
					break
				}
			}
		}()
	}
	wg.Wait()

	return rs
}

// UnrelayedAcknowledgements returns the unrelayed sequence numbers between two chains
func UnrelayedAcknowledgements(ctx context.Context, src, dst *Chain, srcChannel *chantypes.IdentifiedChannel) RelaySequences {
	var (
		srcPacketSeq = []uint64{}
		dstPacketSeq = []uint64{}
		rs           = RelaySequences{Src: []uint64{}, Dst: []uint64{}}
	)

	srch, dsth, err := QueryLatestHeights(ctx, src, dst)
	if err != nil {
		src.log.Error("Error querying latest heights", zap.Error(err))
		return rs
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		var (
			res []*chantypes.PacketState
			err error
		)
		if err = retry.Do(func() error {
			// Query the packet commitment
			res, err = src.ChainProvider.QueryPacketAcknowledgements(ctx, uint64(srch), srcChannel.ChannelId, srcChannel.PortId)
			switch {
			case err != nil:
				return err
			case res == nil:
				return src.errQueryUnrelayedPacketAcks()
			default:
				return nil
			}
		}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr); err != nil {
			src.log.Error(
				"Failed to query packet acknowledgement commitments after max attempts",
				zap.String("channel_id", srcChannel.ChannelId),
				zap.String("port_id", srcChannel.PortId),
				zap.Uint("attempts", RtyAttNum),
				zap.Error(err),
			)
			return
		}
		for _, pc := range res {
			srcPacketSeq = append(srcPacketSeq, pc.Sequence)
		}
	}()

	go func() {
		defer wg.Done()
		var (
			res []*chantypes.PacketState
			err error
		)
		if err = retry.Do(func() error {
			res, err = dst.ChainProvider.QueryPacketAcknowledgements(ctx, uint64(dsth), srcChannel.Counterparty.ChannelId, srcChannel.Counterparty.PortId)
			switch {
			case err != nil:
				return err
			case res == nil:
				return dst.errQueryUnrelayedPacketAcks()
			default:
				return nil
			}
		}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr); err != nil {
			dst.log.Error(
				"Failed to query packet acknowledgement commitments after max attempts",
				zap.String("channel_id", srcChannel.Counterparty.ChannelId),
				zap.String("port_id", srcChannel.Counterparty.PortId),
				zap.Uint("attempts", RtyAttNum),
				zap.Error(err),
			)
			return
		}
		for _, pc := range res {
			dstPacketSeq = append(dstPacketSeq, pc.Sequence)
		}
	}()

	wg.Wait()

	if len(srcPacketSeq) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Query all packets sent by src that have been received by dst
			var err error
			if err := retry.Do(func() error {
				rs.Src, err = dst.ChainProvider.QueryUnreceivedAcknowledgements(ctx, uint64(dsth), srcChannel.Counterparty.ChannelId, srcChannel.Counterparty.PortId, srcPacketSeq)
				return err
			}, retry.Context(ctx), RtyErr, RtyAtt, RtyDel); err != nil {
				dst.log.Error(
					"Failed to query unreceived acknowledgements after max attempts",
					zap.String("channel_id", srcChannel.Counterparty.ChannelId),
					zap.String("port_id", srcChannel.Counterparty.PortId),
					zap.Uint("attempts", RtyAttNum),
					zap.Error(err),
				)
			}
		}()
	}

	if len(dstPacketSeq) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Query all packets sent by dst that have been received by src
			var err error
			if err := retry.Do(func() error {
				rs.Dst, err = src.ChainProvider.QueryUnreceivedAcknowledgements(ctx, uint64(srch), srcChannel.ChannelId, srcChannel.PortId, dstPacketSeq)
				return err
			}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr); err != nil {
				src.log.Error(
					"Failed to query unreceived acknowledgements after max attempts",
					zap.String("channel_id", srcChannel.ChannelId),
					zap.String("port_id", srcChannel.PortId),
					zap.Uint("attempts", RtyAttNum),
					zap.Error(err),
				)
			}
		}()
	}

	wg.Wait()

	return rs
}

// RelaySequences represents unrelayed packets on src and dst
type RelaySequences struct {
	Src []uint64 `json:"src"`
	Dst []uint64 `json:"dst"`
}

func (rs *RelaySequences) Empty() bool {
	if len(rs.Src) == 0 && len(rs.Dst) == 0 {
		return true
	}
	return false
}

// RelayAcknowledgements creates transactions to relay acknowledgements from src to dst and from dst to src
func RelayAcknowledgements(ctx context.Context, log *zap.Logger, src, dst *Chain, sp RelaySequences, maxTxSize, maxMsgLength uint64, memo string, srcChannel *chantypes.IdentifiedChannel) error {
	srch, dsth, err := QueryLatestHeights(ctx, src, dst)
	if err != nil {
		return err
	}

	// set the maximum relay transaction constraints
	msgs := &RelayMsgs{
		Src:          []provider.RelayerMessage{},
		Dst:          []provider.RelayerMessage{},
		MaxTxSize:    maxTxSize,
		MaxMsgLength: maxMsgLength,
	}

	// add messages for received packets on dst
	for _, seq := range sp.Dst {
		// dst wrote the ack. acknowledgementFromSequence will query the acknowledgement
		// from the counterparty chain (second chain provided in the arguments). The message
		// should be sent to src.
		relayAckMsgs, err := src.ChainProvider.AcknowledgementFromSequence(ctx, dst.ChainProvider, uint64(dsth), seq, srcChannel.Counterparty.ChannelId, srcChannel.Counterparty.PortId, srcChannel.ChannelId, srcChannel.PortId)
		if err != nil {
			return err
		}

		// Do not allow nil messages to the queued, or else we will panic in send()
		if relayAckMsgs != nil {
			msgs.Src = append(msgs.Src, relayAckMsgs)
		}
	}

	// add messages for received packets on src
	for _, seq := range sp.Src {
		// src wrote the ack. acknowledgementFromSequence will query the acknowledgement
		// from the counterparty chain (second chain provided in the arguments). The message
		// should be sent to dst.
		relayAckMsgs, err := dst.ChainProvider.AcknowledgementFromSequence(ctx, src.ChainProvider, uint64(srch), seq, srcChannel.ChannelId, srcChannel.PortId, srcChannel.Counterparty.ChannelId, srcChannel.Counterparty.PortId)
		if err != nil {
			return err
		}

		// Do not allow nil messages to the queued, or else we will panic in send()
		if relayAckMsgs != nil {
			msgs.Dst = append(msgs.Dst, relayAckMsgs)
		}
	}

	if !msgs.Ready() {
		log.Info(
			"No acknowledgements to relay",
			zap.String("src_chain_id", src.ChainID()),
			zap.String("src_port_id", srcChannel.PortId),
			zap.String("dst_chain_id", dst.ChainID()),
			zap.String("dst_port_id", srcChannel.Counterparty.PortId),
		)
		return nil
	}

	if err := msgs.PrependMsgUpdateClient(ctx, src, dst, srch, dsth); err != nil {
		return err
	}

	// send messages to their respective chains
	result := msgs.Send(ctx, log, AsRelayMsgSender(src), AsRelayMsgSender(dst), memo)
	if err := result.Error(); err != nil {
		if result.PartiallySent() {
			log.Info(
				"Partial success when relaying acknowledgements",
				zap.String("src_chain_id", src.ChainID()),
				zap.String("src_port_id", srcChannel.PortId),
				zap.String("dst_chain_id", dst.ChainID()),
				zap.String("dst_port_id", srcChannel.Counterparty.PortId),
				zap.Error(err),
			)
		}
		return err
	}

	if result.SuccessfulSrcBatches > 0 {
		src.logPacketsRelayed(dst, result.SuccessfulSrcBatches, srcChannel)
	}
	if result.SuccessfulDstBatches > 0 {
		dst.logPacketsRelayed(src, result.SuccessfulDstBatches, srcChannel)
	}

	return nil
}

// RelayPackets creates transactions to relay packets from src to dst and from dst to src
func RelayPackets(ctx context.Context, log *zap.Logger, src, dst *Chain, sp RelaySequences, maxTxSize, maxMsgLength uint64, memo string, srcChannel *chantypes.IdentifiedChannel) error {
	srch, dsth, err := QueryLatestHeights(ctx, src, dst)
	if err != nil {
		return err
	}

	eg, egCtx := errgroup.WithContext(ctx)
	// add messages for sequences on src
	var msgsSrc1, msgsDst1 []provider.RelayerMessage
	eg.Go(func() error {
		return AddMessagesForSequences(egCtx, sp.Src, src, dst, srch, dsth, &msgsSrc1, &msgsDst1,
			srcChannel.ChannelId, srcChannel.PortId, srcChannel.Counterparty.ChannelId, srcChannel.Counterparty.PortId, srcChannel.Ordering)
	})

	var msgsSrc2, msgsDst2 []provider.RelayerMessage
	// add messages for sequences on dst
	eg.Go(func() error {
		return AddMessagesForSequences(egCtx, sp.Dst, dst, src, dsth, srch, &msgsDst2, &msgsSrc2,
			srcChannel.Counterparty.ChannelId, srcChannel.Counterparty.PortId, srcChannel.ChannelId, srcChannel.PortId, srcChannel.Ordering)
	})

	if err = eg.Wait(); err != nil {
		return err
	}

	// set the maximum relay transaction constraints
	msgs := &RelayMsgs{
		Src:          append(msgsSrc1, msgsSrc2...),
		Dst:          append(msgsDst1, msgsDst2...),
		MaxTxSize:    maxTxSize,
		MaxMsgLength: maxMsgLength,
	}

	if !msgs.Ready() {
		log.Info(
			"No packets to relay",
			zap.String("src_chain_id", src.ChainID()),
			zap.String("src_port_id", srcChannel.PortId),
			zap.String("dst_chain_id", dst.ChainID()),
			zap.String("dst_port_id", srcChannel.Counterparty.PortId),
		)
		return nil
	}

	if err := msgs.PrependMsgUpdateClient(ctx, src, dst, srch, dsth); err != nil {
		return err
	}

	// send messages to their respective chains
	result := msgs.Send(ctx, log, AsRelayMsgSender(src), AsRelayMsgSender(dst), memo)
	if err := result.Error(); err != nil {
		if result.PartiallySent() {
			log.Info(
				"Partial success when relaying packets",
				zap.String("src_chain_id", src.ChainID()),
				zap.String("src_port_id", srcChannel.PortId),
				zap.String("dst_chain_id", dst.ChainID()),
				zap.String("dst_port_id", srcChannel.Counterparty.PortId),
				zap.Error(err),
			)
		}
		return err
	}

	if result.SuccessfulSrcBatches > 0 {
		src.logPacketsRelayed(dst, result.SuccessfulSrcBatches, srcChannel)
	}
	if result.SuccessfulDstBatches > 0 {
		dst.logPacketsRelayed(src, result.SuccessfulDstBatches, srcChannel)
	}

	return nil

}

// AddMessagesForSequences constructs RecvMsgs and TimeoutMsgs from sequence numbers on a src chain
// and adds them to the appropriate queue of msgs for both src and dst
func AddMessagesForSequences(
	ctx context.Context,
	sequences []uint64,
	src, dst *Chain,
	srch, dsth int64,
	srcMsgs, dstMsgs *[]provider.RelayerMessage,
	srcChanID, srcPortID, dstChanID, dstPortID string,
	order chantypes.Order,
) error {
	for _, seq := range sequences {
		recvMsg, timeoutMsg, err := dst.ChainProvider.RelayPacketFromSequence(
			ctx,
			src.ChainProvider,
			uint64(srch), uint64(dsth),
			seq,
			srcChanID, srcPortID,
			order,
		)
		if err != nil {
			src.log.Info(
				"Failed to relay packet from sequence",
				zap.String("src_chain_id", src.ChainID()),
				zap.String("src_channel_id", srcChanID),
				zap.String("src_port_id", srcPortID),
				zap.String("dst_chain_id", dst.ChainID()),
				zap.String("dst_channel_id", dstChanID),
				zap.String("dst_port_id", dstPortID),
				zap.String("channel_order", order.String()),
				zap.Error(err),
			)
			return err
		}

		// Depending on the type of message to be relayed, we need to send to different chains
		if recvMsg != nil {
			*dstMsgs = append(*dstMsgs, recvMsg)
		}

		if timeoutMsg != nil {
			*srcMsgs = append(*srcMsgs, timeoutMsg)
		}
	}

	return nil
}
