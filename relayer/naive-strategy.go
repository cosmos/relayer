package relayer

import (
	"context"
	"fmt"
	"sync"

	"github.com/avast/retry-go/v4"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	"go.uber.org/zap"
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
	return len(rs.Src) == 0 && len(rs.Dst) == 0
}
