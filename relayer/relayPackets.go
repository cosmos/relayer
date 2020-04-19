package relayer

import (
	"fmt"
	"strconv"

	retry "github.com/avast/retry-go"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TODO: rewrite this function to take a relayPacket
func (src *Chain) sendTxFromEventPackets(dst *Chain, rlyPackets []relayPacket, sh *SyncHeaders) {

	for _, rp := range rlyPackets {
		rp.FetchCommitResponse(src, dst, sh)
	}

	txs := &RelayMsgs{
		Src: []sdk.Msg{
			src.PathEnd.UpdateClient(sh.GetHeader(dst.ChainID), src.MustGetAddress()),
		},
		Dst: []sdk.Msg{},
	}

	for _, rp := range rlyPackets {
		txs.Src = append(txs.Src, rp.Msg(src, dst))
	}

	// TODO: Maybe retry here?
	txs.Send(src, dst)
}

func relayPacketsFromEvent(events map[string][]string) (rlyPkts []relayPacket, err error) {
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

	// then, check for recv packets
	if pdval, ok := events["recv_packet.packet_data"]; ok {
		for i, pd := range pdval {
			rp := &relayMsgPacketAck{ack: []byte(pd)}
			// next, get and parse the sequence
			if sval, ok := events["recv_packet.packet_sequence"]; ok {
				seq, err := strconv.ParseUint(sval[i], 10, 64)
				if err != nil {
					return nil, err
				}
				rp.seq = seq
			}

			// finally, get and parse the timeout
			if sval, ok := events["recv_packet.packet_timeout"]; ok {
				timeout, err := strconv.ParseUint(sval[i], 10, 64)
				if err != nil {
					return nil, err
				}
				rp.timeout = timeout
			}
			rlyPkts = append(rlyPkts, rp)
		}
	}
	return
}

type relayPacket interface {
	Msg(src, dst *Chain) sdk.Msg
	FetchCommitResponse(src, dst *Chain, sh *SyncHeaders)
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

func (rp *relayMsgRecvPacket) FetchCommitResponse(src, dst *Chain, sh *SyncHeaders) {
	var (
		err          error
		dstCommitRes CommitmentResponse
	)

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
}

func (rp *relayMsgRecvPacket) Msg(src, dst *Chain) sdk.Msg {
	if rp.dstComRes == nil {
		return nil
	}
	return src.PathEnd.MsgRecvPacket(
		dst.PathEnd,
		rp.seq,
		rp.timeout,
		rp.packetData,
		rp.dstComRes.Proof,
		rp.dstComRes.ProofHeight,
		src.MustGetAddress(),
	)
}

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

func (rp *relayMsgPacketAck) FetchCommitResponse(src, dst *Chain, sh *SyncHeaders) {
	var (
		err          error
		dstCommitRes CommitmentResponse
	)
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
}
