package relayer

import (
	"fmt"
	"strconv"
	"strings"

	retry "github.com/avast/retry-go"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/types"
	chantypes "github.com/cosmos/cosmos-sdk/x/ibc/core/04-channel/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"golang.org/x/sync/errgroup"
)

var (
	// Ensure that NaiveStrategy satisfies the Strategy interface
	_ Strategy = &NaiveStrategy{}

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

// NewNaiveStrategy returns the proper config for the NaiveStrategy
func NewNaiveStrategy() *StrategyCfg {
	return &StrategyCfg{
		Type: (&NaiveStrategy{}).GetType(),
	}
}

// NaiveStrategy is an implementation of Strategy.
type NaiveStrategy struct {
	Ordered      bool
	MaxTxSize    uint64 // maximum permitted size of the msgs in a bundled relay transaction
	MaxMsgLength uint64 // maximum amount of messages in a bundled relay transaction
}

// GetType implements Strategy
func (nrs *NaiveStrategy) GetType() string {
	return "naive"
}

// UnrelayedSequences returns the unrelayed sequence numbers between two chains
func (nrs *NaiveStrategy) UnrelayedSequences(src, dst *Chain) (*RelaySequences, error) {
	var (
		eg           = new(errgroup.Group)
		srcPacketSeq = []uint64{}
		dstPacketSeq = []uint64{}
		rs           = &RelaySequences{Src: []uint64{}, Dst: []uint64{}}
	)

	_, _, err := UpdateLightClients(src, dst)
	if err != nil {
		return nil, err
	}

	eg.Go(func() error {
		var res *chantypes.QueryPacketCommitmentsResponse
		if err = retry.Do(func() error {
			// Query the packet commitment
			res, err = src.QueryPacketCommitments(0, 1000, src.MustGetLatestLightHeight())
			switch {
			case err != nil:
				return err
			case res == nil:
				return fmt.Errorf("no error on QueryPacketCommitments for %s, however response is nil", src.ChainID)
			default:
				return nil
			}
		}, rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
			if src.debug {
				src.Log(fmt.Sprintf("- [%s]@{%d} - try(%d/%d) query packet commitments: %s", src.ChainID,
					src.MustGetLatestLightHeight(), n+1, rtyAttNum, err))
			}
		})); err != nil {
			return err
		}
		for _, pc := range res.Commitments {
			srcPacketSeq = append(srcPacketSeq, pc.Sequence)
		}
		return nil
	})

	eg.Go(func() error {
		var res *chantypes.QueryPacketCommitmentsResponse
		if err = retry.Do(func() error {
			res, err = dst.QueryPacketCommitments(0, 1000, dst.MustGetLatestLightHeight())
			switch {
			case err != nil:
				return err
			case res == nil:
				return fmt.Errorf("no error on QueryPacketCommitments for %s, however response is nil", dst.ChainID)
			default:
				return nil
			}
		}, rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
			if dst.debug {
				dst.Log(fmt.Sprintf("- [%s]@{%d} - try(%d/%d) query packet commitments: %s",
					dst.ChainID, dst.MustGetLatestLightHeight(), n+1, rtyAttNum, err))
			}
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
		rs.Src, err = dst.QueryUnreceivedPackets(dst.MustGetLatestLightHeight(), srcPacketSeq)
		return err
	})

	eg.Go(func() error {
		// Query all packets sent by dst that have been received by src
		rs.Dst, err = src.QueryUnreceivedPackets(src.MustGetLatestLightHeight(), dstPacketSeq)
		return err
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return rs, nil
}

// UnrelayedAcknowledgements returns the unrelayed sequence numbers between two chains
func (nrs *NaiveStrategy) UnrelayedAcknowledgements(src, dst *Chain) (*RelaySequences, error) {
	var (
		eg           = new(errgroup.Group)
		srcPacketSeq = []uint64{}
		dstPacketSeq = []uint64{}
		rs           = &RelaySequences{Src: []uint64{}, Dst: []uint64{}}
	)

	_, _, err := UpdateLightClients(src, dst)
	if err != nil {
		return nil, err
	}

	eg.Go(func() error {
		var res *chantypes.QueryPacketAcknowledgementsResponse
		if err = retry.Do(func() error {
			// Query the packet commitment
			res, err = src.QueryPacketAcknowledgements(0, 1000, src.MustGetLatestLightHeight())
			switch {
			case err != nil:
				return err
			case res == nil:
				return src.errQueryUnrelayedPacketAcks()
			default:
				return nil
			}
		}, rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
			src.logRetryQueryPacketAcknowledgements(src.MustGetLatestLightHeight(), n, err)
			if _, _, err := UpdateLightClients(src, dst); err != nil {
				return
			}
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
			res, err = dst.QueryPacketAcknowledgements(0, 1000, dst.MustGetLatestLightHeight())
			switch {
			case err != nil:
				return err
			case res == nil:
				return dst.errQueryUnrelayedPacketAcks()
			default:
				return nil
			}
		}, rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
			dst.logRetryQueryPacketAcknowledgements(dst.MustGetLatestLightHeight(), n, err)
			if _, _, err := UpdateLightClients(src, dst); err != nil {
				return
			}
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
		rs.Src, err = dst.QueryUnreceivedAcknowledgements(dst.MustGetLatestLightHeight(), srcPacketSeq)
		return err
	})

	eg.Go(func() error {
		// Query all packets sent by dst that have been received by src
		rs.Dst, err = src.QueryUnreceivedAcknowledgements(src.MustGetLatestLightHeight(), dstPacketSeq)
		return err
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return rs, nil
}

// HandleEvents defines how the relayer will handle block and transaction events as they are emitted
func (nrs *NaiveStrategy) HandleEvents(src, dst *Chain, events map[string][]string) {
	// check for misbehaviour and submit if found
	err := checkAndSubmitMisbehaviour(src, events)
	if err != nil {
		src.Error(err)
	}

	rlyPackets, err := relayPacketsFromEventListener(src.PathEnd, dst.PathEnd, events)
	if len(rlyPackets) > 0 && err == nil {
		nrs.sendTxFromEventPackets(src, dst, rlyPackets)
	}
}

func relayPacketsFromEventListener(src, dst *PathEnd, events map[string][]string) (rlyPkts []relayPacket, err error) {
	// check for send packets
	if pdval, ok := events[fmt.Sprintf("%s.%s", spTag, dataTag)]; ok {
		for i, pd := range pdval {
			// Ensure that we only relay over the channel and port specified
			// OPTIONAL FEATURE: add additional filtering options
			// Example Events - "transfer.amount(sdk.Coin)", "message.sender(sdk.AccAddress)"
			srcChan, srcPort := events[fmt.Sprintf("%s.%s", spTag, srcChanTag)], events[fmt.Sprintf("%s.%s", spTag, srcPortTag)]
			dstChan, dstPort := events[fmt.Sprintf("%s.%s", spTag, dstChanTag)], events[fmt.Sprintf("%s.%s", spTag, dstPortTag)]

			// NOTE: Src and Dst are switched here
			if dst.PortID == srcPort[i] && dst.ChannelID == srcChan[i] &&
				src.PortID == dstPort[i] && src.ChannelID == dstChan[i] {
				rp := &relayMsgRecvPacket{packetData: []byte(pd)}

				// next, get and parse the sequence
				if sval, ok := events[fmt.Sprintf("%s.%s", spTag, seqTag)]; ok {
					seq, err := strconv.ParseUint(sval[i], 10, 64)
					if err != nil {
						return nil, err
					}
					rp.seq = seq
				}

				// finally, get and parse the timeout
				if sval, ok := events[fmt.Sprintf("%s.%s", spTag, toHeightTag)]; ok {
					timeout, err := clienttypes.ParseHeight(sval[i])
					if err != nil {
						return nil, err
					}
					rp.timeout = MustGetHeight(timeout)
				}

				// finally, get and parse the timeout
				if sval, ok := events[fmt.Sprintf("%s.%s", spTag, toTSTag)]; ok {
					timeout, err := strconv.ParseUint(sval[i], 10, 64)
					if err != nil {
						return nil, err
					}
					rp.timeoutStamp = timeout
				}

				// queue the packet for return
				rlyPkts = append(rlyPkts, rp)
			}
		}
	}

	// then, check for packet acks
	if pdval, ok := events[fmt.Sprintf("%s.%s", waTag, dataTag)]; ok {
		for i, pd := range pdval {
			// Ensure that we only relay over the channel and port specified
			// OPTIONAL FEATURE: add additional filtering options
			srcChan, srcPort := events[fmt.Sprintf("%s.%s", waTag, srcChanTag)], events[fmt.Sprintf("%s.%s", waTag, srcPortTag)]
			dstChan, dstPort := events[fmt.Sprintf("%s.%s", waTag, dstChanTag)], events[fmt.Sprintf("%s.%s", waTag, dstPortTag)]

			// NOTE: Src and Dst are not switched here
			if src.PortID == srcPort[i] && src.ChannelID == srcChan[i] &&
				dst.PortID == dstPort[i] && dst.ChannelID == dstChan[i] {
				rp := &relayMsgPacketAck{packetData: []byte(pd)}

				// first get the ack
				if ack, ok := events[fmt.Sprintf("%s.%s", waTag, ackTag)]; ok {
					rp.ack = []byte(ack[i])
				}
				// next, get and parse the sequence
				if sval, ok := events[fmt.Sprintf("%s.%s", waTag, seqTag)]; ok {
					seq, err := strconv.ParseUint(sval[i], 10, 64)
					if err != nil {
						return nil, err
					}
					rp.seq = seq
				}

				// finally, get and parse the timeout
				if sval, ok := events[fmt.Sprintf("%s.%s", waTag, toHeightTag)]; ok {
					timeout, err := clienttypes.ParseHeight(sval[i])
					if err != nil {
						return nil, err
					}
					rp.timeout = MustGetHeight(timeout)
				}

				// finally, get and parse the timeout
				if sval, ok := events[fmt.Sprintf("%s.%s", waTag, toTSTag)]; ok {
					timeout, err := strconv.ParseUint(sval[i], 10, 64)
					if err != nil {
						return nil, err
					}
					rp.timeoutStamp = timeout
				}

				// queue the packet for return
				rlyPkts = append(rlyPkts, rp)
			}
		}
	}
	return rlyPkts, nil
}

func (nrs *NaiveStrategy) sendTxFromEventPackets(src, dst *Chain, rlyPackets []relayPacket) {

	// fetch the proofs for the relayPackets
	for _, rp := range rlyPackets {
		if err := rp.FetchCommitResponse(src, dst); err != nil {
			// we don't expect many errors here because of the retry
			// in FetchCommitResponse
			src.Error(err)
		}
	}

	// send the transaction, retrying if not successful
	// TODO: have separate retries for different pieces here
	if err := retry.Do(func() error {
		updateMsg, err := src.UpdateClient(dst)
		if err != nil {
			if src.debug {
				src.Log(fmt.Sprintf("- failed to construct update message for client on chain %s, retrying: %s",
					src.PathEnd.ChainID, err))
			}

			return err
		}

		txs := &RelayMsgs{
			Src:          []sdk.Msg{updateMsg},
			Dst:          []sdk.Msg{},
			MaxTxSize:    nrs.MaxTxSize,
			MaxMsgLength: nrs.MaxMsgLength,
		}

		// add the packet msgs to RelayPackets
		for _, rp := range rlyPackets {
			msg, err := rp.Msg(src, dst)
			if err != nil {
				if src.debug {
					src.Log(fmt.Sprintf("- [%s] failed to create relay packet message bound for %s of type %T, retrying: %s",
						src.ChainID, dst.ChainID, rp, err))
				}
				return err
			}
			txs.Src = append(txs.Src, msg)
		}

		if txs.Send(src, dst); !txs.Success() {
			return fmt.Errorf("failed to send packets, see above logs for details")
		}

		return nil
	}, rtyAtt, rtyDel, rtyErr); err != nil {
		src.Error(err)
	}
}

// RelaySequences represents unrelayed packets on src and dst
type RelaySequences struct {
	Src []uint64 `json:"src"`
	Dst []uint64 `json:"dst"`
}

// RelayAcknowledgements creates transactions to relay acknowledgements from src to dst and from dst to src
func (nrs *NaiveStrategy) RelayAcknowledgements(src, dst *Chain, sp *RelaySequences) error {
	// set the maximum relay transaction constraints
	msgs := &RelayMsgs{
		Src:          []sdk.Msg{},
		Dst:          []sdk.Msg{},
		MaxTxSize:    nrs.MaxTxSize,
		MaxMsgLength: nrs.MaxMsgLength,
	}
	srcUpdateMsg, err := src.UpdateClient(dst)
	if err != nil {
		return err
	}

	dstUpdateMsg, err := dst.UpdateClient(src)
	if err != nil {
		return err
	}

	// add messages for sequences on src
	for _, seq := range sp.Src {
		// SRC wrote ack, so we query packet and send to DST
		relayAckMsgs, err := acknowledgementFromSequence(src, dst, seq)
		if err != nil {
			return err
		}

		msgs.Dst = append(msgs.Dst, relayAckMsgs...)
	}

	// add messages for sequences on dst
	for _, seq := range sp.Dst {
		// DST wrote ack, so we query packet and send to SRC
		relayAckMsgs, err := acknowledgementFromSequence(dst, src, seq)
		if err != nil {
			return err
		}

		msgs.Src = append(msgs.Src, relayAckMsgs...)
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
func (nrs *NaiveStrategy) RelayPackets(src, dst *Chain, sp *RelaySequences) error {
	if _, _, err := UpdateLightClients(src, dst); err != nil {
		return err
	}

	// set the maximum relay transaction constraints
	msgs := &RelayMsgs{
		Src:          []sdk.Msg{},
		Dst:          []sdk.Msg{},
		MaxTxSize:    nrs.MaxTxSize,
		MaxMsgLength: nrs.MaxMsgLength,
	}

	// add messages for sequences on src
	for _, seq := range sp.Src {
		// Query src for the sequence number to get type of packet
		recvMsgs, timeoutMsgs, err := relayPacketFromSequence(src, dst, seq)
		if err != nil {
			return err
		}

		// depending on the type of message to be relayed, we need to
		// send to different chains
		if recvMsgs != nil {
			msgs.Dst = append(msgs.Dst, recvMsgs...)
		}

		if timeoutMsgs != nil {
			msgs.Src = append(msgs.Src, timeoutMsgs...)
		}
	}

	// add messages for sequences on dst
	for _, seq := range sp.Dst {
		// Query dst for the sequence number to get type of packet
		recvMsgs, timeoutMsgs, err := relayPacketFromSequence(dst, src, seq)
		if err != nil {
			return err
		}

		// depending on the type of message to be relayed, we need to
		// send to different chains
		if recvMsgs != nil {
			msgs.Src = append(msgs.Src, recvMsgs...)
		}

		if timeoutMsgs != nil {
			msgs.Dst = append(msgs.Dst, timeoutMsgs...)
		}
	}

	if !msgs.Ready() {
		src.Log(fmt.Sprintf("- No packets to relay between [%s]port{%s} and [%s]port{%s}",
			src.ChainID, src.PathEnd.PortID, dst.ChainID, dst.PathEnd.PortID))
		return nil
	}

	// Prepend non-empty msg lists with UpdateClient
	if len(msgs.Dst) != 0 {
		updateMsg, err := dst.UpdateClient(src)
		if err != nil {
			return err
		}

		msgs.Dst = append([]sdk.Msg{updateMsg}, msgs.Dst...)
	}

	if len(msgs.Src) != 0 {
		updateMsg, err := src.UpdateClient(dst)
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
	}

	return nil
}

// relayPacketFromSequence relays a packet with a given seq on src
// and returns recvPacket msgs, timeoutPacketmsgs and error
func relayPacketFromSequence(src, dst *Chain, seq uint64) ([]sdk.Msg, []sdk.Msg, error) {
	txs, err := src.QueryTxs(src.MustGetLatestLightHeight(), 1, 1000, rcvPacketQuery(src.PathEnd.ChannelID, int(seq)))
	switch {
	case err != nil:
		return nil, nil, err
	case len(txs) == 0:
		return nil, nil, fmt.Errorf("no transactions returned with query")
	case len(txs) > 1:
		return nil, nil, fmt.Errorf("more than one transaction returned with query")
	}

	rcvPackets, timeoutPackets, err := relayPacketsFromResultTx(src, dst, txs[0])
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

		msgs, err := dst.MsgRelayRecvPacket(src, pkt.(*relayMsgRecvPacket))
		if err != nil {
			return nil, nil, err
		}
		return msgs, nil, nil
	}

	if len(timeoutPackets) == 1 {
		pkt := timeoutPackets[0]
		if seq != pkt.Seq() {
			return nil, nil, fmt.Errorf("wrong sequence: expected(%d) got(%d)", seq, pkt.Seq())
		}

		msgs, err := src.MsgRelayTimeout(dst, pkt.(*relayMsgTimeout))
		if err != nil {
			return nil, nil, err
		}
		return nil, msgs, nil
	}

	return nil, nil, fmt.Errorf("should have errored before here")
}

// source is the sending chain, destination is the receiving chain
func acknowledgementFromSequence(src, dst *Chain, seq uint64) ([]sdk.Msg, error) {
	txs, err := src.QueryTxs(src.MustGetLatestLightHeight(), 1, 1000, ackPacketQuery(src.PathEnd.ChannelID, int(seq)))
	switch {
	case err != nil:
		return nil, err
	case len(txs) == 0:
		return nil, fmt.Errorf("no transactions returned with query")
	case len(txs) > 1:
		return nil, fmt.Errorf("more than one transaction returned with query")
	}

	acks, err := acknowledgementsFromResultTx(dst.PathEnd, src.PathEnd, txs[0])
	switch {
	case err != nil:
		return nil, err
	case len(acks) == 0:
		return nil, fmt.Errorf("no ack msgs created from query response")
	case len(acks) > 1:
		return nil, fmt.Errorf("more than one ack msg found in tx query")
	}

	pkt := acks[0]
	if seq != pkt.Seq() {
		return nil, fmt.Errorf("wrong sequence: expected(%d) got(%d)", seq, pkt.Seq())
	}

	msgs, err := src.MsgRelayAcknowledgement(dst, pkt)
	if err != nil {
		return nil, err
	}
	return msgs, nil
}

// relayPacketsFromResultTx looks through the events in a *ctypes.ResultTx
// and returns relayPackets with the appropriate data
func relayPacketsFromResultTx(src, dst *Chain, res *ctypes.ResultTx) ([]relayPacket, []relayPacket, error) {
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
					timeout, _ := strconv.ParseUint(strings.Split(string(p.Value), "-")[1], 10, 64)
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
			block, err := src.GetIBCUpdateHeader(dst)
			if err != nil {
				return nil, nil, err
			}

			switch {
			// If the packet has a timeout height, and it has been reached, return a timeout packet
			case rp.timeout != 0 && block.GetHeight().GetRevisionHeight() >= rp.timeout:
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
//relayPackets with the appropriate data
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
					timeout, _ := strconv.ParseUint(strings.Split(string(p.Value), "-")[1], 10, 64)
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
