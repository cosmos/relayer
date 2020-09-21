package relayer

import (
	"strings"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	xferTypes "github.com/cosmos/cosmos-sdk/x/ibc-transfer/types"
	clientTypes "github.com/cosmos/cosmos-sdk/x/ibc/02-client/types"
	connTypes "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/types"
	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint/types"
	commitmenttypes "github.com/cosmos/cosmos-sdk/x/ibc/23-commitment/types"
	"github.com/cosmos/cosmos-sdk/x/ibc/exported"
	"github.com/tendermint/tendermint/light"
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
	Version      string `yaml:"version,omitempty" json:"version,omitempty"`
}

// OrderFromString parses a string into a channel order byte
func OrderFromString(order string) chanTypes.Order {
	switch order {
	case "UNORDERED":
		return chanTypes.UNORDERED
	case "ORDERED":
		return chanTypes.ORDERED
	default:
		return chanTypes.NONE
	}
}

func (pe *PathEnd) getOrder() chanTypes.Order {
	return OrderFromString(strings.ToUpper(pe.Order))
}

// UpdateClient creates an sdk.Msg to update the client on src with data pulled from dst
func (pe *PathEnd) UpdateClient(dstHeader exported.Header, signer sdk.AccAddress) sdk.Msg {
	if err := dstHeader.ValidateBasic(); err != nil {
		panic(err)
	}
	msg, err := clientTypes.NewMsgUpdateClient(
		pe.ClientID,
		dstHeader,
		signer,
	)
	if err != nil {
		panic(err)
	}
	return msg
}

// CreateClient creates an sdk.Msg to update the client on src with consensus state from dst
func (pe *PathEnd) CreateClient(
	dstHeader *tmclient.Header,
	trustingPeriod, unbondingPeriod time.Duration,
	signer sdk.AccAddress) sdk.Msg {
	if err := dstHeader.ValidateBasic(); err != nil {
		panic(err)
	}

	// Blank Client State
	// TODO: figure out how to dynmaically set unbonding time
	clientState := tmclient.NewClientState(
		dstHeader.GetHeader().GetChainID(),
		tmclient.NewFractionFromTm(light.DefaultTrustLevel),
		trustingPeriod,
		unbondingPeriod,
		time.Minute*1,
		dstHeader.GetHeight().(clientTypes.Height),
		commitmenttypes.GetSDKSpecs(),
		false,
		false,
	)

	msg, err := clientTypes.NewMsgCreateClient(
		pe.ClientID,
		clientState,
		dstHeader.ConsensusState(),
		signer,
	)

	if err != nil {
		panic(err)
	}
	if err = msg.ValidateBasic(); err != nil {
		panic(err)
	}
	return msg
}

// ConnInit creates a MsgConnectionOpenInit
func (pe *PathEnd) ConnInit(dst *PathEnd, signer sdk.AccAddress) sdk.Msg {
	return connTypes.NewMsgConnectionOpenInit(
		pe.ConnectionID,
		pe.ClientID,
		dst.ConnectionID,
		dst.ClientID,
		defaultChainPrefix,
		signer,
	)
}

// ConnTry creates a MsgConnectionOpenTry
// NOTE: ADD NOTE ABOUT PROOF HEIGHT CHANGE HERE
func (pe *PathEnd) ConnTry(
	dst *PathEnd,
	dstClientState *clientTypes.QueryClientStateResponse,
	dstConnState *connTypes.QueryConnectionResponse,
	dstConsState *clientTypes.QueryConsensusStateResponse,
	signer sdk.AccAddress,
) sdk.Msg {
	cs, err := clientTypes.UnpackClientState(dstClientState.ClientState)
	if err != nil {
		panic(err)
	}
	msg := connTypes.NewMsgConnectionOpenTry(
		pe.ConnectionID,
		pe.ClientID,
		dst.ConnectionID,
		dst.ClientID,
		cs,
		defaultChainPrefix,
		connTypes.GetCompatibleEncodedVersions(),
		dstConnState.Proof,
		dstClientState.Proof,
		dstConsState.Proof,
		dstConnState.ProofHeight,
		cs.GetLatestHeight().(clientTypes.Height),
		signer,
	)
	if err = msg.ValidateBasic(); err != nil {
		panic(err)
	}
	return msg
}

// ConnAck creates a MsgConnectionOpenAck
// NOTE: ADD NOTE ABOUT PROOF HEIGHT CHANGE HERE
func (pe *PathEnd) ConnAck(
	dst *PathEnd,
	dstClientState *clientTypes.QueryClientStateResponse,
	dstConnState *connTypes.QueryConnectionResponse,
	dstConsState *clientTypes.QueryConsensusStateResponse,
	signer sdk.AccAddress,
) sdk.Msg {
	cs, err := clientTypes.UnpackClientState(dstClientState.ClientState)
	if err != nil {
		panic(err)
	}
	return connTypes.NewMsgConnectionOpenAck(
		pe.ConnectionID,
		cs,
		dstConnState.Proof,
		dstClientState.Proof,
		dstConsState.Proof,
		dstConsState.ProofHeight,
		cs.GetLatestHeight().(clientTypes.Height),
		connTypes.GetCompatibleEncodedVersions()[0],
		signer,
	)
}

// ConnConfirm creates a MsgConnectionOpenAck
// NOTE: ADD NOTE ABOUT PROOF HEIGHT CHANGE HERE
func (pe *PathEnd) ConnConfirm(dstConnState *connTypes.QueryConnectionResponse, signer sdk.AccAddress) sdk.Msg {
	return connTypes.NewMsgConnectionOpenConfirm(
		pe.ConnectionID,
		dstConnState.Proof,
		dstConnState.ProofHeight,
		signer,
	)
}

// ChanInit creates a MsgChannelOpenInit
func (pe *PathEnd) ChanInit(dst *PathEnd, signer sdk.AccAddress) sdk.Msg {
	return chanTypes.NewMsgChannelOpenInit(
		pe.PortID,
		pe.ChannelID,
		pe.Version,
		pe.getOrder(),
		[]string{pe.ConnectionID},
		dst.PortID,
		dst.ChannelID,
		signer,
	)
}

// ChanTry creates a MsgChannelOpenTry
func (pe *PathEnd) ChanTry(dst *PathEnd, dstChanState *chanTypes.QueryChannelResponse, signer sdk.AccAddress) sdk.Msg {
	return chanTypes.NewMsgChannelOpenTry(
		pe.PortID,
		pe.ChannelID,
		pe.Version,
		dstChanState.Channel.Ordering,
		[]string{pe.ConnectionID},
		dst.PortID,
		dst.ChannelID,
		dstChanState.Channel.Version,
		dstChanState.Proof,
		dstChanState.ProofHeight,
		signer,
	)
}

// ChanAck creates a MsgChannelOpenAck
func (pe *PathEnd) ChanAck(dstChanState *chanTypes.QueryChannelResponse, signer sdk.AccAddress) sdk.Msg {
	return chanTypes.NewMsgChannelOpenAck(
		pe.PortID,
		pe.ChannelID,
		dstChanState.Channel.Version,
		dstChanState.Proof,
		dstChanState.ProofHeight,
		signer,
	)
}

// ChanConfirm creates a MsgChannelOpenConfirm
func (pe *PathEnd) ChanConfirm(dstChanState *chanTypes.QueryChannelResponse, signer sdk.AccAddress) sdk.Msg {
	return chanTypes.NewMsgChannelOpenConfirm(
		pe.PortID,
		pe.ChannelID,
		dstChanState.Proof,
		dstChanState.ProofHeight,
		signer,
	)
}

// ChanCloseInit creates a MsgChannelCloseInit
func (pe *PathEnd) ChanCloseInit(signer sdk.AccAddress) sdk.Msg {
	return chanTypes.NewMsgChannelCloseInit(
		pe.PortID,
		pe.ChannelID,
		signer,
	)
}

// ChanCloseConfirm creates a MsgChannelCloseConfirm
func (pe *PathEnd) ChanCloseConfirm(dstChanState *chanTypes.QueryChannelResponse, signer sdk.AccAddress) sdk.Msg {
	return chanTypes.NewMsgChannelCloseConfirm(
		pe.PortID,
		pe.ChannelID,
		dstChanState.Proof,
		dstChanState.ProofHeight,
		signer,
	)
}

// MsgTransfer creates a new transfer message
func (pe *PathEnd) MsgTransfer(dst *PathEnd, amount sdk.Coin, dstAddr string,
	signer sdk.AccAddress, timeoutHeight, timeoutTimestamp uint64) sdk.Msg {

	return xferTypes.NewMsgTransfer(
		pe.PortID,
		pe.ChannelID,
		amount,
		signer,
		dstAddr,
		clientTypes.NewHeight(0, timeoutHeight),
		timeoutTimestamp,
	)
}

// NewPacket returns a new packet from src to dist w
func (pe *PathEnd) NewPacket(dst *PathEnd, sequence uint64, packetData []byte,
	timeoutHeight, timeoutStamp uint64) chanTypes.Packet {
	return chanTypes.NewPacket(
		packetData,
		sequence,
		pe.PortID,
		pe.ChannelID,
		dst.PortID,
		dst.ChannelID,
		clientTypes.NewHeight(0, timeoutHeight),
		timeoutStamp,
	)
}

// XferPacket creates a new transfer packet
func (pe *PathEnd) XferPacket(amount sdk.Coin, sender, receiver string) []byte {
	return xferTypes.NewFungibleTokenPacketData(
		amount.Denom,
		amount.Amount.Uint64(),
		sender,
		receiver,
	).GetBytes()
}
