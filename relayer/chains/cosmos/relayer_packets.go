package cosmos

import (
	"fmt"

	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
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

func (rp relayMsgTimeout) Msg(src provider.ChainProvider, srcPortId, srcChanId, dstPortId, dstChanId string) (provider.RelayerMessage, error) {
	if rp.dstRecvRes == nil {
		return nil, fmt.Errorf("timeout packet [%s]seq{%d} has no associated proofs", src.ChainId(), rp.seq)
	}
	addr, err := src.Address()
	if err != nil {
		return nil, err
	}

	msg := &chantypes.MsgTimeout{
		Packet: chantypes.Packet{
			Sequence:           rp.seq,
			SourcePort:         srcPortId,
			SourceChannel:      srcChanId,
			DestinationPort:    dstPortId,
			DestinationChannel: dstChanId,
			Data:               rp.packetData,
			TimeoutHeight:      rp.timeout,
			TimeoutTimestamp:   rp.timeoutStamp,
		},
		ProofUnreceived:  rp.dstRecvRes.Proof,
		ProofHeight:      rp.dstRecvRes.ProofHeight,
		NextSequenceRecv: rp.seq,
		Signer:           addr,
	}

	return NewCosmosMessage(msg), nil
}

type relayMsgRecvPacket struct {
	packetData   []byte
	seq          uint64
	timeout      clienttypes.Height
	timeoutStamp uint64
	dstComRes    *chantypes.QueryPacketCommitmentResponse
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

func (rp relayMsgRecvPacket) Msg(src provider.ChainProvider, srcPortId, srcChanId, dstPortId, dstChanId string) (provider.RelayerMessage, error) {
	if rp.dstComRes == nil {
		return nil, fmt.Errorf("receive packet [%s]seq{%d} has no associated proofs", src.ChainId(), rp.seq)
	}
	addr, err := src.Address()
	if err != nil {
		return nil, err
	}

	msg := &chantypes.MsgRecvPacket{
		Packet: chantypes.Packet{
			Sequence:           rp.seq,
			SourcePort:         dstPortId,
			SourceChannel:      dstChanId,
			DestinationPort:    srcPortId,
			DestinationChannel: srcChanId,
			Data:               rp.packetData,
			TimeoutHeight:      rp.timeout,
			TimeoutTimestamp:   rp.timeoutStamp,
		},
		ProofCommitment: rp.dstComRes.Proof,
		ProofHeight:     rp.dstComRes.ProofHeight,
		Signer:          addr,
	}

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

	addr, err := src.Address()
	if err != nil {
		return nil, err
	}

	msg := &chantypes.MsgAcknowledgement{
		Packet: chantypes.Packet{
			Sequence:           rp.seq,
			SourcePort:         srcPortId,
			SourceChannel:      srcChanId,
			DestinationPort:    dstPortId,
			DestinationChannel: dstChanId,
			Data:               rp.packetData,
			TimeoutHeight:      rp.timeout,
			TimeoutTimestamp:   rp.timeoutStamp,
		},
		Acknowledgement: rp.ack,
		ProofAcked:      rp.dstComRes.Proof,
		ProofHeight:     rp.dstComRes.ProofHeight,
		Signer:          addr,
	}

	return NewCosmosMessage(msg), nil
}
