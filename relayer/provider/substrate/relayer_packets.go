package substrate

import (
	"fmt"

	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/cosmos/relayer/relayer/provider"
)

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

	return NewSubstrateRelayerMessage(msg), nil
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
