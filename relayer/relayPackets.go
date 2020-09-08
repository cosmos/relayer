package relayer

import (
	"fmt"

	retry "github.com/avast/retry-go"
	sdk "github.com/cosmos/cosmos-sdk/types"
	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
)

type relayPacket interface {
	Msg(src, dst *Chain) sdk.Msg
	FetchCommitResponse(src, dst *Chain, sh *SyncHeaders) error
	Data() []byte
	Seq() uint64
	Timeout() uint64
}

type relayMsgTimeout struct {
	packetData   []byte
	seq          uint64
	timeout      uint64
	timeoutStamp uint64
	dstRecvRes   *chanTypes.QueryPacketCommitmentResponse

	pass bool
}

func (rp *relayMsgTimeout) Data() []byte {
	return rp.packetData
}

func (rp *relayMsgTimeout) Seq() uint64 {
	return rp.seq
}

func (rp *relayMsgTimeout) Timeout() uint64 {
	return rp.timeout
}

func (rp *relayMsgTimeout) FetchCommitResponse(src, dst *Chain, sh *SyncHeaders) (err error) {
	var dstRecvRes *chanTypes.QueryPacketCommitmentResponse

	// retry getting commit response until it succeeds
	if err = retry.Do(func() error {
		// NOTE: Timeouts currently only work with ORDERED channels for nwo
		dstRecvRes, err = dst.QueryPacketCommitment(sh.GetHeader(dst.ChainID).Header.Height, rp.seq)
		if err != nil {
			return err
		} else if dstRecvRes.Proof == nil {
			return fmt.Errorf("- [%s]@{%d} - Packet Commitment Proof is nil seq(%d)",
				dst.ChainID, int64(sh.GetHeight(dst.ChainID)), rp.seq)
		}
		return nil
	}); err != nil {
		dst.Error(err)
		return
	}

	rp.dstRecvRes = dstRecvRes
	return
}

func (rp *relayMsgTimeout) Msg(src, dst *Chain) sdk.Msg {
	if rp.dstRecvRes == nil {
		return nil
	}
	return src.PathEnd.MsgTimeout(
		dst.PathEnd,
		rp.packetData,
		rp.seq,
		rp.timeout,
		rp.timeoutStamp,
		rp.dstRecvRes.Proof,
		MustGetHeight(rp.dstRecvRes.ProofHeight),
		src.MustGetAddress(),
	)
}

type relayMsgRecvPacket struct {
	packetData   []byte
	seq          uint64
	timeout      uint64
	timeoutStamp uint64
	dstComRes    *chanTypes.QueryPacketCommitmentResponse

	pass bool
}

func (rp *relayMsgRecvPacket) timeoutPacket() *relayMsgTimeout {
	return &relayMsgTimeout{
		packetData:   rp.packetData,
		seq:          rp.seq,
		timeout:      rp.timeout,
		timeoutStamp: rp.timeoutStamp,
		dstRecvRes:   nil,
		pass:         false,
	}
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
	var dstCommitRes *chanTypes.QueryPacketCommitmentResponse

	// retry getting commit response until it succeeds
	if err = retry.Do(func() error {
		dstCommitRes, err = dst.QueryPacketCommitment(sh.GetHeader(dst.ChainID).Header.Height, rp.seq)
		if err != nil {
			return err
		} else if dstCommitRes.Proof == nil {
			return fmt.Errorf("- [%s]@{%d} - Packet Commitment Proof is nil seq(%d)",
				dst.ChainID, int64(sh.GetHeight(dst.ChainID)), rp.seq)
		}
		return nil
	}); err != nil {
		dst.Error(err)
		return
	}

	rp.dstComRes = dstCommitRes
	return
}

func (rp *relayMsgRecvPacket) Msg(src, dst *Chain) sdk.Msg {
	if rp.dstComRes == nil {
		return nil
	}
	return src.PacketMsg(
		dst, rp.packetData, rp.timeout, rp.timeoutStamp, int64(rp.seq), rp.dstComRes,
	)
}

type relayMsgPacketAck struct {
	packetData   []byte
	ack          []byte
	seq          uint64
	timeout      uint64
	timeoutStamp uint64
	dstComRes    *chanTypes.QueryPacketCommitmentResponse
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
		MustGetHeight(rp.dstComRes.ProofHeight),
		src.MustGetAddress(),
	)
}

func (rp *relayMsgPacketAck) FetchCommitResponse(src, dst *Chain, sh *SyncHeaders) (err error) {
	var dstCommitRes *chanTypes.QueryPacketCommitmentResponse
	if err = retry.Do(func() error {
		dstCommitRes, err = dst.QueryPacketCommitment(sh.GetHeader(dst.ChainID).Header.Height, rp.seq)
		if err != nil {
			return err
		} else if dstCommitRes.Proof == nil {
			return fmt.Errorf("- [%s]@{%d} - Packet Ack Proof is nil seq(%d)",
				dst.ChainID, int64(sh.GetHeight(dst.ChainID)), rp.seq)
		}
		return nil
	}); err != nil {
		dst.Error(err)
		return
	}
	rp.dstComRes = dstCommitRes
	return nil
}
