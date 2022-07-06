package cosmos

import (
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

func (ccp *CosmosChainProcessor) handleMessage(m ibcMessage, ibcMessagesCache processor.IBCMessagesCache) {
	switch m.action {
	case processor.MsgTransfer, processor.MsgRecvPacket, processor.MsgAcknowledgement, processor.MsgTimeout, processor.MsgTimeoutOnClose:
		ccp.handlePacketMessage(m.action, m.info.(provider.PacketInfo), ibcMessagesCache)
	case processor.MsgCreateClient, processor.MsgUpdateClient, processor.MsgUpgradeClient, processor.MsgSubmitMisbehaviour:
		ccp.handleClientMessage(m.action, m.info.(clientInfo))
	}
}

func (ccp *CosmosChainProcessor) handlePacketMessage(action string, pi provider.PacketInfo, ibcMessagesCache processor.IBCMessagesCache) {
	channelKey := processor.PacketInfoChannelKey(action, pi)

	if !ibcMessagesCache.PacketFlow.ShouldRetainSequence(ccp.pathProcessors, channelKey, ccp.chainProvider.ChainId(), action, pi.Sequence) {
		ccp.log.Warn("Not retaining packet message",
			zap.String("action", action),
			zap.Uint64("sequence", pi.Sequence),
			zap.Any("channel", channelKey),
		)
		return
	}

	ibcMessagesCache.PacketFlow.Retain(channelKey, action, pi)
	ccp.logPacketMessage(processor.ShortAction(action), pi)
}

func (ccp *CosmosChainProcessor) handleClientMessage(action string, ci clientInfo) {
	ccp.latestClientState.updateLatestClientState(ci)
	ccp.logObservedIBCMessage(processor.ShortAction(action), zap.String("client_id", ci.clientID))
}

func (ccp *CosmosChainProcessor) logObservedIBCMessage(m string, fields ...zap.Field) {
	ccp.log.With(zap.String("message", m)).Debug("Observed IBC message", fields...)
}

func (ccp *CosmosChainProcessor) logPacketMessage(message string, pi provider.PacketInfo) {
	fields := []zap.Field{
		zap.Uint64("sequence", pi.Sequence),
		zap.String("src_channel", pi.SourceChannel),
		zap.String("src_port", pi.SourcePort),
		zap.String("dst_channel", pi.DestinationChannel),
		zap.String("dst_port", pi.DestinationPort),
	}
	if pi.TimeoutHeight.RevisionHeight > 0 {
		fields = append(fields, zap.Uint64("timeout_height", pi.TimeoutHeight.RevisionHeight))
	}
	if pi.TimeoutHeight.RevisionNumber > 0 {
		fields = append(fields, zap.Uint64("timeout_height_revision", pi.TimeoutHeight.RevisionNumber))
	}
	if pi.TimeoutTimestamp > 0 {
		fields = append(fields, zap.Uint64("timeout_timestamp", pi.TimeoutTimestamp))
	}
	ccp.logObservedIBCMessage(message, fields...)
}
