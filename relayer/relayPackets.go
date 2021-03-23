package relayer

import (
	"fmt"

	retry "github.com/avast/retry-go"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/types"
	chantypes "github.com/cosmos/cosmos-sdk/x/ibc/core/04-channel/types"
)

type relayPacket interface {
	Msg(src, dst *Chain) (sdk.Msg, error)
	FetchCommitResponse(src, dst *Chain) error
	Data() []byte
	Seq() uint64
	Timeout() uint64
}

type relayMsgTimeout struct {
	packetData   []byte
	seq          uint64
	timeout      uint64
	timeoutStamp uint64
	dstRecvRes   *chantypes.QueryPacketReceiptResponse

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

func (rp *relayMsgTimeout) FetchCommitResponse(src, dst *Chain) (err error) {
	var dstRecvRes *chantypes.QueryPacketReceiptResponse
	// retry getting commit response until it succeeds
	if err = retry.Do(func() error {
		// NOTE: Timeouts currently only work with ORDERED channels for nwo
		// NOTE: the proof height uses - 1 due to tendermint's delayed execution model
		queryHeight := dst.MustGetLatestLightHeight() - 1
		dstRecvRes, err = dst.QueryPacketReceipt(int64(queryHeight), rp.seq)
		switch {
		case err != nil:
			return err
		case dstRecvRes.Proof == nil:
			return fmt.Errorf("timeout packet receipt proof seq(%d) is nil", rp.seq)
		default:
			return nil
		}
	}, rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
		// OnRetry we want to update the light clients and then debug log
		if _, _, err := UpdateLightClients(src, dst); err != nil {
			return
		}
		if dst.debug {
			dst.Log(fmt.Sprintf("- [%s]@{%d} - try(%d/%d) query packet receipt: %s", dst.ChainID,
				dst.MustGetLatestLightHeight()-1, n+1, rtyAttNum, err))
		}
	})); err != nil {
		dst.Error(err)
		return
	}
	rp.dstRecvRes = dstRecvRes
	return nil
}

func (rp *relayMsgTimeout) Msg(src, dst *Chain) (sdk.Msg, error) {
	if rp.dstRecvRes == nil {
		return nil, fmt.Errorf("timeout packet [%s]seq{%d} has no associated proofs", src.ChainID, rp.seq)
	}
	version := clienttypes.ParseChainID(dst.ChainID)
	msg := chantypes.NewMsgTimeout(
		chantypes.NewPacket(
			rp.packetData,
			rp.seq,
			src.PathEnd.PortID,
			src.PathEnd.ChannelID,
			dst.PathEnd.PortID,
			dst.PathEnd.ChannelID,
			clienttypes.NewHeight(version, rp.timeout),
			rp.timeoutStamp,
		),
		rp.seq,
		rp.dstRecvRes.Proof,
		rp.dstRecvRes.ProofHeight,
		src.MustGetAddress(),
	)
	return msg, nil
}

type relayMsgRecvPacket struct {
	packetData   []byte
	seq          uint64
	timeout      uint64
	timeoutStamp uint64
	dstComRes    *chantypes.QueryPacketCommitmentResponse

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

func (rp *relayMsgRecvPacket) FetchCommitResponse(src, dst *Chain) (err error) {
	var dstCommitRes *chantypes.QueryPacketCommitmentResponse
	// retry getting commit response until it succeeds
	if err = retry.Do(func() error {
		// NOTE: the proof height uses - 1 due to tendermint's delayed execution model
		dstCommitRes, err = dst.QueryPacketCommitment(int64(dst.MustGetLatestLightHeight()-1), rp.seq)
		switch {
		case err != nil:
			return err
		case dstCommitRes.Proof == nil:
			return fmt.Errorf("recv packet commitment proof seq(%d) is nil", rp.seq)
		case dstCommitRes.Commitment == nil:
			return fmt.Errorf("recv packet commitment query seq(%d) is nil", rp.seq)
		default:
			return nil
		}
	}, rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
		// OnRetry we want to update the light clients and then debug log
		if _, _, err := UpdateLightClients(src, dst); err != nil {
			return
		}
		if dst.debug {
			dst.Log(fmt.Sprintf("- [%s]@{%d} - try(%d/%d) query packet commitment: %s", dst.ChainID,
				dst.MustGetLatestLightHeight()-1, n+1, rtyAttNum, err))
		}
	})); err != nil {
		dst.Error(err)
		return
	}
	rp.dstComRes = dstCommitRes
	return nil
}

func (rp *relayMsgRecvPacket) Msg(src, dst *Chain) (sdk.Msg, error) {
	if rp.dstComRes == nil {
		return nil, fmt.Errorf("receive packet [%s]seq{%d} has no associated proofs", src.ChainID, rp.seq)
	}
	version := clienttypes.ParseChainID(src.ChainID)
	packet := chantypes.NewPacket(
		rp.packetData,
		rp.seq,
		dst.PathEnd.PortID,
		dst.PathEnd.ChannelID,
		src.PathEnd.PortID,
		src.PathEnd.ChannelID,
		clienttypes.NewHeight(version, rp.timeout),
		rp.timeoutStamp,
	)
	msg := chantypes.NewMsgRecvPacket(
		packet,
		rp.dstComRes.Proof,
		rp.dstComRes.ProofHeight,
		src.MustGetAddress(),
	)
	return msg, nil
}

type relayMsgPacketAck struct {
	packetData   []byte
	ack          []byte
	seq          uint64
	timeout      uint64
	timeoutStamp uint64
	dstComRes    *chantypes.QueryPacketAcknowledgementResponse

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

func (rp *relayMsgPacketAck) Msg(src, dst *Chain) (sdk.Msg, error) {
	if rp.dstComRes == nil {
		return nil, fmt.Errorf("ack packet [%s]seq{%d} has no associated proofs", src.ChainID, rp.seq)
	}
	version := clienttypes.ParseChainID(dst.ChainID)
	msg := chantypes.NewMsgAcknowledgement(
		chantypes.NewPacket(
			rp.packetData,
			rp.seq,
			src.PathEnd.PortID,
			src.PathEnd.ChannelID,
			dst.PathEnd.PortID,
			dst.PathEnd.ChannelID,
			clienttypes.NewHeight(version, rp.timeout),
			rp.timeoutStamp,
		),
		rp.ack,
		rp.dstComRes.Proof,
		rp.dstComRes.ProofHeight,
		src.MustGetAddress(),
	)
	return msg, nil
}

func (rp *relayMsgPacketAck) FetchCommitResponse(src, dst *Chain) (err error) {
	var dstCommitRes *chantypes.QueryPacketAcknowledgementResponse
	// retry getting commit response until it succeeds
	if err = retry.Do(func() error {
		// NOTE: the proof height uses - 1 due to tendermint's delayed execution model
		dstCommitRes, err = dst.QueryPacketAcknowledgement(int64(dst.MustGetLatestLightHeight())-1, rp.seq)
		switch {
		case err != nil:
			return err
		case dstCommitRes.Proof == nil:
			return fmt.Errorf("ack packet acknowledgement proof seq(%d) is nil", rp.seq)
		case dstCommitRes.Acknowledgement == nil:
			return fmt.Errorf("ack packet acknowledgement query seq(%d) is nil", rp.seq)
		default:
			return nil
		}
	}, rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
		// OnRetry we want to update the light clients and then debug log
		if _, _, err := UpdateLightClients(src, dst); err != nil {
			return
		}
		if dst.debug {
			dst.Log(fmt.Sprintf("- [%s]@{%d} - try(%d/%d) query packet acknowledgement: %s",
				dst.ChainID, dst.MustGetLatestLightHeight()-1, n+1, rtyAttNum, err))
		}
	})); err != nil {
		dst.Error(err)
		return
	}
	rp.dstComRes = dstCommitRes
	return nil
}
