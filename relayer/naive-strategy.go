package relayer

import (
	"fmt"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// NewNaiveStrategy returns the proper config for the NaiveStrategy
func NewNaiveStrategy() *StrategyCfg {
	return &StrategyCfg{
		Type: (&NaiveStrategy{}).GetType(),
	}
}

// NaiveStrategy is an implementation of Strategy
type NaiveStrategy struct {
	Ordered bool
}

// GetType implements Strategy
func (nrs *NaiveStrategy) GetType() string {
	return "naive"
}

// UnrelayedSequencesOrdered returns the unrelayed sequence numbers between two chains
func (nrs *NaiveStrategy) UnrelayedSequencesOrdered(src, dst *Chain, sh *SyncHeaders) (*RelaySequences, error) {
	return UnrelayedSequences(src, dst, sh)
}

// UnrelayedSequencesUnordered returns the unrelayed sequence numbers between two chains
func (nrs *NaiveStrategy) UnrelayedSequencesUnordered(src, dst *Chain, sh *SyncHeaders) (*RelaySequences, error) {
	return UnrelayedSequences(src, dst, sh)
}

// HandleEvents defines how the relayer will handle block and transaction events as they are emmited
func (nrs *NaiveStrategy) HandleEvents(src, dst *Chain, sh *SyncHeaders, events map[string][]string) {
	rlyPackets, err := relayPacketsFromEventListener(src.PathEnd, dst.PathEnd, events)
	if len(rlyPackets) > 0 && err == nil {
		sendTxFromEventPackets(src, dst, rlyPackets, sh)
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
			if dst.PortID == srcPort[i] && dst.ChannelID == srcChan[i] && src.PortID == dstPort[i] && src.ChannelID == dstChan[i] {
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
					timeout, err := strconv.ParseUint(sval[i], 10, 64)
					if err != nil {
						return nil, err
					}
					rp.timeout = timeout
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
			if src.PortID == srcPort[i] && src.ChannelID == srcChan[i] && dst.PortID == dstPort[i] && dst.ChannelID == dstChan[i] {
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
					timeout, err := strconv.ParseUint(sval[i], 10, 64)
					if err != nil {
						return nil, err
					}
					rp.timeout = timeout
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
	return
}

func sendTxFromEventPackets(src, dst *Chain, rlyPackets []relayPacket, sh *SyncHeaders) {
	// fetch the proofs for the relayPackets
	for _, rp := range rlyPackets {
		if err := rp.FetchCommitResponse(src, dst, sh); err != nil {
			// we don't expect many errors here because of the retry
			// in FetchCommitResponse
			src.Error(err)
		}
	}

	// instantiate the RelayMsgs with the appropriate update client
	txs := &RelayMsgs{
		Src: []sdk.Msg{
			src.PathEnd.UpdateClient(sh.GetHeader(dst.ChainID), src.MustGetAddress()),
		},
		Dst: []sdk.Msg{},
	}

	// add the packet msgs to RelayPackets
	for _, rp := range rlyPackets {
		txs.Src = append(txs.Src, rp.Msg(src, dst))
	}

	// send the transaction, maybe retry here if not successful
	if txs.Send(src, dst); !txs.success {
		src.Error(fmt.Errorf("failed to send packets, maybe we should add a retry here"))
	}
}

// RelayPacketsUnorderedChan creates transactions to relay un-relayed messages
func (nrs *NaiveStrategy) RelayPacketsUnorderedChan(src, dst *Chain, sp *RelaySequences, sh *SyncHeaders) error {
	// TODO: Implement unordered channels
	return nrs.RelayPacketsOrderedChan(src, dst, sp, sh)
}

// RelayPacketsOrderedChan creates transactions to clear both queues
// CONTRACT: the SyncHeaders passed in here must be up to date or being kept updated
func (nrs *NaiveStrategy) RelayPacketsOrderedChan(src, dst *Chain, sp *RelaySequences, sh *SyncHeaders) error {

	// create the appropriate update client messages
	msgs := &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}}
	if len(sp.Src) > 0 {
		msgs.Dst = append(msgs.Dst, dst.PathEnd.UpdateClient(sh.GetHeader(src.ChainID), dst.MustGetAddress()))
	}
	if len(sp.Dst) > 0 {
		msgs.Src = append(msgs.Src, src.PathEnd.UpdateClient(sh.GetHeader(dst.ChainID), src.MustGetAddress()))
	}

	// add messages for src -> dst
	for _, seq := range sp.Src {
		msg, err := packetMsgFromTxQuery(src, dst, sh, seq)
		if err != nil {
			return err
		}
		msgs.Dst = append(msgs.Dst, msg)
	}

	// add messages for dst -> src
	for _, seq := range sp.Dst {
		msg, err := packetMsgFromTxQuery(dst, src, sh, seq)
		if err != nil {
			return err
		}
		msgs.Src = append(msgs.Src, msg)
	}

	if !msgs.Ready() {
		src.Log(fmt.Sprintf("- No packets to relay between [%s]port{%s} and [%s]port{%s}", src.ChainID, src.PathEnd.PortID, dst.ChainID, dst.PathEnd.PortID))
		return nil
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
func packetMsgFromTxQuery(src, dst *Chain, sh *SyncHeaders, seq uint64) (sdk.Msg, error) {
	eveSend, err := ParseEvents(fmt.Sprintf(defaultPacketSendQuery, src.PathEnd.ChannelID, seq))
	if err != nil {
		return nil, err
	}

	tx, err := src.QueryTxs(sh.GetHeight(src.ChainID), 1, 1000, eveSend)
	switch {
	case err != nil:
		return nil, err
	case tx.Count == 0:
		return nil, fmt.Errorf("no transactions returned with query")
	case tx.Count > 1:
		return nil, fmt.Errorf("more than one transaction returned with query")
	}

	rlyPackets, err := relayPacketFromQueryResponse(src.PathEnd, dst.PathEnd, tx.Txs[0])
	switch {
	case err != nil:
		return nil, err
	case len(rlyPackets) == 0:
		return nil, fmt.Errorf("no relay msgs created from query response")
	case len(rlyPackets) > 1:
		return nil, fmt.Errorf("more than one relay msg found in tx query")
	}

	// sanity check the sequence number against the one we are querying for
	// TODO: move this into relayPacketFromQueryResponse?
	if seq != rlyPackets[0].Seq() {
		return nil, fmt.Errorf("Different sequence number from query (%d vs %d)", seq, rlyPackets[0].Seq())
	}

	// fetch the proof from the sending chain
	if err = rlyPackets[0].FetchCommitResponse(dst, src, sh); err != nil {
		return nil, err
	}

	// return the sending msg
	return rlyPackets[0].Msg(dst, src), nil
}

// relayPacketFromQueryResponse looks through the events in a sdk.Response
// and returns relayPackets with the appropriate data
func relayPacketFromQueryResponse(src, dst *PathEnd, res sdk.TxResponse) (rlyPackets []relayPacket, err error) {
	for _, l := range res.Logs {
		for _, e := range l.Events {
			if e.Type == "send_packet" {
				// NOTE: Src and Dst are switched here
				rp := &relayMsgRecvPacket{pass: false}
				for _, p := range e.Attributes {
					if p.Key == "packet_src_channel" {
						if p.Value != src.ChannelID {
							rp.pass = true
							continue
						}
					}
					if p.Key == "packet_dst_channel" {
						if p.Value != dst.ChannelID {
							rp.pass = true
							continue
						}
					}
					if p.Key == "packet_src_port" {
						if p.Value != src.PortID {
							rp.pass = true
							continue
						}
					}
					if p.Key == "packet_dst_port" {
						if p.Value != dst.PortID {
							rp.pass = true
							continue
						}
					}
					if p.Key == "packet_data" {
						rp.packetData = []byte(p.Value)
					}
					if p.Key == "packet_timeout_height" {
						timeout, _ := strconv.ParseUint(p.Value, 10, 64)
						rp.timeout = timeout
					}
					if p.Key == "packet_timeout_timestamp" {
						timeout, _ := strconv.ParseUint(p.Value, 10, 64)
						rp.timeoutStamp = timeout
					}
					if p.Key == "packet_sequence" {
						seq, _ := strconv.ParseUint(p.Value, 10, 64)
						rp.seq = seq
					}
				}

				// if we have decided not to relay this packet, don't add it
				if !rp.pass {
					rlyPackets = append(rlyPackets, rp)
				}
			}
		}
	}

	if len(rlyPackets) > 0 {
		return
	}

	return nil, fmt.Errorf("no packet data found")
}
