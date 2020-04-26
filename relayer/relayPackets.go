package relayer

import (
	"fmt"

	retry "github.com/avast/retry-go"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type relayPacket interface {
	Msg(src, dst *Chain) sdk.Msg
	FetchCommitResponse(src, dst *Chain, sh *SyncHeaders) error
	Data() []byte
	Seq() uint64
	Timeout() uint64
}

type relayMsgRecvPacket struct {
	packetData   []byte
	seq          uint64
	timeout      uint64
	timeoutStamp uint64
	dstComRes    *CommitmentResponse

	pass bool
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
	return src.PacketMsg(
		dst, rp.packetData, rp.timeout, rp.timeoutStamp, int64(rp.seq), *rp.dstComRes,
	)
}

type relayMsgPacketAck struct {
	packetData   []byte
	ack          []byte
	seq          uint64
	timeout      uint64
	timeoutStamp uint64
	dstComRes    *CommitmentResponse

	pass bool
}

func (rp *relayMsgPacketAck) Data() []byte {
	return rp.packetData
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
		rp.timeoutStamp,
		rp.ack,
		rp.packetData,
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
