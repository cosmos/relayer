package relayer

import (
	"fmt"

	retry "github.com/avast/retry-go"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clientTypes "github.com/cosmos/cosmos-sdk/x/ibc/02-client/types"
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
		dstRecvRes, err = dst.QueryPacketCommitment(sh.GetHeader(dst.ChainID).Header.Height-1, rp.seq)
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
	return chanTypes.NewMsgTimeout(
		chanTypes.NewPacket(
			rp.packetData,
			rp.seq,
			dst.PathEnd.PortID,
			dst.PathEnd.ChannelID,
			src.PathEnd.PortID,
			src.PathEnd.ChannelID,
			clientTypes.NewHeight(0, rp.timeout),
			rp.timeoutStamp,
		),
		rp.seq,
		rp.dstRecvRes.Proof,
		rp.dstRecvRes.ProofHeight,
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
	fmt.Printf("Querying commit proof on chain(%s) for seq(%d) at height(%d)\n", dst.ChainID, rp.seq, sh.GetHeader(dst.ChainID).Header.Height-1)
	var dstCommitRes *chanTypes.QueryPacketCommitmentResponse
	// retry getting commit response until it succeeds
	if err = retry.Do(func() error {
		dstCommitRes, err = dst.QueryPacketCommitment(sh.GetHeader(dst.ChainID).Header.Height-1, rp.seq)
		if err != nil {
			return err
		} else if dstCommitRes.Proof == nil || dstCommitRes.Commitment == nil {
			if err := sh.Update(src); err != nil {
				return err
			}
			if err := sh.Update(dst); err != nil {
				return err
			}
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
	fmt.Printf("Creating NewMsgRecvPacket to send to chain(%s)\n", src.ChainID)
	packet := chanTypes.NewPacket(
		rp.packetData,
		rp.seq,
		dst.PathEnd.PortID,
		dst.PathEnd.ChannelID,
		src.PathEnd.PortID,
		src.PathEnd.ChannelID,
		clientTypes.NewHeight(0, rp.timeout),
		rp.timeoutStamp,
	)
	fmt.Println("packet", packet)
	fmt.Println("packetData", string(rp.packetData))
	return chanTypes.NewMsgRecvPacket(
		packet,
		rp.dstComRes.Proof,
		rp.dstComRes.ProofHeight,
		src.MustGetAddress(),
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
	return chanTypes.NewMsgAcknowledgement(
		chanTypes.NewPacket(
			rp.packetData,
			rp.seq,
			src.PathEnd.PortID,
			src.PathEnd.ChannelID,
			dst.PathEnd.PortID,
			dst.PathEnd.ChannelID,
			clientTypes.NewHeight(0, rp.timeout),
			rp.timeoutStamp,
		),
		rp.ack,
		rp.dstComRes.Proof,
		rp.dstComRes.ProofHeight,
		src.MustGetAddress(),
	)
}

func (rp *relayMsgPacketAck) FetchCommitResponse(src, dst *Chain, sh *SyncHeaders) (err error) {
	var dstCommitRes *chanTypes.QueryPacketCommitmentResponse
	if err = retry.Do(func() error {
		dstCommitRes, err = dst.QueryPacketCommitment(sh.GetHeader(dst.ChainID).Header.Height-1, rp.seq)
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
