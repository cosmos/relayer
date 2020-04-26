package relayer

import (
	"strings"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clientTypes "github.com/cosmos/cosmos-sdk/x/ibc/02-client/types"
	connTypes "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/types"
	chanState "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/exported"
	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint/types"
	xferTypes "github.com/cosmos/cosmos-sdk/x/ibc/20-transfer/types"
	commitmenttypes "github.com/cosmos/cosmos-sdk/x/ibc/23-commitment/types"
)

// TODO: add Order chanTypes.Order as a property and wire it up in validation
// as well as in the transaction commands

// PathEnd represents the local connection identifers for a relay path
// The path is set on the chain before performing operations
type PathEnd struct {
	ChainID      string `yaml:"chain-id,omitempty" json:"chain-id,omitempty"`
	ClientID     string `yaml:"client-id,omitempty" json:"client-id,omitempty"`
	ConnectionID string `yaml:"connection-id,omitempty" json:"connection-id,omitempty"`
	ChannelID    string `yaml:"channel-id,omitempty" json:"channel-id,omitempty"`
	PortID       string `yaml:"port-id,omitempty" json:"port-id,omitempty"`
	Order        string `yaml:"order,omitempty" json:"order,omitempty"`
}

func (src *PathEnd) getOrder() chanState.Order {
	return chanState.OrderFromString(strings.ToUpper(src.Order))
}

// UpdateClient creates an sdk.Msg to update the client on c with data pulled from cp
func (src *PathEnd) UpdateClient(dstHeader *tmclient.Header, signer sdk.AccAddress) sdk.Msg {
	return tmclient.NewMsgUpdateClient(
		src.ClientID,
		*dstHeader,
		signer,
	)
}

// CreateClient creates an sdk.Msg to update the client on src with consensus state from dst
func (src *PathEnd) CreateClient(dstHeader *tmclient.Header, trustingPeriod time.Duration, signer sdk.AccAddress) sdk.Msg {
	if err := dstHeader.ValidateBasic(dstHeader.ChainID); err != nil {
		panic(err)
	}
	// TODO: figure out how to dynmaically set unbonding time
	return tmclient.NewMsgCreateClient(
		src.ClientID,
		*dstHeader,
		trustingPeriod,
		defaultUnbondingTime,
		defaultMaxClockDrift,
		signer,
	)
}

// ConnInit creates a MsgConnectionOpenInit
func (src *PathEnd) ConnInit(dst *PathEnd, signer sdk.AccAddress) sdk.Msg {
	return connTypes.NewMsgConnectionOpenInit(
		src.ConnectionID,
		src.ClientID,
		dst.ConnectionID,
		dst.ClientID,
		defaultChainPrefix,
		signer,
	)
}

// ConnTry creates a MsgConnectionOpenTry
// NOTE: ADD NOTE ABOUT PROOF HEIGHT CHANGE HERE
func (src *PathEnd) ConnTry(dst *PathEnd, dstConnState connTypes.ConnectionResponse, dstConsState clientTypes.ConsensusStateResponse, dstCsHeight int64, signer sdk.AccAddress) sdk.Msg {
	return connTypes.NewMsgConnectionOpenTry(
		src.ConnectionID,
		src.ClientID,
		dst.ConnectionID,
		dst.ClientID,
		defaultChainPrefix,
		defaultIBCVersions,
		dstConnState.Proof,
		dstConsState.Proof,
		dstConnState.ProofHeight+1,
		uint64(dstCsHeight),
		signer,
	)
}

// ConnAck creates a MsgConnectionOpenAck
// NOTE: ADD NOTE ABOUT PROOF HEIGHT CHANGE HERE
func (src *PathEnd) ConnAck(dstConnState connTypes.ConnectionResponse, dstConsState clientTypes.ConsensusStateResponse, dstCsHeight int64, signer sdk.AccAddress) sdk.Msg {
	return connTypes.NewMsgConnectionOpenAck(
		src.ConnectionID,
		dstConnState.Proof,
		dstConsState.Proof,
		dstConnState.ProofHeight+1,
		uint64(dstCsHeight),
		defaultIBCVersion,
		signer,
	)
}

// ConnConfirm creates a MsgConnectionOpenAck
// NOTE: ADD NOTE ABOUT PROOF HEIGHT CHANGE HERE
func (src *PathEnd) ConnConfirm(dstConnState connTypes.ConnectionResponse, signer sdk.AccAddress) sdk.Msg {
	return connTypes.NewMsgConnectionOpenConfirm(
		src.ConnectionID,
		dstConnState.Proof,
		dstConnState.ProofHeight+1,
		signer,
	)
}

// ChanInit creates a MsgChannelOpenInit
func (src *PathEnd) ChanInit(dst *PathEnd, signer sdk.AccAddress) sdk.Msg {
	return chanTypes.NewMsgChannelOpenInit(
		src.PortID,
		src.ChannelID,
		defaultTransferVersion,
		src.getOrder(),
		[]string{src.ConnectionID},
		dst.PortID,
		dst.ChannelID,
		signer,
	)
}

// ChanTry creates a MsgChannelOpenTry
func (src *PathEnd) ChanTry(dst *PathEnd, dstChanState chanTypes.ChannelResponse, signer sdk.AccAddress) sdk.Msg {
	return chanTypes.NewMsgChannelOpenTry(
		src.PortID,
		src.ChannelID,
		defaultTransferVersion,
		dstChanState.Channel.Channel.Ordering,
		[]string{src.ConnectionID},
		dst.PortID,
		dst.ChannelID,
		dstChanState.Channel.Channel.GetVersion(),
		dstChanState.Proof,
		dstChanState.ProofHeight+1,
		signer,
	)
}

// ChanAck creates a MsgChannelOpenAck
func (src *PathEnd) ChanAck(dstChanState chanTypes.ChannelResponse, signer sdk.AccAddress) sdk.Msg {
	return chanTypes.NewMsgChannelOpenAck(
		src.PortID,
		src.ChannelID,
		dstChanState.Channel.Channel.GetVersion(),
		dstChanState.Proof,
		dstChanState.ProofHeight+1,
		signer,
	)
}

// ChanConfirm creates a MsgChannelOpenConfirm
func (src *PathEnd) ChanConfirm(dstChanState chanTypes.ChannelResponse, signer sdk.AccAddress) sdk.Msg {
	return chanTypes.NewMsgChannelOpenConfirm(
		src.PortID,
		src.ChannelID,
		dstChanState.Proof,
		dstChanState.ProofHeight+1,
		signer,
	)
}

// ChanCloseInit creates a MsgChannelCloseInit
func (src *PathEnd) ChanCloseInit(signer sdk.AccAddress) sdk.Msg {
	return chanTypes.NewMsgChannelCloseInit(
		src.PortID,
		src.ChannelID,
		signer,
	)
}

// ChanCloseConfirm creates a MsgChannelCloseConfirm
func (src *PathEnd) ChanCloseConfirm(dstChanState chanTypes.ChannelResponse, signer sdk.AccAddress) sdk.Msg {
	return chanTypes.NewMsgChannelCloseConfirm(
		src.PortID,
		src.ChannelID,
		dstChanState.Proof,
		dstChanState.ProofHeight+1,
		signer,
	)
}

// MsgRecvPacket creates a MsgPacket
func (src *PathEnd) MsgRecvPacket(dst *PathEnd, sequence, timeoutHeight, timeoutStamp uint64, packetData []byte, proof commitmenttypes.MerkleProof, proofHeight uint64, signer sdk.AccAddress) sdk.Msg {
	return chanTypes.NewMsgPacket(
		dst.NewPacket(
			src,
			sequence,
			packetData,
			timeoutHeight,
			timeoutStamp,
		),
		proof,
		proofHeight+1,
		signer,
	)
}

// MsgTimeout creates MsgTimeout
func (src *PathEnd) MsgTimeout(packet chanTypes.Packet, seq uint64, proof chanTypes.PacketResponse, signer sdk.AccAddress) sdk.Msg {
	return chanTypes.NewMsgTimeout(
		packet,
		seq,
		proof.Proof,
		proof.ProofHeight+1,
		signer,
	)
}

// MsgAck creates MsgAck
func (src *PathEnd) MsgAck(dst *PathEnd, sequence, timeoutHeight, timeoutStamp uint64, ack, packetData []byte, proof commitmenttypes.MerkleProof, proofHeight uint64, signer sdk.AccAddress) sdk.Msg {
	return chanTypes.NewMsgAcknowledgement(
		src.NewPacket(
			dst,
			sequence,
			packetData,
			timeoutHeight,
			timeoutStamp,
		),
		ack,
		proof,
		proofHeight+1,
		signer,
	)
}

// MsgTransfer creates a new transfer message
func (src *PathEnd) MsgTransfer(dst *PathEnd, dstHeight uint64, amount sdk.Coins, dstAddr string, signer sdk.AccAddress) sdk.Msg {
	return xferTypes.NewMsgTransfer(
		src.PortID,
		src.ChannelID,
		dstHeight,
		amount,
		signer,
		dstAddr,
	)
}

// MsgSendPacket creates a new arbitrary packet message
func (src *PathEnd) MsgSendPacket(dst *PathEnd, packetData []byte, relativeTimeout, timeoutStamp uint64, signer sdk.AccAddress) sdk.Msg {
	// NOTE: Use this just to pass the packet integrity checks.
	fakeSequence := uint64(1)
	packet := chanTypes.NewPacket(packetData, fakeSequence, src.PortID, src.ChannelID, dst.PortID, dst.ChannelID, relativeTimeout, timeoutStamp)
	return NewMsgSendPacket(packet, signer)
}

// NewPacket returns a new packet from src to dist w
func (src *PathEnd) NewPacket(dst *PathEnd, sequence uint64, packetData []byte, timeoutHeight, timeoutStamp uint64) chanTypes.Packet {
	return chanTypes.NewPacket(
		packetData,
		sequence,
		src.PortID,
		src.ChannelID,
		dst.PortID,
		dst.ChannelID,
		timeoutHeight,
		timeoutStamp,
	)
}

// XferPacket creates a new transfer packet
func (src *PathEnd) XferPacket(amount sdk.Coins, sender, reciever string) []byte {
	return xferTypes.NewFungibleTokenPacketData(
		amount,
		sender,
		reciever,
	).GetBytes()
}

// PacketMsg returns a new MsgPacket for forwarding packets from one chain to another
func (src *Chain) PacketMsg(dst *Chain, xferPacket []byte, timeout, timeoutStamp uint64, seq int64, dstCommitRes CommitmentResponse) sdk.Msg {
	return src.PathEnd.MsgRecvPacket(
		dst.PathEnd,
		uint64(seq),
		timeout,
		timeoutStamp,
		xferPacket,
		dstCommitRes.Proof,
		dstCommitRes.ProofHeight,
		src.MustGetAddress(),
	)
}
