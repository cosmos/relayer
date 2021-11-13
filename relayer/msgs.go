package relayer

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v2/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v2/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v2/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v2/modules/core/04-channel/types"
	tmclient "github.com/cosmos/ibc-go/v2/modules/light-clients/07-tendermint/types"
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

// CreateClient creates an sdk.Msg to update the client on src with consensus state from dst
func (c *Chain) CreateClient(
	//nolint:interfacer
	clientState *tmclient.ClientState,
	dstHeader *tmclient.Header) (sdk.Msg, error) {

	if err := dstHeader.ValidateBasic(); err != nil {
		return nil, err
	}

	msg, err := clienttypes.NewMsgCreateClient(
		clientState,
		dstHeader.ConsensusState(),
		c.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	)

	if err != nil {
		return nil, err
	}
	if err = msg.ValidateBasic(); err != nil {
		return nil, err
	}
	return msg, nil
}

// UpdateClient creates an sdk.Msg to update the client on src with data pulled from dst
// at the request height..
func (c *Chain) UpdateClient(dst *Chain, dsth *tmclient.Header) (sdk.Msg, error) {
	if err := dsth.ValidateBasic(); err != nil {
		return nil, err
	}
	msg, err := clienttypes.NewMsgUpdateClient(
		c.PathEnd.ClientID,
		dsth,
		c.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// ConnInit creates a MsgConnectionOpenInit
func (c *Chain) ConnInit(counterparty *Chain, counterpartyHeader *tmclient.Header) ([]sdk.Msg, error) {
	updateMsg, err := c.UpdateClient(counterparty, counterpartyHeader)
	if err != nil {
		return nil, err
	}

	var version *conntypes.Version
	msg := conntypes.NewMsgConnectionOpenInit(
		c.PathEnd.ClientID,
		counterparty.PathEnd.ClientID,
		defaultChainPrefix,
		version,
		defaultDelayPeriod,
		c.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	)

	return []sdk.Msg{updateMsg, msg}, nil

}

// ConnTry creates a MsgConnectionOpenTry
func (c *Chain) ConnTry(counterparty *Chain, counterpartyHeader *tmclient.Header) ([]sdk.Msg, error) {
	updateMsg, err := c.UpdateClient(counterparty, counterpartyHeader)
	if err != nil {
		return nil, err
	}

	cph, err := counterparty.QueryLatestHeight()
	if err != nil {
		return nil, err
	}

	clientState, clientStateProof, consensusStateProof, connStateProof,
		proofHeight, err := counterparty.GenerateConnHandshakeProof(uint64(cph))
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

	return []sdk.Msg{updateMsg, msg}, nil
}

// ConnAck creates a MsgConnectionOpenAck
func (c *Chain) ConnAck(counterparty *Chain, counterpartyHeader *tmclient.Header) ([]sdk.Msg, error) {
	updateMsg, err := c.UpdateClient(counterparty, counterpartyHeader)
	if err != nil {
		return nil, err
	}
	cph, err := counterparty.QueryLatestHeight()
	if err != nil {
		return nil, err
	}

	clientState, clientStateProof, consensusStateProof, connStateProof,
		proofHeight, err := counterparty.GenerateConnHandshakeProof(uint64(cph))
	if err != nil {
		return nil, err
	}

	msg := conntypes.NewMsgConnectionOpenAck(
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
	)

	return []sdk.Msg{updateMsg, msg}, nil
}

// ConnConfirm creates a MsgConnectionOpenConfirm
func (c *Chain) ConnConfirm(counterparty *Chain, counterpartyHeader *tmclient.Header) ([]sdk.Msg, error) {
	updateMsg, err := c.UpdateClient(counterparty, counterpartyHeader)
	if err != nil {
		return nil, err
	}

	cph, err := counterparty.QueryLatestHeight()
	if err != nil {
		return nil, err
	}
	counterpartyConnState, err := counterparty.QueryConnection(cph)
	if err != nil {
		return nil, err
	}

	msg := conntypes.NewMsgConnectionOpenConfirm(
		c.PathEnd.ConnectionID,
		counterpartyConnState.Proof,
		counterpartyConnState.ProofHeight,
		c.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	)

	return []sdk.Msg{updateMsg, msg}, nil
}

// ChanInit creates a MsgChannelOpenInit
func (c *Chain) ChanInit(counterparty *Chain, counterpartyHeader *tmclient.Header) ([]sdk.Msg, error) {
	updateMsg, err := c.UpdateClient(counterparty, counterpartyHeader)
	if err != nil {
		return nil, err
	}

	msg := chantypes.NewMsgChannelOpenInit(
		c.PathEnd.PortID,
		c.PathEnd.Version,
		c.PathEnd.GetOrder(),
		[]string{c.PathEnd.ConnectionID},
		counterparty.PathEnd.PortID,
		c.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	)

	return []sdk.Msg{updateMsg, msg}, nil
}

// ChanTry creates a MsgChannelOpenTry
func (c *Chain) ChanTry(counterparty *Chain, counterpartyHeader *tmclient.Header) ([]sdk.Msg, error) {
	updateMsg, err := c.UpdateClient(counterparty, counterpartyHeader)
	if err != nil {
		return nil, err
	}
	cph, err := counterparty.QueryLatestHeight()
	if err != nil {
		return nil, err
	}

	counterpartyChannelRes, err := counterparty.QueryChannel(cph)
	if err != nil {
		return nil, err
	}

	msg := chantypes.NewMsgChannelOpenTry(
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

	)

	return []sdk.Msg{updateMsg, msg}, nil
}

// ChanAck creates a MsgChannelOpenAck
func (c *Chain) ChanAck(counterparty *Chain, counterpartyHeader *tmclient.Header) ([]sdk.Msg, error) {
	updateMsg, err := c.UpdateClient(counterparty, counterpartyHeader)
	if err != nil {
		return nil, err
	}

	cph, err := counterparty.QueryLatestHeight()
	if err != nil {
		return nil, err
	}

	counterpartyChannelRes, err := counterparty.QueryChannel(cph)
	if err != nil {
		return nil, err
	}

	msg := chantypes.NewMsgChannelOpenAck(
		c.PathEnd.PortID,
		c.PathEnd.ChannelID,
		counterparty.PathEnd.ChannelID,
		counterpartyChannelRes.Channel.Version,
		counterpartyChannelRes.Proof,
		counterpartyChannelRes.ProofHeight,
		c.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	)

	return []sdk.Msg{updateMsg, msg}, nil
}

// ChanConfirm creates a MsgChannelOpenConfirm
func (c *Chain) ChanConfirm(counterparty *Chain, counterpartyHeader *tmclient.Header) ([]sdk.Msg, error) {
	updateMsg, err := c.UpdateClient(counterparty, counterpartyHeader)
	if err != nil {
		return nil, err
	}
	cph, err := counterparty.QueryLatestHeight()
	if err != nil {
		return nil, err
	}

	counterpartyChanState, err := counterparty.QueryChannel(cph)
	if err != nil {
		return nil, err
	}

	msg := chantypes.NewMsgChannelOpenConfirm(
		c.PathEnd.PortID,
		c.PathEnd.ChannelID,
		counterpartyChanState.Proof,
		counterpartyChanState.ProofHeight,
		c.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	)

	return []sdk.Msg{updateMsg, msg}, nil
}

// ChanCloseInit creates a MsgChannelCloseInit
func (c *Chain) ChanCloseInit() sdk.Msg {
	return chantypes.NewMsgChannelCloseInit(
		c.PathEnd.PortID,
		c.PathEnd.ChannelID,
		c.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	)
}

// ChanCloseConfirm creates a MsgChannelCloseConfirm
func (c *Chain) ChanCloseConfirm(dstChanState *chantypes.QueryChannelResponse) sdk.Msg {
	return chantypes.NewMsgChannelCloseConfirm(
		c.PathEnd.PortID,
		c.PathEnd.ChannelID,
		dstChanState.Proof,
		dstChanState.ProofHeight,
		c.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	)
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

// MsgRelayRecvPacket constructs the MsgRecvPacket which is to be sent to the receiving chain.
// The counterparty represents the sending chain where the packet commitment would be stored.
func (c *Chain) MsgRelayRecvPacket(counterparty *Chain, counterpartyHeight int64, packet *relayMsgRecvPacket) (sdk.Msg, error) {
	comRes, err := counterparty.QueryPacketCommitment(counterpartyHeight, packet.seq)
	switch {
	case err != nil:
		return nil, err
	case comRes.Proof == nil || comRes.Commitment == nil:
		return nil, fmt.Errorf("recv packet commitment query seq(%d) is nil", packet.seq)
	case comRes == nil:
		return nil, fmt.Errorf("receive packet [%s]seq{%d} has no associated proofs", c.ChainID, packet.seq)
	default:
		return chantypes.NewMsgRecvPacket(
			chantypes.NewPacket(
				packet.packetData,
				packet.seq,
				counterparty.PathEnd.PortID,
				counterparty.PathEnd.ChannelID,
				c.PathEnd.PortID,
				c.PathEnd.ChannelID,
				packet.timeout,
				packet.timeoutStamp,
			),
			comRes.Proof,
			comRes.ProofHeight,
			c.MustGetAddress(),
		), nil
	}
}

// MsgRelayAcknowledgement constructs the MsgAcknowledgement which is to be sent to the sending chain.
// The counterparty represents the receiving chain where the acknowledgement would be stored.
func (c *Chain) MsgRelayAcknowledgement(counterparty *Chain, counterpartyHeight int64, packet *relayMsgPacketAck) (sdk.Msg, error) {
	ackRes, err := counterparty.QueryPacketAcknowledgement(counterpartyHeight, packet.seq)
	switch {
	case err != nil:
		return nil, err
	case ackRes.Proof == nil || ackRes.Acknowledgement == nil:
		return nil, fmt.Errorf("ack packet acknowledgement query seq(%d) is nil", packet.seq)
	case ackRes == nil:
		return nil, fmt.Errorf("ack packet [%s]seq{%d} has no associated proofs", counterparty.ChainID, packet.seq)
	default:
		return chantypes.NewMsgAcknowledgement(
			chantypes.NewPacket(
				packet.packetData,
				packet.seq,
				c.PathEnd.PortID,
				c.PathEnd.ChannelID,
				counterparty.PathEnd.PortID,
				counterparty.PathEnd.ChannelID,
				packet.timeout,
				packet.timeoutStamp,
			),
			packet.ack,
			ackRes.Proof,
			ackRes.ProofHeight,
			c.MustGetAddress()), nil
	}
}

// MsgRelayTimeout constructs the MsgTimeout which is to be sent to the sending chain.
// The counterparty represents the receiving chain where the receipts would have been
// stored.
func (c *Chain) MsgRelayTimeout(counterparty *Chain, counterpartyHeight int64, packet *relayMsgTimeout) (sdk.Msg, error) {
	recvRes, err := counterparty.QueryPacketReceipt(counterpartyHeight, packet.seq)
	switch {
	case err != nil:
		return nil, err
	case recvRes.Proof == nil:
		return nil, fmt.Errorf("timeout packet receipt proof seq(%d) is nil", packet.seq)
	case recvRes == nil:
		return nil, fmt.Errorf("timeout packet [%s]seq{%d} has no associated proofs", c.ChainID, packet.seq)
	default:
		return chantypes.NewMsgTimeout(
			chantypes.NewPacket(
				packet.packetData,
				packet.seq,
				c.PathEnd.PortID,
				c.PathEnd.ChannelID,
				counterparty.PathEnd.PortID,
				counterparty.PathEnd.ChannelID,
				packet.timeout,
				packet.timeoutStamp,
			),
			packet.seq,
			recvRes.Proof,
			recvRes.ProofHeight,
			c.MustGetAddress(),
		), nil
	}
}
