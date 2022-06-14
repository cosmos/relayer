package cosmos

import (
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
		zap.Uint64("sequence", pi.packet.Sequence),
		zap.String("src_channel", pi.packet.SourceChannel),
		zap.String("src_port", pi.packet.SourcePort),
		zap.String("dst_channel", pi.packet.DestinationChannel),
		zap.String("dst_port", pi.packet.DestinationPort),
	}
	fields = append(fields, additionalFields...)
	ccp.logObservedIBCMessage(message, fields...)
}

// END packet msg handlers

// BEGIN client msg handlers

func (ccp *CosmosChainProcessor) handleMsgCreateClient(p MsgHandlerParams) bool {
	clientInfo := p.messageInfo.(*clientInfo)
	// save the latest consensus height and header for this client
	ccp.latestClientState.UpdateLatestClientState(*clientInfo)
	ccp.logObservedIBCMessage("MsgCreateClient", zap.String("client_id", clientInfo.clientID))
	return false
}

func (ccp *CosmosChainProcessor) handleMsgUpdateClient(p MsgHandlerParams) bool {
	clientInfo := p.messageInfo.(*clientInfo)
	// save the latest consensus height and header for this client
	ccp.latestClientState.UpdateLatestClientState(*clientInfo)
	ccp.logObservedIBCMessage("MsgUpdateClient", zap.String("client_id", clientInfo.clientID))
	return false
}

func (ccp *CosmosChainProcessor) handleMsgUpgradeClient(p MsgHandlerParams) bool {
	clientInfo := p.messageInfo.(*clientInfo)
	// save the latest consensus height and header for this client
	ccp.latestClientState.UpdateLatestClientState(*clientInfo)
	ccp.logObservedIBCMessage("MsgUpgradeClient", zap.String("client_id", clientInfo.clientID))
	return false
}

func (ccp *CosmosChainProcessor) handleMsgSubmitMisbehaviour(p MsgHandlerParams) bool {
	clientInfo := p.messageInfo.(*clientInfo)
	// save the latest consensus height and header for this client
	ccp.latestClientState.UpdateLatestClientState(*clientInfo)
	ccp.logObservedIBCMessage("MsgSubmitMisbehaviour", zap.String("client_id", clientInfo.clientID))
	return false
}

// END client msg handlers

func (ccp *CosmosChainProcessor) logObservedIBCMessage(m string, fields ...zap.Field) {
	ccp.log.With(zap.String("message", m)).Debug("Observed IBC message", fields...)
}
