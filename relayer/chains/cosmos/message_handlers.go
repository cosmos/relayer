package cosmos

import (
	"fmt"

	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	"go.uber.org/zap"
)

type MsgHandlerParams struct {
	messageInfo   interface{}
	foundMessages processor.ChannelMessageCache
}

var messageHandlers = map[string]func(*CosmosChainProcessor, MsgHandlerParams) bool{
	processor.MsgTransfer:        (*CosmosChainProcessor).handleMsgTransfer,
	processor.MsgRecvPacket:      (*CosmosChainProcessor).handleMsgRecvPacket,
	processor.MsgAcknowledgement: (*CosmosChainProcessor).handleMsgAcknowlegement,
	processor.MsgTimeout:         (*CosmosChainProcessor).handleMsgTimeout,
	processor.MsgTimeoutOnClose:  (*CosmosChainProcessor).handleMsgTimeoutOnClose,

	// TODO client, connection, channel messages
}

// BEGIN packet msg handlers
func (ccp *CosmosChainProcessor) handleMsgTransfer(p MsgHandlerParams) bool {
	packetInfo := p.messageInfo.(*packetInfo)
	// source chain processor will call this handler
	// source channel used as key because MsgTransfer is sent to source chain
	channelKey := packetInfo.channelKey()
	if !p.foundMessages.ShouldRetainSequence(ccp.pathProcessors, channelKey, ccp.chainProvider.ChainId(), processor.MsgTransfer, packetInfo.packet.Sequence) {
		return false
	}
	// Construct the start of the MsgRecvPacket for the counterparty chain.
	// PathProcessor will determine if this is needed.
	// For example, if a MsgRecvPacket is not detected on the counterparty chain, and
	// a MsgAcknowledgement, MsgTimeout, or MsgTimeout is not detected yet on this chain,
	// and the packet timeout has not expired, a MsgRecvPacket will be sent to the counterparty chain
	// using this information with the packet commitment proof from this chain added.
	p.foundMessages.Retain(channelKey, processor.MsgTransfer, packetInfo.packet.Sequence, cosmos.NewCosmosMessage(&chantypes.MsgRecvPacket{
		Packet: chantypes.Packet{
			Sequence:           packetInfo.packet.Sequence,
			SourcePort:         packetInfo.packet.SourcePort,
			SourceChannel:      packetInfo.packet.SourceChannel,
			DestinationPort:    packetInfo.packet.DestinationPort,
			DestinationChannel: packetInfo.packet.DestinationChannel,
			Data:               packetInfo.packet.Data,
			TimeoutHeight:      packetInfo.packet.TimeoutHeight,
			TimeoutTimestamp:   packetInfo.packet.TimeoutTimestamp,
		},
	}))
	ccp.logPacketMessage("MsgTransfer", packetInfo,
		zap.String("timeout_height", fmt.Sprintf("%d-%d", packetInfo.packet.TimeoutHeight.RevisionNumber, packetInfo.packet.TimeoutHeight.RevisionHeight)),
		zap.Uint64("timeout_timestamp", packetInfo.packet.TimeoutTimestamp),
	)
	return true
}

func (ccp *CosmosChainProcessor) handleMsgRecvPacket(p MsgHandlerParams) bool {
	packetInfo := p.messageInfo.(*packetInfo)
	// destination chain processor will call this handler
	// destination channel used because MsgRecvPacket is sent to destination chain
	channelKey := packetInfo.channelKey().Counterparty()
	if !p.foundMessages.ShouldRetainSequence(ccp.pathProcessors, channelKey, ccp.chainProvider.ChainId(), processor.MsgRecvPacket, packetInfo.packet.Sequence) {
		return false
	}
	// Construct the start of the MsgAcknowledgement for the counterparty chain.
	// PathProcessor will determine if this is needed.
	// For example, if a MsgAcknowledgement is not detected yet on the counterparty chain,
	// a MsgAcknowledgement will be sent to the counterparty chain
	// using this information with the packet acknowledgement commitment proof from this chain added.
	p.foundMessages.Retain(channelKey, processor.MsgRecvPacket, packetInfo.packet.Sequence, cosmos.NewCosmosMessage(&chantypes.MsgAcknowledgement{
		Packet: chantypes.Packet{
			Sequence:           packetInfo.packet.Sequence,
			SourcePort:         packetInfo.packet.SourcePort,
			SourceChannel:      packetInfo.packet.SourceChannel,
			DestinationPort:    packetInfo.packet.DestinationPort,
			DestinationChannel: packetInfo.packet.DestinationChannel,
			Data:               packetInfo.packet.Data,
			TimeoutHeight:      packetInfo.packet.TimeoutHeight,
			TimeoutTimestamp:   packetInfo.packet.TimeoutTimestamp,
		},
		Acknowledgement: packetInfo.ack,
	}))
	ccp.logPacketMessage("MsgRecvPacket", packetInfo)
	return true
}

func (ccp *CosmosChainProcessor) handleMsgAcknowlegement(p MsgHandlerParams) bool {
	packetInfo := p.messageInfo.(*packetInfo)
	// source chain processor will call this handler
	// source channel used as key because MsgAcknowlegement is sent to source chain
	channelKey := packetInfo.channelKey()
	if !p.foundMessages.ShouldRetainSequence(ccp.pathProcessors, channelKey, ccp.chainProvider.ChainId(), processor.MsgAcknowledgement, packetInfo.packet.Sequence) {
		return false
	}
	// Retaining a nil message here because this is for book-keeping in the PathProcessor cache only.
	// A message does not need to be constructed for the counterparty chain after the MsgAcknowledgement is observed,
	// but we want to tell the PathProcessor that the packet flow is complete for this sequence.
	p.foundMessages.Retain(channelKey, processor.MsgAcknowledgement, packetInfo.packet.Sequence, nil)
	ccp.logPacketMessage("MsgAcknowledgement", packetInfo)
	return true
}

func (ccp *CosmosChainProcessor) handleMsgTimeout(p MsgHandlerParams) bool {
	packetInfo := p.messageInfo.(*packetInfo)
	// source chain processor will call this handler
	// source channel used as key because MsgTimeout is sent to source chain
	channelKey := packetInfo.channelKey()
	if !p.foundMessages.ShouldRetainSequence(ccp.pathProcessors, channelKey, ccp.chainProvider.ChainId(), processor.MsgTimeout, packetInfo.packet.Sequence) {
		return false
	}
	// Retaining a nil message here because this is for book-keeping in the PathProcessor cache only.
	// A message does not need to be constructed for the counterparty chain after the MsgTimeout is observed,
	// but we want to tell the PathProcessor that the packet flow is complete for this sequence.
	p.foundMessages.Retain(channelKey, processor.MsgTimeout, packetInfo.packet.Sequence, nil)
	ccp.logPacketMessage("MsgTimeout", packetInfo)
	return true
}

func (ccp *CosmosChainProcessor) handleMsgTimeoutOnClose(p MsgHandlerParams) bool {
	packetInfo := p.messageInfo.(*packetInfo)
	// source channel used because timeout is sent to source chain
	channelKey := packetInfo.channelKey()
	if !p.foundMessages.ShouldRetainSequence(ccp.pathProcessors, channelKey, ccp.chainProvider.ChainId(), processor.MsgTimeoutOnClose, packetInfo.packet.Sequence) {
		return false
	}
	// Retaining a nil message here because this is for book-keeping in the PathProcessor cache only.
	// A message does not need to be constructed for the counterparty chain after the MsgTimeoutOnClose is observed,
	// but we want to tell the PathProcessor that the packet flow is complete for this sequence.
	p.foundMessages.Retain(channelKey, processor.MsgTimeoutOnClose, packetInfo.packet.Sequence, nil)
	ccp.logPacketMessage("MsgTimeoutOnClose", packetInfo)
	return true
}

func (ccp *CosmosChainProcessor) logPacketMessage(message string, packetInfo *packetInfo, additionalFields ...zap.Field) {
	fields := []zap.Field{
		zap.String("message", message),
		zap.Uint64("sequence", packetInfo.packet.Sequence),
		zap.String("src_channel", packetInfo.packet.SourceChannel),
		zap.String("src_port", packetInfo.packet.SourcePort),
		zap.String("dst_channel", packetInfo.packet.DestinationChannel),
		zap.String("dst_port", packetInfo.packet.DestinationPort),
	}
	fields = append(fields, additionalFields...)
	ccp.log.Debug("observed ibc message", fields...)
}

// END packet msg handlers
