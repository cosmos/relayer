package relayer

import (
	"strings"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/cosmos-sdk/x/ibc/applications/transfer/types"
	clienttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/types"
	conntypes "github.com/cosmos/cosmos-sdk/x/ibc/core/03-connection/types"
	chantypes "github.com/cosmos/cosmos-sdk/x/ibc/core/04-channel/types"
	commitmenttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/23-commitment/types"
	ibcexported "github.com/cosmos/cosmos-sdk/x/ibc/core/exported"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
	"github.com/tendermint/tendermint/light"
)

var (
	defaultChainPrefix = commitmenttypes.NewMerklePrefix([]byte("ibc"))
	defaultDelayPeriod = uint64(0)
	defaultUpgradePath = []string{"upgrade", "upgradedIBCState"}
)

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
func OrderFromString(order string) chantypes.Order {
	switch order {
	case "UNORDERED":
		return chantypes.UNORDERED
	case "ORDERED":
		return chantypes.ORDERED
	default:
		return chantypes.NONE
	}
}

func (pe *PathEnd) GetOrder() chantypes.Order {
	return OrderFromString(strings.ToUpper(pe.Order))
}

var marshalledChains = map[PathEnd]*Chain{}

func MarshalChain(c *Chain) PathEnd {
	pe := *c.PathEnd
	if _, ok := marshalledChains[pe]; !ok {
		marshalledChains[pe] = c
	}
	return pe
}

func UnmarshalChain(pe PathEnd) *Chain {
	if c, ok := marshalledChains[pe]; ok {
		return c
	}
	return nil
}

// UpdateClient creates an sdk.Msg to update the client on src with data pulled from dst
func (pe *PathEnd) UpdateClient(dstHeader ibcexported.Header, signer sdk.AccAddress) sdk.Msg {
	if err := dstHeader.ValidateBasic(); err != nil {
		panic(err)
	}
	msg, err := clienttypes.NewMsgUpdateClient(
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
	clientState := tmclient.NewClientState(
		dstHeader.GetHeader().GetChainID(),
		tmclient.NewFractionFromTm(light.DefaultTrustLevel),
		trustingPeriod,
		unbondingPeriod,
		time.Minute*10,
		dstHeader.GetHeight().(clienttypes.Height),
		commitmenttypes.GetSDKSpecs(),
		defaultUpgradePath,
		false,
		false,
	)

	msg, err := clienttypes.NewMsgCreateClient(
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
func (pe *PathEnd) ConnInit(counterparty *PathEnd, signer sdk.AccAddress) sdk.Msg {
	var version *conntypes.Version
	return conntypes.NewMsgConnectionOpenInit(
		pe.ClientID,
		counterparty.ClientID,
		defaultChainPrefix,
		version,
		defaultDelayPeriod,
		signer,
	)
}

// ConnTry creates a MsgConnectionOpenTry
func (pe *PathEnd) ConnTry(
	counterparty *Chain,
	height uint64,
	signer sdk.AccAddress,
) (sdk.Msg, error) {
	clientState, clientStateProof, consensusStateProof, connStateProof, proofHeight, err := counterparty.GenerateConnHandshakeProof(height)
	if err != nil {
		return nil, err
	}

	// TODO: Get DelayPeriod from counterparty connection rather than using default value
	msg := conntypes.NewMsgConnectionOpenTry(
		pe.ConnectionID,
		pe.ClientID,
		counterparty.PathEnd.ConnectionID,
		counterparty.PathEnd.ClientID,
		clientState,
		defaultChainPrefix,
		conntypes.ExportedVersionsToProto(conntypes.GetCompatibleVersions()),
		defaultDelayPeriod,
		connStateProof,
		clientStateProof,
		consensusStateProof,
		proofHeight,
		clientState.GetLatestHeight().(clienttypes.Height),
		signer,
	)
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	return msg, nil
}

// ConnAck creates a MsgConnectionOpenAck
func (pe *PathEnd) ConnAck(
	counterparty *Chain,
	height uint64,
	signer sdk.AccAddress,
) (sdk.Msg, error) {
	clientState, clientStateProof, consensusStateProof, connStateProof, proofHeight, err := counterparty.GenerateConnHandshakeProof(height)
	if err != nil {
		return nil, err
	}

	return conntypes.NewMsgConnectionOpenAck(
		pe.ConnectionID,
		counterparty.PathEnd.ConnectionID,
		clientState,
		connStateProof,
		clientStateProof,
		consensusStateProof,
		proofHeight,
		clientState.GetLatestHeight().(clienttypes.Height),
		conntypes.DefaultIBCVersion,
		signer,
	), nil
}

// ConnConfirm creates a MsgConnectionOpenConfirm
func (pe *PathEnd) ConnConfirm(counterpartyConnState *conntypes.QueryConnectionResponse, signer sdk.AccAddress) sdk.Msg {
	return conntypes.NewMsgConnectionOpenConfirm(
		pe.ConnectionID,
		counterpartyConnState.Proof,
		counterpartyConnState.ProofHeight,
		signer,
	)
}

// ChanInit creates a MsgChannelOpenInit
func (pe *PathEnd) ChanInit(counterparty *PathEnd, signer sdk.AccAddress) sdk.Msg {
	return chantypes.NewMsgChannelOpenInit(
		pe.PortID,
		pe.Version,
		pe.GetOrder(),
		[]string{pe.ConnectionID},
		counterparty.PortID,
		signer,
	)
}

// ChanTry creates a MsgChannelOpenTry
func (pe *PathEnd) ChanTry(counterparty *Chain, height uint64, signer sdk.AccAddress) (sdk.Msg, error) {
	// obtain proof from counterparty chain
	counterpartyChannelRes, err := counterparty.QueryChannel(int64(height))
	if err != nil {
		return nil, err
	}

	return chantypes.NewMsgChannelOpenTry(
		pe.PortID,
		pe.ChannelID,
		pe.Version,
		counterpartyChannelRes.Channel.Ordering,
		[]string{pe.ConnectionID},
		counterparty.PathEnd.PortID,
		counterparty.PathEnd.ChannelID,
		counterpartyChannelRes.Channel.Version,
		counterpartyChannelRes.Proof,
		counterpartyChannelRes.ProofHeight,
		signer,
	), nil
}

// ChanAck creates a MsgChannelOpenAck
func (pe *PathEnd) ChanAck(counterparty *Chain, height uint64, signer sdk.AccAddress) (sdk.Msg, error) {
	// obtain proof from counterparty chain
	counterpartyChannelRes, err := counterparty.QueryChannel(int64(height))
	if err != nil {
		return nil, err
	}

	return chantypes.NewMsgChannelOpenAck(
		pe.PortID,
		pe.ChannelID,
		counterparty.PathEnd.ChannelID,
		counterpartyChannelRes.Channel.Version,
		counterpartyChannelRes.Proof,
		counterpartyChannelRes.ProofHeight,
		signer,
	), nil
}

// ChanConfirm creates a MsgChannelOpenConfirm
func (pe *PathEnd) ChanConfirm(dstChanState *chantypes.QueryChannelResponse, signer sdk.AccAddress) sdk.Msg {
	return chantypes.NewMsgChannelOpenConfirm(
		pe.PortID,
		pe.ChannelID,
		dstChanState.Proof,
		dstChanState.ProofHeight,
		signer,
	)
}

// ChanCloseInit creates a MsgChannelCloseInit
func (pe *PathEnd) ChanCloseInit(signer sdk.AccAddress) sdk.Msg {
	return chantypes.NewMsgChannelCloseInit(
		pe.PortID,
		pe.ChannelID,
		signer,
	)
}

// ChanCloseConfirm creates a MsgChannelCloseConfirm
func (pe *PathEnd) ChanCloseConfirm(dstChanState *chantypes.QueryChannelResponse, signer sdk.AccAddress) sdk.Msg {
	return chantypes.NewMsgChannelCloseConfirm(
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

	version := clienttypes.ParseChainID(dst.ChainID)
	return transfertypes.NewMsgTransfer(
		pe.PortID,
		pe.ChannelID,
		amount,
		signer,
		dstAddr,
		clienttypes.NewHeight(version, timeoutHeight),
		timeoutTimestamp,
	)
}

// NewPacket returns a new packet from src to dist w
func (pe *PathEnd) NewPacket(dst *PathEnd, sequence uint64, packetData []byte,
	timeoutHeight, timeoutStamp uint64) chantypes.Packet {
	version := clienttypes.ParseChainID(dst.ChainID)
	return chantypes.NewPacket(
		packetData,
		sequence,
		pe.PortID,
		pe.ChannelID,
		dst.PortID,
		dst.ChannelID,
		clienttypes.NewHeight(version, timeoutHeight),
		timeoutStamp,
	)
}

// XferPacket creates a new transfer packet
func (pe *PathEnd) XferPacket(amount sdk.Coin, sender, receiver string) []byte {
	return transfertypes.NewFungibleTokenPacketData(
		amount.Denom,
		amount.Amount.Uint64(),
		sender,
		receiver,
	).GetBytes()
}
