package relayer

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/cosmos-sdk/x/ibc/applications/transfer/types"
	clienttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/types"
	conntypes "github.com/cosmos/cosmos-sdk/x/ibc/core/03-connection/types"
	chantypes "github.com/cosmos/cosmos-sdk/x/ibc/core/04-channel/types"
	commitmenttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/23-commitment/types"
	ibcexported "github.com/cosmos/cosmos-sdk/x/ibc/core/exported"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
	light "github.com/tendermint/tendermint/light"
)

// NOTE: we explicitly call 'MustGetAddress' before 'NewMsg...'
// to ensure the correct config file is being used to generate
// the account prefix. 'NewMsg...' functions take an AccAddress
// rather than a string. The 'address.String()' function uses
// the currently set config file. Querying a counterparty would
// swap the config file. 'MustGetAddress' sets the config file
// correctly. Do not change this ordering until the SDK config
// file handling has been refactored.
// https://github.com/cosmos/cosmos-sdk/issues/8332

// UpdateClient creates an sdk.Msg to update the client on src with data pulled from dst
func (c *Chain) UpdateClient(dstHeader ibcexported.Header) sdk.Msg {
	if err := dstHeader.ValidateBasic(); err != nil {
		panic(err)
	}
	msg, err := clienttypes.NewMsgUpdateClient(
		c.PathEnd.ClientID,
		dstHeader,
		c.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	)
	if err != nil {
		panic(err)
	}
	return msg
}

// ConnTry creates a MsgConnectionOpenTry
func (c *Chain) ConnTry(
	counterparty *Chain,
	height uint64,
) (sdk.Msg, error) {
	clientState, clientStateProof, consensusStateProof, connStateProof, proofHeight, err := counterparty.GenerateConnHandshakeProof(height)
	if err != nil {
		return nil, err
	}

	// TODO: Get DelayPeriod from counterparty connection rather than using default value
	msg := conntypes.NewMsgConnectionOpenTry(
		c.PathEnd.ConnectionID,
		c.PathEnd.ClientID,
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
		c.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	)
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	return msg, nil
}

// ConnAck creates a MsgConnectionOpenAck
func (c *Chain) ConnAck(
	counterparty *Chain,
	height uint64,
) (sdk.Msg, error) {
	clientState, clientStateProof, consensusStateProof, connStateProof, proofHeight, err := counterparty.GenerateConnHandshakeProof(height)
	if err != nil {
		return nil, err
	}

	return conntypes.NewMsgConnectionOpenAck(
		c.PathEnd.ConnectionID,
		counterparty.PathEnd.ConnectionID,
		clientState,
		connStateProof,
		clientStateProof,
		consensusStateProof,
		proofHeight,
		clientState.GetLatestHeight().(clienttypes.Height),
		conntypes.DefaultIBCVersion,
		c.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	), nil
}

// ChanTry creates a MsgChannelOpenTry
func (c *Chain) ChanTry(
	counterparty *Chain,
	height uint64,
) (sdk.Msg, error) {
	// obtain proof from counterparty chain
	counterpartyChannelRes, err := counterparty.QueryChannel(int64(height))
	if err != nil {
		return nil, err
	}

	return chantypes.NewMsgChannelOpenTry(
		c.PathEnd.PortID,
		c.PathEnd.ChannelID,
		c.PathEnd.Version,
		counterpartyChannelRes.Channel.Ordering,
		[]string{c.PathEnd.ConnectionID},
		counterparty.PathEnd.PortID,
		counterparty.PathEnd.ChannelID,
		counterpartyChannelRes.Channel.Version,
		counterpartyChannelRes.Proof,
		counterpartyChannelRes.ProofHeight,
		c.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'

	), nil
}

// ChanAck creates a MsgChannelOpenAck
func (c *Chain) ChanAck(
	counterparty *Chain,
	height uint64,
) (sdk.Msg, error) {
	// obtain proof from counterparty chain
	counterpartyChannelRes, err := counterparty.QueryChannel(int64(height))
	if err != nil {
		return nil, err
	}

	return chantypes.NewMsgChannelOpenAck(
		c.PathEnd.PortID,
		c.PathEnd.ChannelID,
		counterparty.PathEnd.ChannelID,
		counterpartyChannelRes.Channel.Version,
		counterpartyChannelRes.Proof,
		counterpartyChannelRes.ProofHeight,
		c.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	), nil
}

// MsgTransfer creates a new transfer message
func (c *Chain) MsgTransfer(dst *PathEnd, amount sdk.Coin, dstAddr string,
	timeoutHeight, timeoutTimestamp uint64) sdk.Msg {

	version := clienttypes.ParseChainID(dst.ChainID)
	return transfertypes.NewMsgTransfer(
		c.PathEnd.PortID,
		c.PathEnd.ChannelID,
		amount,
		c.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
		dstAddr,
		clienttypes.NewHeight(version, timeoutHeight),
		timeoutTimestamp,
	)
}

// CreateClient creates an sdk.Msg to update the client on src with consensus state from dst
func (c *Chain) CreateClient(
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
func (c *Chain) ConnInit(counterparty *PathEnd, signer sdk.AccAddress) sdk.Msg {
	var version *conntypes.Version
	return conntypes.NewMsgConnectionOpenInit(
		c.PathEnd.ClientID,
		counterparty.ClientID,
		defaultChainPrefix,
		version,
		defaultDelayPeriod,
		signer,
	)
}

// ConnConfirm creates a MsgConnectionOpenConfirm
func (c *Chain) ConnConfirm(counterpartyConnState *conntypes.QueryConnectionResponse, signer sdk.AccAddress) sdk.Msg {
	return conntypes.NewMsgConnectionOpenConfirm(
		c.PathEnd.ConnectionID,
		counterpartyConnState.Proof,
		counterpartyConnState.ProofHeight,
		signer,
	)
}

// ChanInit creates a MsgChannelOpenInit
func (c *Chain) ChanInit(counterparty *PathEnd, signer sdk.AccAddress) sdk.Msg {
	return chantypes.NewMsgChannelOpenInit(
		c.PathEnd.PortID,
		c.PathEnd.Version,
		c.PathEnd.GetOrder(),
		[]string{c.PathEnd.ConnectionID},
		counterparty.PortID,
		signer,
	)
}

// ChanConfirm creates a MsgChannelOpenConfirm
func (c *Chain) ChanConfirm(dstChanState *chantypes.QueryChannelResponse, signer sdk.AccAddress) sdk.Msg {
	return chantypes.NewMsgChannelOpenConfirm(
		c.PathEnd.PortID,
		c.PathEnd.ChannelID,
		dstChanState.Proof,
		dstChanState.ProofHeight,
		signer,
	)
}

// ChanCloseInit creates a MsgChannelCloseInit
func (c *Chain) ChanCloseInit(signer sdk.AccAddress) sdk.Msg {
	return chantypes.NewMsgChannelCloseInit(
		c.PathEnd.PortID,
		c.PathEnd.ChannelID,
		signer,
	)
}

// ChanCloseConfirm creates a MsgChannelCloseConfirm
func (c *Chain) ChanCloseConfirm(dstChanState *chantypes.QueryChannelResponse, signer sdk.AccAddress) sdk.Msg {
	return chantypes.NewMsgChannelCloseConfirm(
		c.PathEnd.PortID,
		c.PathEnd.ChannelID,
		dstChanState.Proof,
		dstChanState.ProofHeight,
		signer,
	)
}
