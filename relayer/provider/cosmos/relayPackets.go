package cosmos

import (
	"fmt"
	clienttypes "github.com/cosmos/ibc-go/v2/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v2/modules/core/04-channel/types"
	"github.com/cosmos/relayer/relayer/provider"
)

var (
	_ provider.RelayPacket = relayMsgTimeout{}
	_ provider.RelayPacket = relayMsgRecvPacket{}
	_ provider.RelayPacket = relayMsgPacketAck{}
)

type relayMsgTimeout struct {
	packetData   []byte
	seq          uint64
	timeout      clienttypes.Height
	timeoutStamp uint64
	dstRecvRes   *chantypes.QueryPacketReceiptResponse

	pass bool
}

func (rp relayMsgTimeout) Data() []byte {
	return rp.packetData
}

func (rp relayMsgTimeout) Seq() uint64 {
	return rp.seq
}

func (rp relayMsgTimeout) Timeout() clienttypes.Height {
	return rp.timeout
}

func (rp relayMsgTimeout) TimeoutStamp() uint64 {
	return rp.timeoutStamp
}

func (rp relayMsgTimeout) FetchCommitResponse(dst provider.ChainProvider, queryHeight uint64, dstChanId, dstPortId string) error {
	dstRecvRes, err := dst.QueryPacketReceipt(int64(queryHeight)-1, dstChanId, dstPortId, rp.seq)
	switch {
	case err != nil:
		return err
	case dstRecvRes.Proof == nil:
		return fmt.Errorf("timeout packet receipt proof seq(%d) is nil", rp.seq)
	default:
		rp.dstRecvRes = dstRecvRes
		return nil
	}
}

func (rp relayMsgTimeout) Msg(src provider.ChainProvider, srcPortId, srcChanId, dstPortId, dstChanId string) (provider.RelayerMessage, error) {
	if rp.dstRecvRes == nil {
		return nil, fmt.Errorf("timeout packet [%s]seq{%d} has no associated proofs", src.ChainId(), rp.seq)
	}
	msg := chantypes.NewMsgTimeout(
		chantypes.NewPacket(
			rp.packetData,
			rp.seq,
			srcPortId,
			srcChanId,
			dstPortId,
			dstChanId,
			rp.timeout,
			rp.timeoutStamp,
		),
		rp.seq,
		rp.dstRecvRes.Proof,
		rp.dstRecvRes.ProofHeight,
		src.Address(),
	)
	return NewCosmosMessage(msg), nil
}

type relayMsgRecvPacket struct {
	packetData   []byte
	seq          uint64
	timeout      clienttypes.Height
	timeoutStamp uint64
	dstComRes    *chantypes.QueryPacketCommitmentResponse

	pass bool
}

func (rp relayMsgRecvPacket) timeoutPacket() *relayMsgTimeout {
	return &relayMsgTimeout{
		packetData:   rp.packetData,
		seq:          rp.seq,
		timeout:      rp.timeout,
		timeoutStamp: rp.timeoutStamp,
		dstRecvRes:   nil,
		pass:         false,
	}
}

func (rp relayMsgRecvPacket) Data() []byte {
	return rp.packetData
}

func (rp relayMsgRecvPacket) Seq() uint64 {
	return rp.seq
}

func (rp relayMsgRecvPacket) Timeout() clienttypes.Height {
	return rp.timeout
}

func (rp relayMsgRecvPacket) TimeoutStamp() uint64 {
	return rp.timeoutStamp
}

func (rp relayMsgRecvPacket) FetchCommitResponse(dst provider.ChainProvider, queryHeight uint64, dstChanId, dstPortId string) error {
	dstCommitRes, err := dst.QueryPacketCommitment(int64(queryHeight)-1, dstChanId, dstPortId, rp.seq)
	switch {
	case err != nil:
		return err
	case dstCommitRes.Proof == nil:
		return fmt.Errorf("recv packet commitment proof seq(%d) is nil", rp.seq)
	case dstCommitRes.Commitment == nil:
		return fmt.Errorf("recv packet commitment query seq(%d) is nil", rp.seq)
	default:
		rp.dstComRes = dstCommitRes
		return nil
	}
}

func (rp relayMsgRecvPacket) Msg(src provider.ChainProvider, srcPortId, srcChanId, dstPortId, dstChanId string) (provider.RelayerMessage, error) {
	if rp.dstComRes == nil {
		return nil, fmt.Errorf("receive packet [%s]seq{%d} has no associated proofs", src.ChainId(), rp.seq)
	}
	packet := chantypes.NewPacket(
		rp.packetData,
		rp.seq,
		dstPortId,
		dstChanId,
		srcPortId,
		srcChanId,
		rp.timeout,
		rp.timeoutStamp,
	)
	msg := chantypes.NewMsgRecvPacket(
		packet,
		rp.dstComRes.Proof,
		rp.dstComRes.ProofHeight,
		src.Address(),
	)
	return NewCosmosMessage(msg), nil
}

type relayMsgPacketAck struct {
	packetData   []byte
	ack          []byte
	seq          uint64
	timeout      clienttypes.Height
	timeoutStamp uint64
	dstComRes    *chantypes.QueryPacketAcknowledgementResponse

	pass bool
}

func (rp relayMsgPacketAck) Data() []byte {
	return rp.packetData
}
func (rp relayMsgPacketAck) Seq() uint64 {
	return rp.seq
}
func (rp relayMsgPacketAck) Timeout() clienttypes.Height {
	return rp.timeout
}

func (rp relayMsgPacketAck) TimeoutStamp() uint64 {
	return rp.timeoutStamp
}

func (rp relayMsgPacketAck) Msg(src provider.ChainProvider, srcPortId, srcChanId, dstPortId, dstChanId string) (provider.RelayerMessage, error) {
	if rp.dstComRes == nil {
		return nil, fmt.Errorf("ack packet [%s]seq{%d} has no associated proofs", src.ChainId(), rp.seq)
	}
	msg := chantypes.NewMsgAcknowledgement(
		chantypes.NewPacket(
			rp.packetData,
			rp.seq,
			srcPortId,
			srcChanId,
			dstPortId,
			dstChanId,
			rp.timeout,
			rp.timeoutStamp,
		),
		rp.ack,
		rp.dstComRes.Proof,
		rp.dstComRes.ProofHeight,
		src.Address(),
	)
	return NewCosmosMessage(msg), nil
}

func (rp relayMsgPacketAck) FetchCommitResponse(dst provider.ChainProvider, queryHeight uint64, dstChanId, dstPortId string) error {
	dstCommitRes, err := dst.QueryPacketAcknowledgement(int64(queryHeight)-1, dstChanId, dstPortId, rp.seq)
	switch {
	case err != nil:
		return err
	case dstCommitRes.Proof == nil:
		return fmt.Errorf("ack packet acknowledgement proof seq(%d) is nil", rp.seq)
	case dstCommitRes.Acknowledgement == nil:
		return fmt.Errorf("ack packet acknowledgement query seq(%d) is nil", rp.seq)
	default:
		rp.dstComRes = dstCommitRes
		return nil
	}
}
