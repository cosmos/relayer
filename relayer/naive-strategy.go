package relayer

import (
	"fmt"
	"strconv"

	retry "github.com/avast/retry-go"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v2/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v2/modules/core/04-channel/types"
	tmclient "github.com/cosmos/ibc-go/v2/modules/light-clients/07-tendermint/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"golang.org/x/sync/errgroup"
)

var (
	// Strings for parsing events
	spTag       = "send_packet"
	waTag       = "write_acknowledgement"
	srcChanTag  = "packet_src_channel"
	dstChanTag  = "packet_dst_channel"
	srcPortTag  = "packet_src_port"
	dstPortTag  = "packet_dst_port"
	dataTag     = "packet_data"
	ackTag      = "packet_ack"
	toHeightTag = "packet_timeout_height"
	toTSTag     = "packet_timeout_timestamp"
	seqTag      = "packet_sequence"
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
			res, err = src.QueryPacketCommitments(DefaultPageRequest(), uint64(srch))
			switch {
			case err != nil:
				return err
			case res == nil:
				return fmt.Errorf("no error on QueryPacketCommitments for %s, however response is nil", src.ChainID)
			default:
				return nil
			}
		}, rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
			srch, _ = src.QueryLatestHeight()
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
			res, err = dst.QueryPacketCommitments(DefaultPageRequest(), uint64(dsth))
			switch {
			case err != nil:
				return err
			case res == nil:
				return fmt.Errorf("no error on QueryPacketCommitments for %s, however response is nil", dst.ChainID)
			default:
				return nil
			}
		}, rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
			dsth, _ = dst.QueryLatestHeight()
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
			rs.Src, err = dst.QueryUnreceivedPackets(uint64(dsth), srcPacketSeq)
			return err
		}, rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
			dsth, _ = dst.QueryLatestHeight()
		}))
	})

	eg.Go(func() error {
		// Query all packets sent by dst that have been received by src
		return retry.Do(func() error {
			var err error
			rs.Dst, err = src.QueryUnreceivedPackets(uint64(srch), dstPacketSeq)
			return err
		}, rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
			dsth, _ = dst.QueryLatestHeight()
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
		var res *chantypes.QueryPacketAcknowledgementsResponse
		if err = retry.Do(func() error {
			// Query the packet commitment
			res, err = src.QueryPacketAcknowledgements(DefaultPageRequest(), uint64(srch))
			switch {
			case err != nil:
				return err
			case res == nil:
				return src.errQueryUnrelayedPacketAcks()
			default:
				return nil
			}
		}, rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
			srch, _ = src.QueryLatestHeight()
		})); err != nil {
			return err
		}
		for _, pc := range res.Acknowledgements {
			srcPacketSeq = append(srcPacketSeq, pc.Sequence)
		}
		return nil
	})

	eg.Go(func() error {
		var res *chantypes.QueryPacketAcknowledgementsResponse
		if err = retry.Do(func() error {
			res, err = dst.QueryPacketAcknowledgements(DefaultPageRequest(), uint64(dsth))
			switch {
			case err != nil:
				return err
			case res == nil:
				return dst.errQueryUnrelayedPacketAcks()
			default:
				return nil
			}
		}, rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
			dsth, _ = dst.QueryLatestHeight()
		})); err != nil {
			return err
		}
		for _, pc := range res.Acknowledgements {
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
			rs.Src, err = dst.QueryUnreceivedAcknowledgements(uint64(dsth), srcPacketSeq)
			return err
		}, rtyErr, rtyAtt, rtyDel, retry.OnRetry(func(n uint, err error) {
			dsth, _ = dst.QueryLatestHeight()
		}))
	})

	eg.Go(func() error {
		// Query all packets sent by dst that have been received by src
		return retry.Do(func() error {
			rs.Dst, err = src.QueryUnreceivedAcknowledgements(uint64(srch), dstPacketSeq)
			return err
		}, rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
			srch, _ = src.QueryLatestHeight()
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
		Src:          []sdk.Msg{},
		Dst:          []sdk.Msg{},
		MaxTxSize:    maxTxSize,
		MaxMsgLength: maxMsgLength,
	}

	srch, dsth, err := QueryLatestHeights(src, dst)
	if err != nil {
		return err
	}

	var (
		eg                   errgroup.Group
		srcHeader, dstHeader *tmclient.Header
	)
	eg.Go(func() error {
		srcHeader, err = src.GetIBCUpdateHeader(dst, srch)
		return err
	})
	eg.Go(func() error {
		dstHeader, err = dst.GetIBCUpdateHeader(src, dsth)
		return err
	})
	if err := eg.Wait(); err != nil {
		return err
	}

	srcUpdateMsg, err := src.UpdateClient(dst, dstHeader)
	if err != nil {
		return err
	}
	dstUpdateMsg, err := dst.UpdateClient(src, srcHeader)
	if err != nil {
		return err
	}

	// add messages for received packets on dst
	for _, seq := range sp.Dst {
		// dst wrote the ack. acknowledgementFromSequence will query the acknowledgement
		// from the counterparty chain (second chain provided in the arguments). The message
		// should be sent to src.
		relayAckMsgs, err := acknowledgementFromSequence(src, dst, uint64(dsth), seq)
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
		relayAckMsgs, err := acknowledgementFromSequence(dst, src, uint64(srch), seq)
		if err != nil {
			return err
		}

		msgs.Dst = append(msgs.Dst, relayAckMsgs)
	}

	if !msgs.Ready() {
		src.Log(fmt.Sprintf("- No acknowledgements to relay between [%s]port{%s} and [%s]port{%s}",
			src.ChainID, src.PathEnd.PortID, dst.ChainID, dst.PathEnd.PortID))
		return nil
	}

	// Prepend non-empty msg lists with UpdateClient
	if len(msgs.Dst) != 0 {
		msgs.Dst = append([]sdk.Msg{dstUpdateMsg}, msgs.Dst...)
	}

	if len(msgs.Src) != 0 {
		msgs.Src = append([]sdk.Msg{srcUpdateMsg}, msgs.Src...)
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
		Src:          []sdk.Msg{},
		Dst:          []sdk.Msg{},
		MaxTxSize:    maxTxSize,
		MaxMsgLength: maxMsgLength,
	}

	srch, dsth, err := QueryLatestHeights(src, dst)
	if err != nil {
		return err
	}

	// add messages for sequences on src
	for _, seq := range sp.Src {
		// Query src for the sequence number to get type of packet
		var recvMsg, timeoutMsg sdk.Msg
		if err = retry.Do(func() error {
			recvMsg, timeoutMsg, err = relayPacketFromSequence(src, dst, uint64(srch), uint64(dsth), seq)
			return err
		}, rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
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

	// add messages for sequences on dst
	for _, seq := range sp.Dst {
		// Query dst for the sequence number to get type of packet
		var recvMsg, timeoutMsg sdk.Msg
		if err = retry.Do(func() error {
			recvMsg, timeoutMsg, err = relayPacketFromSequence(dst, src, uint64(dsth), uint64(srch), seq)
			return nil
		}, rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
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

	if !msgs.Ready() {
		src.Log(fmt.Sprintf("- No packets to relay between [%s]port{%s} and [%s]port{%s}",
			src.ChainID, src.PathEnd.PortID, dst.ChainID, dst.PathEnd.PortID))
		return nil
	}

	// Prepend non-empty msg lists with UpdateClient
	if len(msgs.Dst) != 0 {
		srcHeader, err := src.GetIBCUpdateHeader(dst, srch)
		if err != nil {
			return err
		}
		updateMsg, err := dst.UpdateClient(src, srcHeader)
		if err != nil {
			return err
		}

		msgs.Dst = append([]sdk.Msg{updateMsg}, msgs.Dst...)
	}

	if len(msgs.Src) != 0 {
		dstHeader, err := dst.GetIBCUpdateHeader(src, dsth)
		if err != nil {
			return err
		}
		updateMsg, err := src.UpdateClient(dst, dstHeader)
		if err != nil {
			return err
		}

		msgs.Src = append([]sdk.Msg{updateMsg}, msgs.Src...)
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

// relayPacketFromSequence relays a packet with a given seq on src
// and returns recvPacket msgs, timeoutPacketmsgs and error
func relayPacketFromSequence(src, dst *Chain, srch, dsth, seq uint64) (sdk.Msg, sdk.Msg, error) {
	// var packet, timeout sdk.Msg
	txs, err := src.QueryTxs(uint64(srch), 1, 1000, rcvPacketQuery(src.PathEnd.ChannelID, int(seq)))
	switch {
	case err != nil:
		return nil, nil, err
	case len(txs) == 0:
		return nil, nil, fmt.Errorf("no transactions returned with query")
	case len(txs) > 1:
		return nil, nil, fmt.Errorf("more than one transaction returned with query")
	}

	rcvPackets, timeoutPackets, err := relayPacketsFromResultTx(src, dst, int64(dsth), txs[0])
	switch {
	case err != nil:
		return nil, nil, err
	case len(rcvPackets) == 0 && len(timeoutPackets) == 0:
		return nil, nil, fmt.Errorf("no relay msgs created from query response")
	case len(rcvPackets)+len(timeoutPackets) > 1:
		return nil, nil, fmt.Errorf("more than one relay msg found in tx query")
	}

	if len(rcvPackets) == 1 {
		pkt := rcvPackets[0]
		if seq != pkt.Seq() {
			return nil, nil, fmt.Errorf("wrong sequence: expected(%d) got(%d)", seq, pkt.Seq())
		}

		packet, err := dst.MsgRelayRecvPacket(src, int64(srch), pkt.(*relayMsgRecvPacket))
		if err != nil {
			return nil, nil, err
		}

		return packet, nil, nil
	}

	if len(timeoutPackets) == 1 {
		pkt := timeoutPackets[0]
		if seq != pkt.Seq() {
			return nil, nil, fmt.Errorf("wrong sequence: expected(%d) got(%d)", seq, pkt.Seq())
		}

		timeout, err := src.MsgRelayTimeout(dst, int64(dsth), pkt.(*relayMsgTimeout))
		if err != nil {
			return nil, nil, err
		}
		return nil, timeout, nil
	}

	return nil, nil, fmt.Errorf("should have errored before here")
}

// source is the sending chain, destination is the receiving chain
func acknowledgementFromSequence(src, dst *Chain, dsth, seq uint64) (sdk.Msg, error) {
	txs, err := dst.QueryTxs(uint64(dsth), 1, 1000, ackPacketQuery(dst.PathEnd.ChannelID, int(seq)))
	switch {
	case err != nil:
		return nil, err
	case len(txs) == 0:
		return nil, fmt.Errorf("no transactions returned with query")
	case len(txs) > 1:
		return nil, fmt.Errorf("more than one transaction returned with query")
	}

	acks, err := acknowledgementsFromResultTx(src.PathEnd, dst.PathEnd, txs[0])
	switch {
	case err != nil:
		return nil, err
	case len(acks) == 0:
		return nil, fmt.Errorf("no ack msgs created from query response")
	}

	var out sdk.Msg
	for _, ack := range acks {
		if seq != ack.Seq() {
			continue
		}
		msg, err := src.MsgRelayAcknowledgement(dst, int64(dsth), ack)
		if err != nil {
			return nil, err
		}
		out = msg
	}
	return out, nil
}

// relayPacketsFromResultTx looks through the events in a *ctypes.ResultTx
// and returns relayPackets with the appropriate data
func relayPacketsFromResultTx(src, dst *Chain, dsth int64, res *ctypes.ResultTx) ([]relayPacket, []relayPacket, error) {
	var (
		rcvPackets     []relayPacket
		timeoutPackets []relayPacket
	)

	srcPE := src.PathEnd
	dstPE := dst.PathEnd

	for _, e := range res.TxResult.Events {
		if e.Type == spTag {
			// NOTE: Src and Dst are switched here
			rp := &relayMsgRecvPacket{pass: false}
			for _, p := range e.Attributes {
				if string(p.Key) == srcChanTag {
					if string(p.Value) != srcPE.ChannelID {
						rp.pass = true
						continue
					}
				}
				if string(p.Key) == dstChanTag {
					if string(p.Value) != dstPE.ChannelID {
						rp.pass = true
						continue
					}
				}
				if string(p.Key) == srcPortTag {
					if string(p.Value) != srcPE.PortID {
						rp.pass = true
						continue
					}
				}
				if string(p.Key) == dstPortTag {
					if string(p.Value) != dstPE.PortID {
						rp.pass = true
						continue
					}
				}
				if string(p.Key) == dataTag {
					rp.packetData = p.Value
				}
				if string(p.Key) == toHeightTag {
					timeout, err := clienttypes.ParseHeight(string(p.Value))
					if err != nil {
						return nil, nil, err
					}

					rp.timeout = timeout
				}
				if string(p.Key) == toTSTag {
					timeout, _ := strconv.ParseUint(string(p.Value), 10, 64)
					rp.timeoutStamp = timeout
				}
				if string(p.Key) == seqTag {
					seq, _ := strconv.ParseUint(string(p.Value), 10, 64)
					rp.seq = seq
				}
			}

			// fetch the header which represents a block produced on destination
			block, err := dst.GetIBCUpdateHeader(src, dsth)
			if err != nil {
				return nil, nil, err
			}

			switch {
			// If the packet has a timeout height, and it has been reached, return a timeout packet
			case !rp.timeout.IsZero() && block.GetHeight().GTE(rp.timeout):
				timeoutPackets = append(timeoutPackets, rp.timeoutPacket())
			// If the packet has a timeout timestamp and it has been reached, return a timeout packet
			case rp.timeoutStamp != 0 && block.GetTime().UnixNano() >= int64(rp.timeoutStamp):
				timeoutPackets = append(timeoutPackets, rp.timeoutPacket())
			// If the packet matches the relay constraints relay it as a MsgReceivePacket
			case !rp.pass:
				rcvPackets = append(rcvPackets, rp)
			}
		}
	}

	// If there is a relayPacket, return it
	if len(rcvPackets)+len(timeoutPackets) > 0 {
		return rcvPackets, timeoutPackets, nil
	}

	return nil, nil, fmt.Errorf("no packet data found")
}

// acknowledgementsFromResultTx looks through the events in a *ctypes.ResultTx and returns
// relayPackets with the appropriate data
func acknowledgementsFromResultTx(src, dst *PathEnd,
	res *ctypes.ResultTx) ([]*relayMsgPacketAck, error) {
	var ackPackets []*relayMsgPacketAck
	for _, e := range res.TxResult.Events {
		if e.Type == waTag {
			// NOTE: Src and Dst are switched here
			rp := &relayMsgPacketAck{pass: false}
			for _, p := range e.Attributes {
				if string(p.Key) == srcChanTag {
					if string(p.Value) != src.ChannelID {
						rp.pass = true
						continue
					}
				}
				if string(p.Key) == dstChanTag {
					if string(p.Value) != dst.ChannelID {
						rp.pass = true
						continue
					}
				}
				if string(p.Key) == srcPortTag {
					if string(p.Value) != src.PortID {
						rp.pass = true
						continue
					}
				}
				if string(p.Key) == dstPortTag {
					if string(p.Value) != dst.PortID {
						rp.pass = true
						continue
					}
				}
				if string(p.Key) == ackTag {
					rp.ack = p.Value
				}
				if string(p.Key) == dataTag {
					rp.packetData = p.Value
				}
				if string(p.Key) == toHeightTag {
					timeout, err := clienttypes.ParseHeight(string(p.Value))
					if err != nil {
						return nil, err
					}
					rp.timeout = timeout
				}
				if string(p.Key) == toTSTag {
					timeout, _ := strconv.ParseUint(string(p.Value), 10, 64)
					rp.timeoutStamp = timeout
				}
				if string(p.Key) == seqTag {
					seq, _ := strconv.ParseUint(string(p.Value), 10, 64)
					rp.seq = seq
				}
			}
			if !rp.pass {
				ackPackets = append(ackPackets, rp)
			}
		}
	}

	// If there is a relayPacket, return it
	if len(ackPackets) > 0 {
		return ackPackets, nil
	}

	return nil, fmt.Errorf("no packet data found")
}

func rcvPacketQuery(channelID string, seq int) []string {
	return []string{fmt.Sprintf("%s.packet_src_channel='%s'", spTag, channelID),
		fmt.Sprintf("%s.packet_sequence='%d'", spTag, seq)}
}

func ackPacketQuery(channelID string, seq int) []string {
	return []string{fmt.Sprintf("%s.packet_dst_channel='%s'", waTag, channelID),
		fmt.Sprintf("%s.packet_sequence='%d'", waTag, seq)}
}
