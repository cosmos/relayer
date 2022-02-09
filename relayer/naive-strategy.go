package relayer

import (
	"fmt"

	"github.com/avast/retry-go"
	chantypes "github.com/cosmos/ibc-go/v2/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v2/modules/core/exported"
	"github.com/cosmos/relayer/relayer/provider"
	"golang.org/x/sync/errgroup"
)

// UnrelayedSequences returns the unrelayed sequence numbers between two chains
func UnrelayedSequences(src, dst *Chain) (*RelaySequences, error) {
	var (
		eg           = new(errgroup.Group)
		srcPacketSeq = []uint64{}
		dstPacketSeq = []uint64{}
		rs           = &RelaySequences{Src: []uint64{}, Dst: []uint64{}}
	)

	srch, dsth, err := QueryLatestHeights(src, dst)
	if err != nil {
		return nil, err
	}

	eg.Go(func() error {
		var (
			res *chantypes.QueryPacketCommitmentsResponse
			err error
		)
		if err = retry.Do(func() error {
			// Query the packet commitment
			res, err = src.ChainProvider.QueryPacketCommitments(uint64(srch), src.PathEnd.ChannelID, src.PathEnd.PortID)
			switch {
			case err != nil:
				return err
			case res == nil:
				return fmt.Errorf("no error on QueryPacketCommitments for %s, however response is nil", src.ChainID())
			default:
				return nil
			}
		}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
			srch, _ = src.ChainProvider.QueryLatestHeight()
		})); err != nil {
			return err
		}
		for _, pc := range res.Commitments {
			srcPacketSeq = append(srcPacketSeq, pc.Sequence)
		}
		return nil
	})

	eg.Go(func() error {
		var (
			res *chantypes.QueryPacketCommitmentsResponse
			err error
		)
		if err = retry.Do(func() error {
			res, err = dst.ChainProvider.QueryPacketCommitments(uint64(dsth), dst.PathEnd.ChannelID, dst.PathEnd.PortID)
			switch {
			case err != nil:
				return err
			case res == nil:
				return fmt.Errorf("no error on QueryPacketCommitments for %s, however response is nil", dst.ChainID())
			default:
				return nil
			}
		}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
			dsth, _ = dst.ChainProvider.QueryLatestHeight()
		})); err != nil {
			return err
		}
		for _, pc := range res.Commitments {
			dstPacketSeq = append(dstPacketSeq, pc.Sequence)
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	eg.Go(func() error {
		// Query all packets sent by src that have been received by dst
		return retry.Do(func() error {
			var err error
			rs.Src, err = dst.ChainProvider.QueryUnreceivedPackets(uint64(dsth), dst.PathEnd.ChannelID, dst.PathEnd.PortID, srcPacketSeq)
			return err
		}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
			dsth, _ = dst.ChainProvider.QueryLatestHeight()
		}))
	})

	eg.Go(func() error {
		// Query all packets sent by dst that have been received by src
		return retry.Do(func() error {
			var err error
			rs.Dst, err = src.ChainProvider.QueryUnreceivedPackets(uint64(srch), src.PathEnd.ChannelID, src.PathEnd.PortID, dstPacketSeq)
			return err
		}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
			dsth, _ = dst.ChainProvider.QueryLatestHeight()
		}))
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return rs, nil
}

// UnrelayedAcknowledgements returns the unrelayed sequence numbers between two chains
func UnrelayedAcknowledgements(src, dst *Chain) (*RelaySequences, error) {
	var (
		eg           = new(errgroup.Group)
		srcPacketSeq = []uint64{}
		dstPacketSeq = []uint64{}
		rs           = &RelaySequences{Src: []uint64{}, Dst: []uint64{}}
	)

	srch, dsth, err := QueryLatestHeights(src, dst)
	if err != nil {
		return nil, err
	}

	eg.Go(func() error {
		var (
			res []*chantypes.PacketState
			err error
		)
		if err = retry.Do(func() error {
			// Query the packet commitment
			res, err = src.ChainProvider.QueryPacketAcknowledgements(uint64(srch), src.PathEnd.ChannelID, src.PathEnd.PortID)
			switch {
			case err != nil:
				return err
			case res == nil:
				return src.errQueryUnrelayedPacketAcks()
			default:
				return nil
			}
		}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
			srch, _ = src.ChainProvider.QueryLatestHeight()
		})); err != nil {
			return err
		}
		for _, pc := range res {
			srcPacketSeq = append(srcPacketSeq, pc.Sequence)
		}
		return nil
	})

	eg.Go(func() error {
		var (
			res []*chantypes.PacketState
			err error
		)
		if err = retry.Do(func() error {
			res, err = dst.ChainProvider.QueryPacketAcknowledgements(uint64(dsth), dst.PathEnd.ChannelID, dst.PathEnd.PortID)
			switch {
			case err != nil:
				return err
			case res == nil:
				return dst.errQueryUnrelayedPacketAcks()
			default:
				return nil
			}
		}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
			dsth, _ = dst.ChainProvider.QueryLatestHeight()
		})); err != nil {
			return err
		}
		for _, pc := range res {
			dstPacketSeq = append(dstPacketSeq, pc.Sequence)
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	eg.Go(func() error {
		// Query all packets sent by src that have been received by dst
		var err error
		return retry.Do(func() error {
			rs.Src, err = dst.ChainProvider.QueryUnreceivedAcknowledgements(uint64(dsth), dst.PathEnd.ChannelID, dst.PathEnd.PortID, srcPacketSeq)
			return err
		}, RtyErr, RtyAtt, RtyDel, retry.OnRetry(func(n uint, err error) {
			dsth, _ = dst.ChainProvider.QueryLatestHeight()
		}))
	})

	eg.Go(func() error {
		// Query all packets sent by dst that have been received by src
		var err error
		return retry.Do(func() error {
			rs.Dst, err = src.ChainProvider.QueryUnreceivedAcknowledgements(uint64(srch), src.PathEnd.ChannelID, src.PathEnd.PortID, dstPacketSeq)
			return err
		}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
			srch, _ = src.ChainProvider.QueryLatestHeight()
		}))
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return rs, nil
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
func RelayAcknowledgements(src, dst *Chain, sp *RelaySequences, maxTxSize, maxMsgLength uint64) error {
	// set the maximum relay transaction constraints
	msgs := &RelayMsgs{
		Src:          []provider.RelayerMessage{},
		Dst:          []provider.RelayerMessage{},
		MaxTxSize:    maxTxSize,
		MaxMsgLength: maxMsgLength,
	}

	srch, dsth, err := QueryLatestHeights(src, dst)
	if err != nil {
		return err
	}

	var (
		eg                   errgroup.Group
		srcHeader, dstHeader ibcexported.Header
	)
	eg.Go(func() error {
		var err error
		srcHeader, err = src.ChainProvider.GetIBCUpdateHeader(srch, dst.ChainProvider, dst.PathEnd.ClientID)
		return err
	})
	eg.Go(func() error {
		var err error
		dstHeader, err = dst.ChainProvider.GetIBCUpdateHeader(dsth, src.ChainProvider, src.PathEnd.ClientID)
		return err
	})
	if err := eg.Wait(); err != nil {
		return err
	}

	srcUpdateMsg, err := src.ChainProvider.UpdateClient(src.PathEnd.ClientID, dstHeader)
	if err != nil {
		return err
	}
	dstUpdateMsg, err := dst.ChainProvider.UpdateClient(dst.PathEnd.ClientID, srcHeader)
	if err != nil {
		return err
	}

	// add messages for received packets on dst
	for _, seq := range sp.Dst {
		// dst wrote the ack. acknowledgementFromSequence will query the acknowledgement
		// from the counterparty chain (second chain provided in the arguments). The message
		// should be sent to src.
		relayAckMsgs, err := src.ChainProvider.AcknowledgementFromSequence(dst.ChainProvider, uint64(dsth), seq, dst.PathEnd.ChannelID, dst.PathEnd.PortID, src.PathEnd.ChannelID, src.PathEnd.PortID)
		if err != nil {
			return err
		}

		msgs.Src = append(msgs.Src, relayAckMsgs)
	}

	// add messages for received packets on src
	for _, seq := range sp.Src {
		// src wrote the ack. acknowledgementFromSequence will query the acknowledgement
		// from the counterparty chain (second chain provided in the arguments). The message
		// should be sent to dst.
		relayAckMsgs, err := dst.ChainProvider.AcknowledgementFromSequence(src.ChainProvider, uint64(srch), seq, src.PathEnd.ChannelID, src.PathEnd.PortID, dst.PathEnd.ChannelID, dst.PathEnd.PortID)
		if err != nil {
			return err
		}

		msgs.Dst = append(msgs.Dst, relayAckMsgs)
	}

	if !msgs.Ready() {
		src.Log(fmt.Sprintf("- No acknowledgements to relay between [%s]port{%s} and [%s]port{%s}",
			src.ChainID(), src.PathEnd.PortID, dst.ChainID(), dst.PathEnd.PortID))
		return nil
	}

	// Prepend non-empty msg lists with UpdateClient
	if len(msgs.Dst) != 0 {
		msgs.Dst = append([]provider.RelayerMessage{dstUpdateMsg}, msgs.Dst...)
	}

	if len(msgs.Src) != 0 {
		msgs.Src = append([]provider.RelayerMessage{srcUpdateMsg}, msgs.Src...)
	}

	// send messages to their respective chains
	if msgs.Send(src, dst); msgs.Success() {
		if len(msgs.Dst) > 1 {
			dst.logPacketsRelayed(src, len(msgs.Dst)-1)
		}
		if len(msgs.Src) > 1 {
			src.logPacketsRelayed(dst, len(msgs.Src)-1)
		}
	}

	return nil
}

// RelayPackets creates transactions to relay packets from src to dst and from dst to src
func RelayPackets(src, dst *Chain, sp *RelaySequences, maxTxSize, maxMsgLength uint64) error {
	// set the maximum relay transaction constraints
	msgs := &RelayMsgs{
		Src:          []provider.RelayerMessage{},
		Dst:          []provider.RelayerMessage{},
		MaxTxSize:    maxTxSize,
		MaxMsgLength: maxMsgLength,
	}

	srch, dsth, err := QueryLatestHeights(src, dst)
	if err != nil {
		return err
	}

	// add messages for sequences on src
	if err = AddMessagesForSequences(sp.Src, src, dst, srch, dsth, &msgs.Src, &msgs.Dst); err != nil {
		return err
	}

	// add messages for sequences on dst
	if err = AddMessagesForSequences(sp.Dst, dst, src, dsth, srch, &msgs.Dst, &msgs.Src); err != nil {
		return err
	}

	if !msgs.Ready() {
		src.Log(fmt.Sprintf("- No packets to relay between [%s]port{%s} and [%s]port{%s}",
			src.ChainID(), src.PathEnd.PortID, dst.ChainID(), dst.PathEnd.PortID))
		return nil
	}

	// Prepend non-empty msg lists with UpdateClient
	eg := new(errgroup.Group)

	eg.Go(func() error {
		return PrependUpdateClientMsg(&msgs.Dst, src, dst, srch)
	})

	eg.Go(func() error {
		return PrependUpdateClientMsg(&msgs.Src, dst, src, dsth)
	})

	if err = eg.Wait(); err != nil {
		return err
	}

	// send messages to their respective chains
	if msgs.Send(src, dst); msgs.Success() {
		if len(msgs.Dst) > 1 {
			dst.logPacketsRelayed(src, len(msgs.Dst)-1)
		}
		if len(msgs.Src) > 1 {
			src.logPacketsRelayed(dst, len(msgs.Src)-1)
		}
	}

	return nil
}

// AddMessagesForSequences constructs RecvMsgs and TimeoutMsgs from sequence numbers on a src chain
// and adds them to the appropriate queue of msgs for both src and dst
func AddMessagesForSequences(sequences []uint64, src, dst *Chain, srch, dsth int64, srcMsgs, dstMsgs *[]provider.RelayerMessage) error {
	for _, seq := range sequences {

		var (
			recvMsg, timeoutMsg provider.RelayerMessage
			err                 error
		)

		// Query src for the sequence number to get type of packet
		if err = retry.Do(func() error {
			recvMsg, timeoutMsg, err = src.ChainProvider.RelayPacketFromSequence(src.ChainProvider, dst.ChainProvider, uint64(srch), uint64(dsth), seq, dst.PathEnd.ChannelID, dst.PathEnd.PortID, src.PathEnd.ChannelID, src.PathEnd.PortID, src.PathEnd.ClientID)
			return err
		}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
			srch, dsth, _ = QueryLatestHeights(src, dst)
		})); err != nil {
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

// PrependUpdateClientMsg adds an UpdateClient msg to the front of non-empty msg lists
func PrependUpdateClientMsg(msgs *[]provider.RelayerMessage, src, dst *Chain, srch int64) error {
	if len(*msgs) != 0 {
		var (
			srcHeader ibcexported.Header
			updateMsg provider.RelayerMessage
			err       error
		)

		// Query IBC Update Header
		if err = retry.Do(func() error {
			srcHeader, err = src.ChainProvider.GetIBCUpdateHeader(srch, dst.ChainProvider, dst.PathEnd.ClientID)
			return err
		}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
			srch, _, _ = QueryLatestHeights(src, dst)
		})); err != nil {
			return err
		}

		// Construct UpdateClient msg
		if err = retry.Do(func() error {
			updateMsg, err = dst.ChainProvider.UpdateClient(dst.PathEnd.ClientID, srcHeader)
			return nil
		}, RtyAtt, RtyDel, RtyErr); err != nil {
			return err
		}

		// Prepend UpdateClient msg to the slice of msgs
		*msgs = append([]provider.RelayerMessage{updateMsg}, *msgs...)
	}
	return nil
}

// RelayPacket creates transactions to relay packets from src to dst and from dst to src
func RelayPacket(src, dst *Chain, sp *RelaySequences, maxTxSize, maxMsgLength, seqNum uint64) error {
	// set the maximum relay transaction constraints
	msgs := &RelayMsgs{
		Src:          []provider.RelayerMessage{},
		Dst:          []provider.RelayerMessage{},
		MaxTxSize:    maxTxSize,
		MaxMsgLength: maxMsgLength,
	}

	srch, dsth, err := QueryLatestHeights(src, dst)
	if err != nil {
		return err
	}

	// add messages for sequences on src
	for _, seq := range sp.Src {
		if seq == seqNum {
			// Query src for the sequence number to get type of packet
			var recvMsg, timeoutMsg provider.RelayerMessage
			if err = retry.Do(func() error {
				recvMsg, timeoutMsg, err = src.ChainProvider.RelayPacketFromSequence(src.ChainProvider, dst.ChainProvider, uint64(srch), uint64(dsth), seq, dst.PathEnd.ChannelID, dst.PathEnd.PortID, src.PathEnd.ChannelID, src.PathEnd.PortID, src.PathEnd.ClientID)
				if err != nil {
					fmt.Println("Failing to relay packet from seq on src")
				}
				return err
			}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
				srch, dsth, _ = QueryLatestHeights(src, dst)
			})); err != nil {
				return err
			}

			// depending on the type of message to be relayed, we need to
			// send to different chains
			if recvMsg != nil {
				msgs.Dst = append(msgs.Dst, recvMsg)
			}

			if timeoutMsg != nil {
				msgs.Src = append(msgs.Src, timeoutMsg)
			}
		}
	}

	// add messages for sequences on dst
	for _, seq := range sp.Dst {
		if seq == seqNum {
			// Query dst for the sequence number to get type of packet
			var recvMsg, timeoutMsg provider.RelayerMessage
			if err = retry.Do(func() error {
				recvMsg, timeoutMsg, err = dst.ChainProvider.RelayPacketFromSequence(dst.ChainProvider, src.ChainProvider, uint64(dsth), uint64(srch), seq, src.PathEnd.ChannelID, src.PathEnd.PortID, dst.PathEnd.ChannelID, dst.PathEnd.PortID, dst.PathEnd.ClientID)
				if err != nil {
					fmt.Println("Failing to relay packet from seq on dst")
				}
				return nil
			}, RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
				srch, dsth, _ = QueryLatestHeights(src, dst)
			})); err != nil {
				return err
			}

			// depending on the type of message to be relayed, we need to
			// send to different chains
			if recvMsg != nil {
				msgs.Src = append(msgs.Src, recvMsg)
			}

			if timeoutMsg != nil {
				msgs.Dst = append(msgs.Dst, timeoutMsg)
			}
		}
	}

	if !msgs.Ready() {
		src.Log(fmt.Sprintf("- No packets to relay between [%s]port{%s} and [%s]port{%s}",
			src.ChainID(), src.PathEnd.PortID, dst.ChainID(), dst.PathEnd.PortID))
		return nil
	}

	// Prepend non-empty msg lists with UpdateClient
	if len(msgs.Dst) != 0 {
		srcHeader, err := src.ChainProvider.GetIBCUpdateHeader(srch, dst.ChainProvider, dst.PathEnd.ClientID)
		if err != nil {
			return err
		}
		updateMsg, err := dst.ChainProvider.UpdateClient(dst.PathEnd.ClientID, srcHeader)
		if err != nil {
			return err
		}

		msgs.Dst = append([]provider.RelayerMessage{updateMsg}, msgs.Dst...)
	}

	if len(msgs.Src) != 0 {
		dstHeader, err := dst.ChainProvider.GetIBCUpdateHeader(dsth, src.ChainProvider, src.PathEnd.ClientID)
		if err != nil {
			return err
		}
		updateMsg, err := src.ChainProvider.UpdateClient(src.PathEnd.ClientID, dstHeader)
		if err != nil {
			return err
		}

		msgs.Src = append([]provider.RelayerMessage{updateMsg}, msgs.Src...)
	}

	// send messages to their respective chains
	if msgs.Send(src, dst); msgs.Success() {
		if len(msgs.Dst) > 1 {
			dst.logPacketsRelayed(src, len(msgs.Dst)-1)
		}
		if len(msgs.Src) > 1 {
			src.logPacketsRelayed(dst, len(msgs.Src)-1)
		}
	} else {
		fmt.Println()
	}

	return nil
}
