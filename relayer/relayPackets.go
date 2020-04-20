package relayer

import (
	"fmt"
	"strconv"

	retry "github.com/avast/retry-go"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func relayPacketsFromEventListener(events map[string][]string) (rlyPkts []relayPacket, err error) {
	// check for send packets
	if pdval, ok := events["send_packet.packet_data"]; ok {
		for i, pd := range pdval {
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
			if sval, ok := events["send_packet.packet_timeout"]; ok {
				timeout, err := strconv.ParseUint(sval[i], 10, 64)
				if err != nil {
					return nil, err
				}
				rp.timeout = timeout
			}
			rlyPkts = append(rlyPkts, rp)
		}
	}

	// // then, check for packet acks
	// if pdval, ok := events["recv_packet.packet_data"]; ok {
	// 	for i, pd := range pdval {
	// 		rp := &relayMsgPacketAck{ack: []byte(pd)}
	// 		// next, get and parse the sequence
	// 		if sval, ok := events["recv_packet.packet_sequence"]; ok {
	// 			seq, err := strconv.ParseUint(sval[i], 10, 64)
	// 			if err != nil {
	// 				return nil, err
	// 			}
	// 			rp.seq = seq
	// 		}

	// 		// finally, get and parse the timeout
	// 		if sval, ok := events["recv_packet.packet_timeout"]; ok {
	// 			timeout, err := strconv.ParseUint(sval[i], 10, 64)
	// 			if err != nil {
	// 				return nil, err
	// 			}
	// 			rp.timeout = timeout
	// 		}

	// 		rlyPkts = append(rlyPkts, rp)
	// 	}
	// }
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

type relayPacket interface {
	Msg(src, dst *Chain) sdk.Msg
	FetchCommitResponse(src, dst *Chain, sh *SyncHeaders) error
	Data() []byte
	Seq() uint64
	Timeout() uint64
}

type relayMsgRecvPacket struct {
	packetData []byte
	seq        uint64
	timeout    uint64
	dstComRes  *CommitmentResponse
}

func (rp *relayMsgRecvPacket) Data() []byte {
	return rp.packetData
}

func (rp *relayMsgRecvPacket) Seq() uint64 {
	return rp.seq
}

func (rp *relayMsgRecvPacket) Timeout() uint64 {
	return rp.timeout
}

func (rp *relayMsgRecvPacket) FetchCommitResponse(src, dst *Chain, sh *SyncHeaders) (err error) {
	var dstCommitRes CommitmentResponse

	// retry getting commit response until it succeeds
	if err = retry.Do(func() error {
		dstCommitRes, err = dst.QueryPacketCommitment(int64(sh.GetHeight(dst.ChainID)-1), int64(rp.seq))
		if err != nil {
			return err
		} else if dstCommitRes.Proof.Proof == nil {
			return fmt.Errorf("- [%s]@{%d} - Packet Commitment Proof is nil seq(%d)", dst.ChainID, int64(sh.GetHeight(dst.ChainID)-1), rp.seq)
		}
		return nil
	}); err != nil {
		dst.Error(err)
		return
	}

	rp.dstComRes = &dstCommitRes
	return
}

func (rp *relayMsgRecvPacket) Msg(src, dst *Chain) sdk.Msg {
	if rp.dstComRes == nil {
		return nil
	}
	return src.PacketMsg(dst, rp.packetData, rp.timeout, int64(rp.seq), *rp.dstComRes)
}

// nolint
type relayMsgPacketAck struct {
	ack       []byte
	seq       uint64
	timeout   uint64
	dstComRes *CommitmentResponse
}

func (rp *relayMsgPacketAck) Data() []byte {
	return rp.ack
}
func (rp *relayMsgPacketAck) Seq() uint64 {
	return rp.seq
}
func (rp *relayMsgPacketAck) Timeout() uint64 {
	return rp.timeout
}

func (rp *relayMsgPacketAck) Msg(src, dst *Chain) sdk.Msg {
	return src.PathEnd.MsgAck(
		dst.PathEnd,
		rp.seq,
		rp.timeout,
		rp.ack,
		rp.dstComRes.Proof,
		rp.dstComRes.ProofHeight,
		src.MustGetAddress(),
	)
}

func (rp *relayMsgPacketAck) FetchCommitResponse(src, dst *Chain, sh *SyncHeaders) (err error) {
	var dstCommitRes CommitmentResponse
	if err = retry.Do(func() error {
		dstCommitRes, err = dst.QueryPacketAck(int64(sh.GetHeight(dst.ChainID)-1), int64(rp.seq))
		if err != nil {
			return err
		} else if dstCommitRes.Proof.Proof == nil {
			return fmt.Errorf("- [%s]@{%d} - Packet Ack Proof is nil seq(%d)", dst.ChainID, int64(sh.GetHeight(dst.ChainID)-1), rp.seq)
		}
		return nil
	}); err != nil {
		dst.Error(err)
		return
	}
	rp.dstComRes = &dstCommitRes
	return nil
}
