package relayer

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/cosmos-sdk/x/ibc/applications/transfer/types"
	clienttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/types"
	conntypes "github.com/cosmos/cosmos-sdk/x/ibc/core/03-connection/types"
	chantypes "github.com/cosmos/cosmos-sdk/x/ibc/core/04-channel/types"
	ibcexported "github.com/cosmos/cosmos-sdk/x/ibc/core/exported"
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
