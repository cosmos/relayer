package cosmos

import (
	conntypes "github.com/cosmos/ibc-go/v3/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	"go.uber.org/zap"
)

type MsgHandlerParams struct {
	// incoming IBC message
	messageInfo interface{}

	// reference to the caches that will be assembled by the handlers in this file
	ibcMessagesCache processor.IBCMessagesCache
}

var messageHandlers = map[string]func(*CosmosChainProcessor, MsgHandlerParams) bool{
	processor.MsgTransfer:        (*CosmosChainProcessor).handleMsgTransfer,
	processor.MsgRecvPacket:      (*CosmosChainProcessor).handleMsgRecvPacket,
	processor.MsgAcknowledgement: (*CosmosChainProcessor).handleMsgAcknowledgement,
	processor.MsgTimeout:         (*CosmosChainProcessor).handleMsgTimeout,
	processor.MsgTimeoutOnClose:  (*CosmosChainProcessor).handleMsgTimeoutOnClose,

	processor.MsgCreateClient:       (*CosmosChainProcessor).handleMsgCreateClient,
	processor.MsgUpdateClient:       (*CosmosChainProcessor).handleMsgUpdateClient,
	processor.MsgUpgradeClient:      (*CosmosChainProcessor).handleMsgUpgradeClient,
	processor.MsgSubmitMisbehaviour: (*CosmosChainProcessor).handleMsgSubmitMisbehaviour,

	processor.MsgConnectionOpenInit:    (*CosmosChainProcessor).handleMsgConnectionOpenInit,
	processor.MsgConnectionOpenTry:     (*CosmosChainProcessor).handleMsgConnectionOpenTry,
	processor.MsgConnectionOpenAck:     (*CosmosChainProcessor).handleMsgConnectionOpenAck,
	processor.MsgConnectionOpenConfirm: (*CosmosChainProcessor).handleMsgConnectionOpenConfirm,

	processor.MsgChannelCloseConfirm: (*CosmosChainProcessor).handleMsgChannelCloseConfirm,
	processor.MsgChannelCloseInit:    (*CosmosChainProcessor).handleMsgChannelCloseInit,
	processor.MsgChannelOpenAck:      (*CosmosChainProcessor).handleMsgChannelOpenAck,
	processor.MsgChannelOpenConfirm:  (*CosmosChainProcessor).handleMsgChannelOpenConfirm,
	processor.MsgChannelOpenInit:     (*CosmosChainProcessor).handleMsgChannelOpenInit,
	processor.MsgChannelOpenTry:      (*CosmosChainProcessor).handleMsgChannelOpenTry,
}

// BEGIN packet msg handlers
func (ccp *CosmosChainProcessor) handleMsgTransfer(p MsgHandlerParams) bool {
	pi := p.messageInfo.(*packetInfo)
	// source chain processor will call this handler
	// source channel used as key because MsgTransfer is sent to source chain
	channelKey := pi.channelKey()
	if !p.ibcMessagesCache.PacketFlow.ShouldRetainSequence(ccp.pathProcessors, channelKey, ccp.chainProvider.ChainId(), processor.MsgTransfer, pi.packet.Sequence) {
		return false
	}
	// Construct the start of the MsgRecvPacket for the counterparty chain.
	// PathProcessor will determine if this is needed.
	// For example, if a MsgRecvPacket is not detected on the counterparty chain, and
	// a MsgAcknowledgement, MsgTimeout, or MsgTimeout is not detected yet on this chain,
	// and the packet timeout has not expired, a MsgRecvPacket will be sent to the counterparty chain
	// using this information with the packet commitment proof from this chain added.
	p.ibcMessagesCache.PacketFlow.Retain(channelKey, processor.MsgTransfer, pi.packet.Sequence, cosmos.NewCosmosMessage(&chantypes.MsgRecvPacket{
		Packet: chantypes.Packet{
			Sequence:           pi.packet.Sequence,
			SourcePort:         pi.packet.SourcePort,
			SourceChannel:      pi.packet.SourceChannel,
			DestinationPort:    pi.packet.DestinationPort,
			DestinationChannel: pi.packet.DestinationChannel,
			Data:               pi.packet.Data,
			TimeoutHeight:      pi.packet.TimeoutHeight,
			TimeoutTimestamp:   pi.packet.TimeoutTimestamp,
		},
	}))
	ccp.logPacketMessage("MsgTransfer", pi,
		zap.Uint64("timeout_height", pi.packet.TimeoutHeight.RevisionHeight),
		zap.Uint64("timeout_height_revision", pi.packet.TimeoutHeight.RevisionNumber),
		zap.Uint64("timeout_timestamp", pi.packet.TimeoutTimestamp),
	)
	return true
}

func (ccp *CosmosChainProcessor) handleMsgRecvPacket(p MsgHandlerParams) bool {
	pi := p.messageInfo.(*packetInfo)
	// destination chain processor will call this handler
	// destination channel used because MsgRecvPacket is sent to destination chain
	channelKey := pi.channelKey().Counterparty()
	if !p.ibcMessagesCache.PacketFlow.ShouldRetainSequence(ccp.pathProcessors, channelKey, ccp.chainProvider.ChainId(), processor.MsgRecvPacket, pi.packet.Sequence) {
		return false
	}
	// Construct the start of the MsgAcknowledgement for the counterparty chain.
	// PathProcessor will determine if this is needed.
	// For example, if a MsgAcknowledgement is not detected yet on the counterparty chain,
	// a MsgAcknowledgement will be sent to the counterparty chain
	// using this information with the packet acknowledgement commitment proof from this chain added.
	p.ibcMessagesCache.PacketFlow.Retain(channelKey, processor.MsgRecvPacket, pi.packet.Sequence, cosmos.NewCosmosMessage(&chantypes.MsgAcknowledgement{
		Packet: chantypes.Packet{
			Sequence:           pi.packet.Sequence,
			SourcePort:         pi.packet.SourcePort,
			SourceChannel:      pi.packet.SourceChannel,
			DestinationPort:    pi.packet.DestinationPort,
			DestinationChannel: pi.packet.DestinationChannel,
			Data:               pi.packet.Data,
			TimeoutHeight:      pi.packet.TimeoutHeight,
			TimeoutTimestamp:   pi.packet.TimeoutTimestamp,
		},
		Acknowledgement: pi.ack,
	}))
	ccp.logPacketMessage("MsgRecvPacket", pi)
	return true
}

func (ccp *CosmosChainProcessor) handleMsgAcknowledgement(p MsgHandlerParams) bool {
	pi := p.messageInfo.(*packetInfo)
	// source chain processor will call this handler
	// source channel used as key because MsgAcknowledgement is sent to source chain
	channelKey := pi.channelKey()
	if !p.ibcMessagesCache.PacketFlow.ShouldRetainSequence(ccp.pathProcessors, channelKey, ccp.chainProvider.ChainId(), processor.MsgAcknowledgement, pi.packet.Sequence) {
		return false
	}
	// Retaining a nil message here because this is for book-keeping in the PathProcessor cache only.
	// A message does not need to be constructed for the counterparty chain after the MsgAcknowledgement is observed,
	// but we want to tell the PathProcessor that the packet flow is complete for this sequence.
	p.ibcMessagesCache.PacketFlow.Retain(channelKey, processor.MsgAcknowledgement, pi.packet.Sequence, nil)
	ccp.logPacketMessage("MsgAcknowledgement", pi)
	return true
}

func (ccp *CosmosChainProcessor) handleMsgTimeout(p MsgHandlerParams) bool {
	pi := p.messageInfo.(*packetInfo)
	// source chain processor will call this handler
	// source channel used as key because MsgTimeout is sent to source chain
	channelKey := pi.channelKey()
	if !p.ibcMessagesCache.PacketFlow.ShouldRetainSequence(ccp.pathProcessors, channelKey, ccp.chainProvider.ChainId(), processor.MsgTimeout, pi.packet.Sequence) {
		return false
	}
	// Retaining a nil message here because this is for book-keeping in the PathProcessor cache only.
	// A message does not need to be constructed for the counterparty chain after the MsgTimeout is observed,
	// but we want to tell the PathProcessor that the packet flow is complete for this sequence.
	p.ibcMessagesCache.PacketFlow.Retain(channelKey, processor.MsgTimeout, pi.packet.Sequence, nil)
	ccp.logPacketMessage("MsgTimeout", pi)
	return true
}

func (ccp *CosmosChainProcessor) handleMsgTimeoutOnClose(p MsgHandlerParams) bool {
	pi := p.messageInfo.(*packetInfo)
	// source channel used because timeout is sent to source chain
	channelKey := pi.channelKey()
	if !p.ibcMessagesCache.PacketFlow.ShouldRetainSequence(ccp.pathProcessors, channelKey, ccp.chainProvider.ChainId(), processor.MsgTimeoutOnClose, pi.packet.Sequence) {
		return false
	}
	// Retaining a nil message here because this is for book-keeping in the PathProcessor cache only.
	// A message does not need to be constructed for the counterparty chain after the MsgTimeoutOnClose is observed,
	// but we want to tell the PathProcessor that the packet flow is complete for this sequence.
	p.ibcMessagesCache.PacketFlow.Retain(channelKey, processor.MsgTimeoutOnClose, pi.packet.Sequence, nil)
	ccp.logPacketMessage("MsgTimeoutOnClose", pi)
	return true
}

func (ccp *CosmosChainProcessor) logPacketMessage(message string, pi *packetInfo, additionalFields ...zap.Field) {
	fields := []zap.Field{
		zap.String("message", message),
		zap.Uint64("sequence", pi.packet.Sequence),
		zap.String("src_channel", pi.packet.SourceChannel),
		zap.String("src_port", pi.packet.SourcePort),
		zap.String("dst_channel", pi.packet.DestinationChannel),
		zap.String("dst_port", pi.packet.DestinationPort),
	}
	fields = append(fields, additionalFields...)
	ccp.log.Debug("Observed IBC message", fields...)
}

// END packet msg handlers

// BEGIN client msg handlers

func (ccp *CosmosChainProcessor) handleMsgCreateClient(p MsgHandlerParams) bool {
	clientInfo := p.messageInfo.(*clientInfo)
	// save the latest consensus height and header for this client
	ccp.latestClientState.UpdateLatestClientState(*clientInfo)
	ccp.logClientMessage("MsgCreateClient", clientInfo)
	return true
}

func (ccp *CosmosChainProcessor) handleMsgUpdateClient(p MsgHandlerParams) bool {
	clientInfo := p.messageInfo.(*clientInfo)
	// save the latest consensus height and header for this client
	ccp.latestClientState.UpdateLatestClientState(*clientInfo)
	ccp.logClientMessage("MsgUpdateClient", clientInfo)
	return true
}

func (ccp *CosmosChainProcessor) handleMsgUpgradeClient(p MsgHandlerParams) bool {
	clientInfo := p.messageInfo.(*clientInfo)
	// save the latest consensus height and header for this client
	ccp.latestClientState.UpdateLatestClientState(*clientInfo)
	ccp.logClientMessage("MsgUpgradeClient", clientInfo)
	return true
}

func (ccp *CosmosChainProcessor) handleMsgSubmitMisbehaviour(p MsgHandlerParams) bool {
	clientInfo := p.messageInfo.(*clientInfo)
	// save the latest consensus height and header for this client
	ccp.latestClientState.UpdateLatestClientState(*clientInfo)
	ccp.logClientMessage("MsgSubmitMisbehaviour", clientInfo)
	return true
}

func (ccp *CosmosChainProcessor) logClientMessage(message string, clientInfo *clientInfo) bool {
	ccp.log.Debug("observed ibc message",
		zap.String("message", message),
		zap.String("client_id", clientInfo.clientID),
	)
	return true
}

// END client msg handlers

// BEGIN connection msg handlers

func (ccp *CosmosChainProcessor) handleMsgConnectionOpenInit(p MsgHandlerParams) bool {
	ci := p.messageInfo.(*connectionInfo)
	k := ci.connectionKey()
	// Construct the start of the MsgConnectionOpenTry for the counterparty chain.
	// PathProcessor will determine if this is needed.
	// For example, if a MsgConnectionOpenTry or MsgConnectionOpenConfirm is not detected on the counterparty chain, and
	// a MsgConnectionOpenAck is not detected yet on this chain,
	// a MsgConnectionOpenTry will be sent to the counterparty chain
	// using this information with the connection init proof from this chain added.
	p.ibcMessagesCache.ConnectionHandshake.Retain(k, processor.MsgConnectionOpenInit, cosmos.NewCosmosMessage(&conntypes.MsgConnectionOpenTry{
		ClientId:             k.ClientID,
		PreviousConnectionId: k.ConnectionID,
		Counterparty: conntypes.Counterparty{
			ClientId:     k.CounterpartyClientID,
			ConnectionId: k.CounterpartyConnectionID,
		},
	}))
	ccp.connectionStateCache[k] = false
	ccp.logConnectionMessage("MsgConnectionOpenInit", ci)
	return true
}

func (ccp *CosmosChainProcessor) handleMsgConnectionOpenTry(p MsgHandlerParams) bool {
	ci := p.messageInfo.(*connectionInfo)
	k := ci.connectionKey().Counterparty()
	// Construct the start of the MsgConnectionOpenAck for the counterparty chain.
	// PathProcessor will determine if this is needed.
	// For example, if a MsgConnectionOpenAck is not detected on the counterparty chain, and
	// a MsgConnectionOpenConfirm is not detected yet on this chain,
	// a MsgConnectionOpenAck will be sent to the counterparty chain
	// using this information with the connection try proof from this chain added.
	p.ibcMessagesCache.ConnectionHandshake.Retain(k, processor.MsgConnectionOpenTry, cosmos.NewCosmosMessage(&conntypes.MsgConnectionOpenAck{
		ConnectionId:             k.ConnectionID,
		CounterpartyConnectionId: k.CounterpartyConnectionID,
	}))
	ccp.connectionStateCache[k] = false
	ccp.logConnectionMessage("MsgConnectionOpenTry", ci)
	return true
}

func (ccp *CosmosChainProcessor) handleMsgConnectionOpenAck(p MsgHandlerParams) bool {
	ci := p.messageInfo.(*connectionInfo)
	k := ci.connectionKey()
	// Construct the start of the MsgConnectionOpenConfirm for the counterparty chain.
	// PathProcessor will determine if this is needed.
	// For example, if a MsgConnectionOpenConfirm is not detected on the counterparty chain,
	// a MsgConnectionOpenConfirm will be sent to the counterparty chain
	// using this information with the connection ack proof from this chain added.
	p.ibcMessagesCache.ConnectionHandshake.Retain(k, processor.MsgConnectionOpenAck, cosmos.NewCosmosMessage(&conntypes.MsgConnectionOpenConfirm{
		ConnectionId: k.ConnectionID,
	}))
	ccp.connectionStateCache[k] = true
	ccp.logConnectionMessage("MsgConnectionOpenAck", ci)
	return true
}

func (ccp *CosmosChainProcessor) handleMsgConnectionOpenConfirm(p MsgHandlerParams) bool {
	ci := p.messageInfo.(*connectionInfo)
	k := ci.connectionKey().Counterparty()
	// Retaining a nil message here because this is for book-keeping in the PathProcessor cache only.
	// A message does not need to be constructed for the counterparty chain after the MsgConnectionOpenConfirm is observed,
	// but we want to tell the PathProcessor that the connection handshake is complete for this sequence.
	p.ibcMessagesCache.ConnectionHandshake.Retain(k, processor.MsgConnectionOpenConfirm, nil)
	ccp.connectionStateCache[k] = true
	ccp.logConnectionMessage("MsgConnectionOpenConfirm", ci)
	return true
}

func (ccp *CosmosChainProcessor) logConnectionMessage(message string, connectionInfo *connectionInfo) {
	ccp.log.Debug("Observed IBC message",
		zap.String("message", message),
		zap.String("client_id", connectionInfo.clientID),
		zap.String("connection_id", connectionInfo.connectionID),
		zap.String("counterparty_client_id", connectionInfo.counterpartyClientID),
		zap.String("counterparty_connection_id", connectionInfo.counterpartyConnectionID),
	)
}

// END connection msg handlers

// BEGIN channel msg handlers

func (ccp *CosmosChainProcessor) handleMsgChannelOpenInit(p MsgHandlerParams) bool {
	ci := p.messageInfo.(*channelInfo)
	k := ci.channelKey()
	// Construct the start of the MsgChannelOpenTry for the counterparty chain.
	// PathProcessor will determine if this is needed.
	// For example, if a MsgChannelOpenTry or MsgChannelOpenConfirm is not detected on the counterparty chain, and
	// a MsgChannelOpenAck is not detected yet on this chain,
	// a MsgChannelOpenTry will be sent to the counterparty chain
	// using this information with the channel open init proof from this chain added.
	// Sending false for open because channel is not open until MsgChannelOpenAck on this chain
	p.ibcMessagesCache.ChannelHandshake.Retain(k, processor.MsgChannelOpenInit, cosmos.NewCosmosMessage(&chantypes.MsgChannelOpenTry{
		PortId:            k.PortID,
		PreviousChannelId: k.ChannelID,
		Channel: chantypes.Channel{
			Counterparty: chantypes.Counterparty{
				PortId:    k.CounterpartyPortID,
				ChannelId: k.CounterpartyChannelID,
			},
			ConnectionHops: []string{ci.connectionID},
		},
	}))
	ccp.channelStateCache[k] = false
	ccp.logChannelMessage("MsgChannelOpenInit", ci)
	return true
}

func (ccp *CosmosChainProcessor) handleMsgChannelOpenTry(p MsgHandlerParams) bool {
	ci := p.messageInfo.(*channelInfo)
	// using flipped counterparty since counterparty initialized this handshake
	k := ci.channelKey().Counterparty()
	// Construct the start of the MsgChannelOpenAck for the counterparty chain.
	// PathProcessor will determine if this is needed.
	// For example, if a MsgChannelOpenAck is not detected on the counterparty chain, and
	// a MsgChannelOpenConfirm is not detected yet on this chain,
	// a MsgChannelOpenAck will be sent to the counterparty chain
	// using this information with the channel open try proof from this chain added.
	// Sending false for open because channel is not open until MsgChannelOpenConfirm on this chain
	p.ibcMessagesCache.ChannelHandshake.Retain(k, processor.MsgChannelOpenTry, cosmos.NewCosmosMessage(&chantypes.MsgChannelOpenAck{
		PortId:                k.PortID,
		ChannelId:             k.ChannelID,
		CounterpartyChannelId: k.CounterpartyChannelID,
	}))
	ccp.channelStateCache[k] = false
	ccp.logChannelMessage("MsgChannelOpenTry", ci)
	return true
}

func (ccp *CosmosChainProcessor) handleMsgChannelOpenAck(p MsgHandlerParams) bool {
	ci := p.messageInfo.(*channelInfo)
	k := ci.channelKey()
	// Construct the start of the MsgChannelOpenConfirm for the counterparty chain.
	// PathProcessor will determine if this is needed.
	// For example, if a MsgChannelOpenConfirm is not detected on the counterparty chain,
	// a MsgChannelOpenConfirm will be sent to the counterparty chain
	// using this information with the channel open ack proof from this chain added.
	p.ibcMessagesCache.ChannelHandshake.Retain(k, processor.MsgChannelOpenAck, cosmos.NewCosmosMessage(&chantypes.MsgChannelOpenConfirm{
		PortId:    k.PortID,
		ChannelId: k.ChannelID,
	}))
	ccp.channelStateCache[k] = true
	ccp.logChannelMessage("MsgChannelOpenAck", ci)
	return true
}

func (ccp *CosmosChainProcessor) handleMsgChannelOpenConfirm(p MsgHandlerParams) bool {
	ci := p.messageInfo.(*channelInfo)
	// using flipped counterparty since counterparty initialized this handshake
	k := ci.channelKey().Counterparty()
	// Retaining a nil message here because this is for book-keeping in the PathProcessor cache only.
	// A message does not need to be constructed for the counterparty chain after the MsgChannelOpenConfirm is observed,
	// but we want to tell the PathProcessor that the channel handshake is complete for this channel.
	p.ibcMessagesCache.ChannelHandshake.Retain(k, processor.MsgChannelOpenConfirm, nil)
	ccp.channelStateCache[k] = true
	ccp.logChannelMessage("MsgChannelOpenConfirm", ci)
	return true
}

func (ccp *CosmosChainProcessor) handleMsgChannelCloseInit(p MsgHandlerParams) bool {
	ci := p.messageInfo.(*channelInfo)
	k := ci.channelKey()
	// Construct the start of the MsgChannelCloseConfirm for the counterparty chain.
	// PathProcessor will determine if this is needed.
	// For example, if a MsgChannelCloseConfirm is not detected on the counterparty chain,
	// a MsgChannelCloseConfirm will be sent to the counterparty chain
	// using this information with the channel close init proof from this chain added.
	p.ibcMessagesCache.ChannelHandshake.Retain(k, processor.MsgChannelCloseInit, cosmos.NewCosmosMessage(&chantypes.MsgChannelCloseConfirm{
		PortId:    k.PortID,
		ChannelId: k.ChannelID,
	}))
	ccp.channelStateCache[k] = false
	ccp.logChannelMessage("MsgChannelCloseInit", ci)
	return true
}

func (ccp *CosmosChainProcessor) handleMsgChannelCloseConfirm(p MsgHandlerParams) bool {
	ci := p.messageInfo.(*channelInfo)
	// using flipped counterparty since counterparty initialized this channel close
	k := ci.channelKey().Counterparty()
	// Retaining a nil message here because this is for book-keeping in the PathProcessor cache only.
	// A message does not need to be constructed for the counterparty chain after the MsgChannelCloseConfirm is observed,
	// but we want to tell the PathProcessor that the channel close is complete for this channel.
	p.ibcMessagesCache.ChannelHandshake.Retain(k, processor.MsgChannelCloseConfirm, nil)
	ccp.channelStateCache[k] = false
	ccp.logChannelMessage("MsgChannelCloseConfirm", ci)
	return true
}

func (ccp *CosmosChainProcessor) logChannelMessage(message string, channelInfo *channelInfo) {
	ccp.log.Debug("Observed IBC message",
		zap.String("message", message),
		zap.String("channel_id", channelInfo.channelID),
		zap.String("port_id", channelInfo.portID),
		zap.String("counterparty_channel_id", channelInfo.counterpartyChannelID),
		zap.String("counterparty_port_id", channelInfo.counterpartyPortID),
		zap.String("connection_id", channelInfo.connectionID),
	)
}

// END channel msg handlers
