package relayer

import (
	"fmt"

	retry "github.com/avast/retry-go"
	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/cosmos-sdk/x/ibc/applications/transfer/types"
	clienttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/types"
	conntypes "github.com/cosmos/cosmos-sdk/x/ibc/core/03-connection/types"
	chantypes "github.com/cosmos/cosmos-sdk/x/ibc/core/04-channel/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
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
	dstHeader *tmclient.Header) sdk.Msg {

	if err := dstHeader.ValidateBasic(); err != nil {
		panic(err)
	}

	msg, err := clienttypes.NewMsgCreateClient(
		clientState,
		dstHeader.ConsensusState(),
		c.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	)

	if err != nil {
		panic(err)
	}
	if err = msg.ValidateBasic(); err != nil {
		panic(err)
	}
	return msg
}

// UpdateClient creates an sdk.Msg to update the client on src with data pulled from dst
// at the request height..
func (c *Chain) UpdateClient(dst *Chain) (sdk.Msg, error) {
	header, err := dst.GetIBCUpdateHeader(c)
	if err != nil {
		return nil, err
	}

	if err := header.ValidateBasic(); err != nil {
		return nil, err
	}
	msg, err := clienttypes.NewMsgUpdateClient(
		c.PathEnd.ClientID,
		header,
		c.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// ConnInit creates a MsgConnectionOpenInit
func (c *Chain) ConnInit(counterparty *Chain) ([]sdk.Msg, error) {
	updateMsg, err := c.UpdateClient(counterparty)
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
func (c *Chain) ConnTry(
	counterparty *Chain,
) ([]sdk.Msg, error) {
	updateMsg, err := c.UpdateClient(counterparty)
	if err != nil {
		return nil, err
	}

	// NOTE: the proof height uses - 1 due to tendermint's delayed execution model
	clientState, clientStateProof, consensusStateProof, connStateProof,
		proofHeight, err := counterparty.GenerateConnHandshakeProof(counterparty.MustGetLatestLightHeight() - 1)
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
func (c *Chain) ConnAck(
	counterparty *Chain,
) ([]sdk.Msg, error) {
	updateMsg, err := c.UpdateClient(counterparty)
	if err != nil {
		return nil, err
	}

	// NOTE: the proof height uses - 1 due to tendermint's delayed execution model
	clientState, clientStateProof, consensusStateProof, connStateProof,
		proofHeight, err := counterparty.GenerateConnHandshakeProof(counterparty.MustGetLatestLightHeight() - 1)
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
func (c *Chain) ConnConfirm(counterparty *Chain) ([]sdk.Msg, error) {
	updateMsg, err := c.UpdateClient(counterparty)
	if err != nil {
		return nil, err
	}

	counterpartyConnState, err := counterparty.QueryConnection(int64(counterparty.MustGetLatestLightHeight()) - 1)
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
func (c *Chain) ChanInit(counterparty *Chain) ([]sdk.Msg, error) {
	updateMsg, err := c.UpdateClient(counterparty)
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
func (c *Chain) ChanTry(
	counterparty *Chain,
) ([]sdk.Msg, error) {
	updateMsg, err := c.UpdateClient(counterparty)
	if err != nil {
		return nil, err
	}

	// NOTE: the proof height uses - 1 due to tendermint's delayed execution model
	counterpartyChannelRes, err := counterparty.QueryChannel(int64(counterparty.MustGetLatestLightHeight()) - 1)
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
func (c *Chain) ChanAck(
	counterparty *Chain,
) ([]sdk.Msg, error) {
	updateMsg, err := c.UpdateClient(counterparty)
	if err != nil {
		return nil, err
	}

	// NOTE: the proof height uses - 1 due to tendermint's delayed execution model
	counterpartyChannelRes, err := counterparty.QueryChannel(int64(counterparty.MustGetLatestLightHeight()) - 1)
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
func (c *Chain) ChanConfirm(counterparty *Chain) ([]sdk.Msg, error) {
	updateMsg, err := c.UpdateClient(counterparty)
	if err != nil {
		return nil, err
	}

	// NOTE: the proof height uses - 1 due to tendermint's delayed execution model
	counterpartyChanState, err := counterparty.QueryChannel(int64(counterparty.MustGetLatestLightHeight()) - 1)
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
func (c *Chain) MsgRelayRecvPacket(counterparty *Chain, packet *relayMsgRecvPacket) (msgs []sdk.Msg, err error) {
	var comRes *chantypes.QueryPacketCommitmentResponse
	if err = retry.Do(func() (err error) {
		// NOTE: the proof height uses - 1 due to tendermint's delayed execution model
		comRes, err = counterparty.QueryPacketCommitment(int64(counterparty.MustGetLatestLightHeight())-1, packet.seq)
		if err != nil {
			return err
		}

		if comRes.Proof == nil || comRes.Commitment == nil {
			return fmt.Errorf("recv packet commitment query seq(%d) is nil", packet.seq)
		}

		return nil
	}, rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, _ error) {
		// clear messages
		msgs = []sdk.Msg{}

		// OnRetry we want to update the light clients and then debug log
		updateMsg, err := c.UpdateClient(counterparty)
		if err != nil {
			return
		}

		msgs = append(msgs, updateMsg)

		if counterparty.debug {
			counterparty.Log(fmt.Sprintf("- [%s]@{%d} - try(%d/%d) query packet commitment: %s",
				counterparty.ChainID, counterparty.MustGetLatestLightHeight()-1, n+1, rtyAttNum, err))
		}

	})); err != nil {
		counterparty.Error(err)
		return
	}

	if comRes == nil {
		return nil, fmt.Errorf("receive packet [%s]seq{%d} has no associated proofs", c.ChainID, packet.seq)
	}

	version := clienttypes.ParseChainID(c.ChainID)
	msg := chantypes.NewMsgRecvPacket(
		chantypes.NewPacket(
			packet.packetData,
			packet.seq,
			counterparty.PathEnd.PortID,
			counterparty.PathEnd.ChannelID,
			c.PathEnd.PortID,
			c.PathEnd.ChannelID,
			clienttypes.NewHeight(version, packet.timeout),
			packet.timeoutStamp,
		),
		comRes.Proof,
		comRes.ProofHeight,
		c.MustGetAddress(),
	)

	return append(msgs, msg), nil
}

// MsgRelayAcknowledgement constructs the MsgAcknowledgement which is to be sent to the sending chain.
// The counterparty represents the receiving chain where the acknowledgement would be stored.
func (c *Chain) MsgRelayAcknowledgement(counterparty *Chain, packet *relayMsgPacketAck) (msgs []sdk.Msg, err error) {
	var ackRes *chantypes.QueryPacketAcknowledgementResponse
	if err = retry.Do(func() (err error) {
		// NOTE: the proof height uses - 1 due to tendermint's delayed execution model
		ackRes, err = counterparty.QueryPacketAcknowledgement(int64(counterparty.MustGetLatestLightHeight())-1, packet.seq)
		if err != nil {
			return err
		}

		if ackRes.Proof == nil || ackRes.Acknowledgement == nil {
			return fmt.Errorf("ack packet acknowledgement query seq(%d) is nil", packet.seq)
		}

		return nil
	}, rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, _ error) {
		// clear messages
		msgs = []sdk.Msg{}

		// OnRetry we want to update the light clients and then debug log
		updateMsg, err := c.UpdateClient(counterparty)
		if err != nil {
			return
		}

		msgs = append(msgs, updateMsg)

		if counterparty.debug {
			counterparty.Log(fmt.Sprintf("- [%s]@{%d} - try(%d/%d) query packet acknowledgement: %s",
				counterparty.ChainID, counterparty.MustGetLatestLightHeight()-1, n+1, rtyAttNum, err))
		}

	})); err != nil {
		counterparty.Error(err)
		return
	}

	if ackRes == nil {
		return nil, fmt.Errorf("ack packet [%s]seq{%d} has no associated proofs", counterparty.ChainID, packet.seq)
	}

	version := clienttypes.ParseChainID(counterparty.ChainID)
	msg := chantypes.NewMsgAcknowledgement(
		chantypes.NewPacket(
			packet.packetData,
			packet.seq,
			c.PathEnd.PortID,
			c.PathEnd.ChannelID,
			counterparty.PathEnd.PortID,
			counterparty.PathEnd.ChannelID,
			clienttypes.NewHeight(version, packet.timeout),
			packet.timeoutStamp,
		),
		packet.ack,
		ackRes.Proof,
		ackRes.ProofHeight,
		c.MustGetAddress(),
	)

	return append(msgs, msg), nil
}

// MsgRelayTimeout constructs the MsgTimeout which is to be sent to the sending chain.
// The counterparty represents the receiving chain where the receipts would have been
// stored.
func (c *Chain) MsgRelayTimeout(counterparty *Chain, packet *relayMsgTimeout) (msgs []sdk.Msg, err error) {
	var recvRes *chantypes.QueryPacketReceiptResponse
	if err = retry.Do(func() (err error) {
		// NOTE: Timeouts currently only work with ORDERED channels for nwo
		// NOTE: the proof height uses - 1 due to tendermint's delayed execution model
		recvRes, err = counterparty.QueryPacketReceipt(int64(counterparty.MustGetLatestLightHeight())-1, packet.seq)
		if err != nil {
			return err
		}

		if recvRes.Proof == nil {
			return fmt.Errorf("timeout packet receipt proof seq(%d) is nil", packet.seq)
		}

		return nil
	}, rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, _ error) {
		// clear messages
		msgs = []sdk.Msg{}

		// OnRetry we want to update the light clients and then debug log
		updateMsg, err := c.UpdateClient(counterparty)
		if err != nil {
			return
		}

		msgs = append(msgs, updateMsg)

		if counterparty.debug {
			counterparty.Log(fmt.Sprintf("- [%s]@{%d} - try(%d/%d) query packet receipt: %s",
				counterparty.ChainID, counterparty.MustGetLatestLightHeight()-1, n+1, rtyAttNum, err))
		}

	})); err != nil {
		counterparty.Error(err)
		return
	}

	if recvRes == nil {
		return nil, fmt.Errorf("timeout packet [%s]seq{%d} has no associated proofs", c.ChainID, packet.seq)
	}

	version := clienttypes.ParseChainID(counterparty.ChainID)
	msg := chantypes.NewMsgTimeout(
		chantypes.NewPacket(
			packet.packetData,
			packet.seq,
			c.PathEnd.PortID,
			c.PathEnd.ChannelID,
			counterparty.PathEnd.PortID,
			counterparty.PathEnd.ChannelID,
			clienttypes.NewHeight(version, packet.timeout),
			packet.timeoutStamp,
		),
		packet.seq,
		recvRes.Proof,
		recvRes.ProofHeight,
		c.MustGetAddress(),
	)

	return append(msgs, msg), nil
}
