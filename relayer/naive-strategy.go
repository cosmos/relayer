package relayer

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	retry "github.com/avast/retry-go"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/types"
	chantypes "github.com/cosmos/cosmos-sdk/x/ibc/core/04-channel/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"golang.org/x/sync/errgroup"
)

var _ Strategy = &NaiveStrategy{}

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

// UnrelayedSequencesOrdered returns the unrelayed sequence numbers between two chains
func (nrs *NaiveStrategy) UnrelayedSequencesOrdered(src, dst *Chain, sh *SyncHeaders) (*RelaySequences, error) {
	var (
		eg           = new(errgroup.Group)
		srcPacketSeq = []uint64{}
		dstPacketSeq = []uint64{}
		err          error
		rs           = &RelaySequences{Src: []uint64{}, Dst: []uint64{}}
	)

	eg.Go(func() error {
		var res *chantypes.QueryPacketCommitmentsResponse
		if err = retry.Do(func() error {
			// Query the packet commitment
			res, err = src.QueryPacketCommitments(0, 1000, sh.GetHeight(src.ChainID))
			switch {
			case err != nil:
				return err
			case res == nil:
				return fmt.Errorf("No error on QueryPacketCommitments for %s, however response is nil", src.ChainID)
			default:
				return nil
			}
		}, retry.Attempts(3), retry.Delay(time.Millisecond*200), retry.LastErrorOnly(true),
			retry.OnRetry(func(n uint, err error) {
				if src.debug {
					src.Log(fmt.Sprintf("- [%s] failed to query packet commitments %d/3, retrying: %s", src.ChainID, n, err))
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
			res, err = dst.QueryPacketCommitments(0, 1000, sh.GetHeight(dst.ChainID))
			switch {
			case err != nil:
				return err
			case res == nil:
				return fmt.Errorf("No error on QueryPacketCommitments for %s, however response is nil", dst.ChainID)
			default:
				return nil
			}
		}, retry.Attempts(3), retry.Delay(time.Millisecond*200), retry.LastErrorOnly(true),
			retry.OnRetry(func(n uint, err error) {
				if dst.debug {
					dst.Log(fmt.Sprintf("- [%s] failed to query packet commitments %d/3, retrying: %s", dst.ChainID, n, err))
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
		rs.Src, err = dst.QueryUnrecievedPackets(sh.GetHeight(dst.ChainID), srcPacketSeq)
		return err
	})

	eg.Go(func() error {
		// Query all packets sent by dst that have been received by src
		rs.Dst, err = src.QueryUnrecievedPackets(sh.GetHeight(src.ChainID), dstPacketSeq)
		return err
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return rs, nil
}

// UnrelayedSequencesUnordered returns the unrelayed sequence numbers between two chains
func (nrs *NaiveStrategy) UnrelayedSequencesUnordered(src, dst *Chain, sh *SyncHeaders) (*RelaySequences, error) {
	return nrs.UnrelayedSequencesOrdered(src, dst, sh)
}

// HandleEvents defines how the relayer will handle block and transaction events as they are emitted
func (nrs *NaiveStrategy) HandleEvents(src, dst *Chain, sh *SyncHeaders, events map[string][]string) {
	rlyPackets, err := relayPacketsFromEventListener(src.PathEnd, dst.PathEnd, events)
	if len(rlyPackets) > 0 && err == nil {
		nrs.sendTxFromEventPackets(src, dst, rlyPackets, sh)
	}
}

func relayPacketsFromEventListener(src, dst *PathEnd, events map[string][]string) (rlyPkts []relayPacket, err error) {
	// check for send packets
	if pdval, ok := events["send_packet.packet_data"]; ok {
		for i, pd := range pdval {
			// Ensure that we only relay over the channel and port specified
			// OPTIONAL FEATURE: add additional filtering options
			// Example Events - "transfer.amount(sdk.Coin)", "message.sender(sdk.AccAddress)"
			srcChan, srcPort := events["send_packet.packet_src_channel"], events["send_packet.packet_src_port"]
			dstChan, dstPort := events["send_packet.packet_dst_channel"], events["send_packet.packet_dst_port"]

			// NOTE: Src and Dst are switched here
			if dst.PortID == srcPort[i] && dst.ChannelID == srcChan[i] &&
				src.PortID == dstPort[i] && src.ChannelID == dstChan[i] {
				rp := &relayMsgRecvPacket{packetData: []byte(pd)}

				// next, get and parse the sequence
				if sval, ok := events["send_packet.packet_sequence"]; ok {
					seq, err := strconv.ParseUint(sval[i], 10, 64)
					if err != nil {
						return nil, err
					}
					rp.seq = seq
				}

				// finally, get and parse the timeout
				if sval, ok := events["send_packet.packet_timeout_height"]; ok {
					timeout, err := clienttypes.ParseHeight(sval[i])
					if err != nil {
						return nil, err
					}
					rp.timeout = MustGetHeight(timeout)
				}

				// finally, get and parse the timeout
				if sval, ok := events["send_packet.packet_timeout_timestamp"]; ok {
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
	if pdval, ok := events["recv_packet.packet_data"]; ok {
		for i, pd := range pdval {
			// Ensure that we only relay over the channel and port specified
			// OPTIONAL FEATURE: add additional filtering options
			srcChan, srcPort := events["recv_packet.packet_src_channel"], events["recv_packet.packet_src_port"]
			dstChan, dstPort := events["recv_packet.packet_dst_channel"], events["recv_packet.packet_dst_port"]

			// NOTE: Src and Dst are not switched here
			if src.PortID == srcPort[i] && src.ChannelID == srcChan[i] &&
				dst.PortID == dstPort[i] && dst.ChannelID == dstChan[i] {
				rp := &relayMsgPacketAck{packetData: []byte(pd)}

				// first get the ack
				if ack, ok := events["recv_packet.packet_ack"]; ok {
					rp.ack = []byte(ack[i])
				}
				// next, get and parse the sequence
				if sval, ok := events["recv_packet.packet_sequence"]; ok {
					seq, err := strconv.ParseUint(sval[i], 10, 64)
					if err != nil {
						return nil, err
					}
					rp.seq = seq
				}

				// finally, get and parse the timeout
				if sval, ok := events["recv_packet.packet_timeout_height"]; ok {
					timeout, err := clienttypes.ParseHeight(sval[i])
					if err != nil {
						return nil, err
					}
					rp.timeout = MustGetHeight(timeout)
				}

				// finally, get and parse the timeout
				if sval, ok := events["recv_packet.packet_timeout_timestamp"]; ok {
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

func (nrs *NaiveStrategy) sendTxFromEventPackets(src, dst *Chain, rlyPackets []relayPacket, sh *SyncHeaders) {

	// fetch the proofs for the relayPackets
	for _, rp := range rlyPackets {
		if err := rp.FetchCommitResponse(src, dst, sh); err != nil {
			// we don't expect many errors here because of the retry
			// in FetchCommitResponse
			src.Error(err)
		}
	}

	// send the transaction, retrying if not successful
	if err := retry.Do(func() error {
		if err := sh.Updates(src, dst); err != nil {
			if src.debug {
				src.Log(fmt.Sprintf("- failed to update headers for %s and %s, retrying: %s", src.ChainID, dst.ChainID, err))
			}
			return err
		}
		updateHeader, err := sh.GetUpdateHeader(dst, src)
		if err != nil {
			if src.debug {
				src.Log(fmt.Sprintf("- failed to enrich update headers for %s and %s, retrying: %s", src.ChainID, dst.ChainID, err))
			}
			return err
		}
		// instantiate the RelayMsgs with the appropriate update client
		txs := &RelayMsgs{
			Src: []sdk.Msg{
				src.PathEnd.UpdateClient(updateHeader, src.MustGetAddress()),
			},
			Dst:          []sdk.Msg{},
			MaxTxSize:    nrs.MaxTxSize,
			MaxMsgLength: nrs.MaxMsgLength,
		}

		// add the packet msgs to RelayPackets
		for _, rp := range rlyPackets {
			msg, err := rp.Msg(src, dst)
			if err != nil {
				if src.debug {
					src.Log(fmt.Sprintf("- [%s] failed to create relay packet message bound for %s of type %T, retrying: %s", src.ChainID, dst.ChainID, rp, err))
				}
				return err
			}
			txs.Src = append(txs.Src, msg)
		}

		if txs.Send(src, dst); !txs.success {
			return fmt.Errorf("failed to send packets, see above logs for details")
		}

		return nil
	}, retry.Attempts(3), retry.Delay(time.Millisecond*200), retry.LastErrorOnly(true)); err != nil {
		src.Error(err)
	}
}

// RelaySequences represents unrelayed packets on src and dst
type RelaySequences struct {
	Src []uint64 `json:"src"`
	Dst []uint64 `json:"dst"`
}

// RelayPacketsUnorderedChan creates transactions to relay un-relayed messages
func (nrs *NaiveStrategy) RelayPacketsUnorderedChan(src, dst *Chain, sp *RelaySequences, sh *SyncHeaders) error {
	// TODO: Implement unordered channels
	return nrs.RelayPacketsOrderedChan(src, dst, sp, sh)
}

// RelayPacketsOrderedChan creates transactions to clear both queues
// CONTRACT: the SyncHeaders passed in here must be up to date or being kept updated
func (nrs *NaiveStrategy) RelayPacketsOrderedChan(src, dst *Chain, sp *RelaySequences, sh *SyncHeaders) error {
	// set the maximum relay transaction constraints
	msgs := &RelayMsgs{
		Src:          []sdk.Msg{},
		Dst:          []sdk.Msg{},
		MaxTxSize:    nrs.MaxTxSize,
		MaxMsgLength: nrs.MaxMsgLength,
	}

	// add messages for src -> dst
	for _, seq := range sp.Src {
		chain, msg, err := packetMsgFromTxQuery(src, dst, sh, seq)
		if err != nil {
			return err
		}
		if chain == dst {
			msgs.Dst = append(msgs.Dst, msg)
		} else {
			msgs.Src = append(msgs.Src, msg)
		}
	}

	// add messages for dst -> src
	for _, seq := range sp.Dst {
		chain, msg, err := packetMsgFromTxQuery(dst, src, sh, seq)
		if err != nil {
			return err
		}
		if chain == src {
			msgs.Src = append(msgs.Src, msg)
		} else {
			msgs.Dst = append(msgs.Dst, msg)
		}
	}

	if !msgs.Ready() {
		src.Log(fmt.Sprintf("- No packets to relay between [%s]port{%s} and [%s]port{%s}",
			src.ChainID, src.PathEnd.PortID, dst.ChainID, dst.PathEnd.PortID))
		return nil
	}

	// Prepend non-empty msg lists with UpdateClient
	if len(msgs.Dst) != 0 {
		// Sending an update from src to dst
		updateHeader, err := sh.GetUpdateHeader(src, dst)
		if err != nil {
			return err
		}
		msgs.Dst = append([]sdk.Msg{dst.PathEnd.UpdateClient(updateHeader, dst.MustGetAddress())}, msgs.Dst...)
	}

	if len(msgs.Src) != 0 {
		// Sending an update from dst to src
		updateHeader, err := sh.GetUpdateHeader(dst, src)
		if err != nil {
			return err
		}
		msgs.Src = append([]sdk.Msg{src.PathEnd.UpdateClient(updateHeader, src.MustGetAddress())}, msgs.Src...)
	}

	// TODO: increase the amount of gas as the number of messages increases
	// notify the user of that
	if msgs.Send(src, dst); msgs.success {
		if len(msgs.Dst) > 1 {
			dst.logPacketsRelayed(src, len(msgs.Dst)-1)
		}
		if len(msgs.Src) > 1 {
			src.logPacketsRelayed(dst, len(msgs.Src)-1)
		}
	}

	return nil
}

// packetMsgFromTxQuery returns a sdk.Msg to relay a packet with a given seq on src
func packetMsgFromTxQuery(src, dst *Chain, sh *SyncHeaders, seq uint64) (*Chain, sdk.Msg, error) {
	eveSend, err := ParseEvents(fmt.Sprintf(defaultPacketSendQuery, src.PathEnd.ChannelID, seq))
	if err != nil {
		return nil, nil, err
	}

	txs, err := src.QueryTxs(sh.GetHeight(src.ChainID), 1, 1000, eveSend)
	switch {
	case err != nil:
		return nil, nil, err
	case len(txs) == 0:
		return nil, nil, fmt.Errorf("no transactions returned with query")
	case len(txs) > 1:
		return nil, nil, fmt.Errorf("more than one transaction returned with query")
	}

	rcvPackets, timeoutPackets, err := relayPacketFromQueryResponse(src.PathEnd, dst.PathEnd, txs[0], sh)
	switch {
	case err != nil:
		return nil, nil, err
	case len(rcvPackets) == 0 && len(timeoutPackets) == 0:
		return nil, nil, fmt.Errorf("no relay msgs created from query response")
	case len(rcvPackets)+len(timeoutPackets) > 1:
		return nil, nil, fmt.Errorf("more than one relay msg found in tx query")
	}

	// If relayPacketFromQueryResponse returned a packet to be received on destination chain
	// then create receive msg to be sent to destination chain
	if len(rcvPackets) == 1 {
		// sanity check the sequence number against the one we are querying for
		// TODO: move this into relayPacketFromQueryResponse?
		if seq != rcvPackets[0].Seq() {
			return nil, nil,
				fmt.Errorf("different sequence number from query (%d vs %d)", seq, rcvPackets[0].Seq())
		}

		// fetch the proof from the sending chain
		if err = rcvPackets[0].FetchCommitResponse(dst, src, sh); err != nil {
			return nil, nil, err
		}

		// return the sending msg
		msg, err := rcvPackets[0].Msg(dst, src)
		if err != nil {
			return nil, nil, err
		}
		return dst, msg, nil
	}

	// NOTE: Since timeout packets are sent to the original sending chain, src and dst are flipped relative to the
	// recv packets
	// relayPacketFromQueryResponse returned a timeout msg, create a timeout msg to be sent to original sender chain
	if seq != timeoutPackets[0].Seq() {
		return nil, nil,
			fmt.Errorf("different sequence number from query (%d vs %d)", seq, timeoutPackets[0].Seq())
	}

	// fetch the timeout proof from the receiving chain
	if err = timeoutPackets[0].FetchCommitResponse(src, dst, sh); err != nil {
		return nil, nil, err
	}

	// return the sending msg
	msg, err := timeoutPackets[0].Msg(src, dst)
	if err != nil {
		return nil, nil, err
	}
	return src, msg, nil

}

// relayPacketFromQueryResponse looks through the events in a sdk.Response
// and returns relayPackets with the appropriate data
func relayPacketFromQueryResponse(src, dst *PathEnd, res *ctypes.ResultTx,
	sh *SyncHeaders) (rcvPackets []relayPacket, timeoutPackets []relayPacket, err error) {
	for _, e := range res.TxResult.Events {
		if e.Type == "send_packet" {
			// NOTE: Src and Dst are switched here
			rp := &relayMsgRecvPacket{pass: false}
			for _, p := range e.Attributes {
				if string(p.Key) == "packet_src_channel" {
					if string(p.Value) != src.ChannelID {
						rp.pass = true
						continue
					}
				}
				if string(p.Key) == "packet_dst_channel" {
					if string(p.Value) != dst.ChannelID {
						rp.pass = true
						continue
					}
				}
				if string(p.Key) == "packet_src_port" {
					if string(p.Value) != src.PortID {
						rp.pass = true
						continue
					}
				}
				if string(p.Key) == "packet_dst_port" {
					if string(p.Value) != dst.PortID {
						rp.pass = true
						continue
					}
				}
				if string(p.Key) == "packet_data" {
					rp.packetData = p.Value
				}
				if string(p.Key) == "packet_timeout_height" {
					timeout, _ := strconv.ParseUint(strings.Split(string(p.Value), "-")[1], 10, 64)
					rp.timeout = timeout
				}
				if string(p.Key) == "packet_timeout_timestamp" {
					timeout, _ := strconv.ParseUint(string(p.Value), 10, 64)
					rp.timeoutStamp = timeout
				}
				if string(p.Key) == "packet_sequence" {
					seq, _ := strconv.ParseUint(string(p.Value), 10, 64)
					rp.seq = seq
				}
			}

			// if we have decided not to relay this packet, don't add it
			block := sh.GetHeader(dst.ChainID)

			switch {
			case rp.timeout != 0 && block.GetHeight().GetVersionHeight() >= rp.timeout:
				timeoutPackets = append(timeoutPackets, rp.timeoutPacket())
			case rp.timeoutStamp != 0 && block.GetTime().UnixNano() >= int64(rp.timeoutStamp):
				timeoutPackets = append(timeoutPackets, rp.timeoutPacket())
			case !rp.pass:
				rcvPackets = append(rcvPackets, rp)
			}
		}
	}

	if len(rcvPackets)+len(timeoutPackets) > 0 {
		return
	}

	return nil, nil, fmt.Errorf("no packet data found")
}
