package relayer

import (
	"fmt"
	"strconv"
	"time"

	retry "github.com/avast/retry-go"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ Strategy = &NaiveStrategy{}

// NewNaiveStrategy returns the proper config for the NaiveStrategy
func NewNaiveStrategy() *StrategyCfg {
	return &StrategyCfg{
		Type: (&NaiveStrategy{}).GetType(),
	}
}

// NaiveStrategy is an implementation of Strategy. MaxTxSize and MaxMsgLength
// are ignored if they are set to zero.
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
		for len(rlyPackets) > 0 {
			txSize := 0
			// instantiate the RelayMsgs with the appropriate update client
			tx := &RelayMsgs{
				Src: []sdk.Msg{
					src.PathEnd.UpdateClient(sh.GetHeader(dst.ChainID), src.MustGetAddress()),
				},
				Dst: []sdk.Msg{},
			}

			// add the packet msgs to RelayPackets
			for i, rp := range rlyPackets {
				msg := rp.Msg(src, dst)
				txSize += len(msg.GetSignBytes())

				if nrs.IsMaxTx(i, txSize) {
					rlyPackets = rlyPackets[i:]
					break
				}

				tx.Src = append(tx.Src, msg)

				if i == len(rlyPackets)-1 {
					// clear the queue
					rlyPackets = []relayPacket{}
				}
			}

			if tx.Send(src, dst); !tx.success {
				return fmt.Errorf("failed to send packet %v", tx)
			}
		}
		return nil
	}); err != nil {
		src.Error(err)
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
	relayMsgs, err := nrs.buildRelayMsgs(src, dst, sp, sh)
	if err != nil {
		return err
	}

	if len(relayMsgs) == 0 {
		src.Log(fmt.Sprintf("- No packets to relay between [%s]port{%s} and [%s]port{%s}", src.ChainID, src.PathEnd.PortID, dst.ChainID, dst.PathEnd.PortID))
		return nil
	}

	for _, relayMsg := range relayMsgs {
		// TODO: increase the amount of gas as the number of messages increases
		// notify the user of that
		// XXX: should this just be a single log entry or one log per relay transaction?
		if relayMsg.Send(src, dst); relayMsg.success {
			if len(relayMsg.Dst) > 1 {
				dst.logPacketsRelayed(src, len(relayMsg.Dst)-1)
			}
			if len(relayMsg.Src) > 1 {
				src.logPacketsRelayed(dst, len(relayMsg.Src)-1)
			}
		}
	}

	return nil
}

func (nrs *NaiveStrategy) buildRelayMsgs(src, dst *Chain, sp *RelaySequences, sh *SyncHeaders) ([]RelayMsgs, error) {
	var txSize int

	// create the appropriate update client messages
	msgs := RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}}

	// add messages for src -> dst
	for i, seq := range sp.Src {
		chain, msg, err := packetMsgFromTxQuery(src, dst, sh, seq)
		if err != nil {
			return nil, err
		}

		txSize += len(msg.GetSignBytes())

		// enforce a maximum message length and maximum size of combined messages
		if nrs.IsMaxTx(i, txSize) {
			// update sequences processed
			sp.Src = sp.Src[i:]
			break
		}

		if chain == dst {
			msgs.Dst = append(msgs.Dst, msg)
		} else {
			msgs.Src = append(msgs.Src, msg)
		}

		// clear sequence queue
		if i == len(sp.Src)-1 {
			sp.Src = []uint64{}
		}
	}

	// reset txSize
	txSize = 0

	// add messages for dst -> src
	for i, seq := range sp.Dst {
		chain, msg, err := packetMsgFromTxQuery(dst, src, sh, seq)
		if err != nil {
			return nil, err
		}

		txSize += len(msg.GetSignBytes())

		// enforce a maximum message length and maximum size of combined messages
		if nrs.IsMaxTx(i, txSize) {
			// update sequences processed
			sp.Dst = sp.Dst[i:]
			break
		}

		if chain == src {
			msgs.Src = append(msgs.Src, msg)
		} else {
			msgs.Dst = append(msgs.Dst, msg)
		}

		// clear sequence queue
		if i == len(sp.Dst)-1 {
			sp.Dst = []uint64{}
		}
	}

	// Prepend non-empty msg lists with UpdateClient
	// XXX: should this be done just once since relay sequences aren't dynamically added after the call to this function?
	if len(msgs.Dst) != 0 {
		msgs.Dst = append([]sdk.Msg{dst.PathEnd.UpdateClient(sh.GetHeader(src.ChainID), dst.MustGetAddress())}, msgs.Dst...)
	}
	if len(msgs.Src) != 0 {
		msgs.Src = append([]sdk.Msg{src.PathEnd.UpdateClient(sh.GetHeader(dst.ChainID), src.MustGetAddress())}, msgs.Src...)
	}

	// base case
	if len(sp.Src) == 0 && len(sp.Dst) == 0 {
		return []RelayMsgs{msgs}, nil
	}

	// recurse until all sequences have been processed
	relayMsgs, err := nrs.buildRelayMsgs(src, dst, sp, sh)
	if err != nil {
		return nil, err
	}

	return append([]RelayMsgs{msgs}, relayMsgs...), nil
}

// IsMaxTx returns true if the passed in parameters surpass the maximum message length or maximum tx size.
// Defualt values of zero are ignored.
func (nrs *NaiveStrategy) IsMaxTx(msgLength, txSize int) bool {
	return (nrs.MaxMsgLength != 0 && uint64(msgLength) >= nrs.MaxMsgLength) || (nrs.MaxTxSize != 0 && uint64(txSize) >= nrs.MaxTxSize)
}

// packetMsgFromTxQuery returns a sdk.Msg to relay a packet with a given seq on src
func packetMsgFromTxQuery(src, dst *Chain, sh *SyncHeaders, seq uint64) (*Chain, sdk.Msg, error) {
	eveSend, err := ParseEvents(fmt.Sprintf(defaultPacketSendQuery, src.PathEnd.ChannelID, seq))
	if err != nil {
		return nil, nil, err
	}

	tx, err := src.QueryTxs(sh.GetHeight(src.ChainID), 1, 1000, eveSend)
	switch {
	case err != nil:
		return nil, nil, err
	case tx.Count == 0:
		return nil, nil, fmt.Errorf("no transactions returned with query")
	case tx.Count > 1:
		return nil, nil, fmt.Errorf("more than one transaction returned with query")
	}

	rcvPackets, timeoutPackets, err := relayPacketFromQueryResponse(src.PathEnd, dst.PathEnd, tx.Txs[0], sh)
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
			return nil, nil, fmt.Errorf("Different sequence number from query (%d vs %d)", seq, rcvPackets[0].Seq())
		}

		// fetch the proof from the sending chain
		if err = rcvPackets[0].FetchCommitResponse(dst, src, sh); err != nil {
			return nil, nil, err
		}

		// return the sending msg
		return dst, rcvPackets[0].Msg(dst, src), nil
	}

	// NOTE: Since timeout packets are sent to the original sending chain, src and dst are flipped relative to the
	// recv packets
	// relayPacketFromQueryResponse returned a timeout msg, create a timeout msg to be sent to original sender chain
	if seq != timeoutPackets[0].Seq() {
		return nil, nil, fmt.Errorf("Different sequence number from query (%d vs %d)", seq, timeoutPackets[0].Seq())
	}

	// fetch the timeout proof from the receiving chain
	if err = timeoutPackets[0].FetchCommitResponse(src, dst, sh); err != nil {
		return nil, nil, err
	}

	// return the sending msg
	return src, timeoutPackets[0].Msg(src, dst), nil

}

// relayPacketFromQueryResponse looks through the events in a sdk.Response
// and returns relayPackets with the appropriate data
func relayPacketFromQueryResponse(src, dst *PathEnd, res sdk.TxResponse, sh *SyncHeaders) (rcvPackets []relayPacket, timeoutPackets []relayPacket, err error) {
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
				switch {
				case sh.GetHeight(src.ChainID) >= rp.timeout:
					timeoutPackets = append(timeoutPackets, rp.timeoutPacket())
				case rp.timeoutStamp != 0 && time.Now().UnixNano() >= int64(rp.timeoutStamp):
					timeoutPackets = append(timeoutPackets, rp.timeoutPacket())
				case !rp.pass:
					rcvPackets = append(rcvPackets, rp)
				}
			}
		}
	}

	if len(rcvPackets)+len(timeoutPackets) > 0 {
		return
	}

	return nil, nil, fmt.Errorf("no packet data found")
}
