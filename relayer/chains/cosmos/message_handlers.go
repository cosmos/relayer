package cosmos

import (
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func (ccp *CosmosChainProcessor) handleMessage(m ibcMessage, c processor.IBCMessagesCache) {
	// These are the only actions we care about, not logging for non-IBC actions.
	switch m.action {
	case processor.MsgTransfer, processor.MsgRecvPacket, processor.MsgAcknowledgement, processor.MsgTimeout, processor.MsgTimeoutOnClose:
		ccp.handlePacketMessage(m.action, provider.PacketInfo(*m.info.(*packetInfo)), c)
	case processor.MsgCreateClient, processor.MsgUpdateClient, processor.MsgUpgradeClient, processor.MsgSubmitMisbehaviour:
		ccp.handleClientMessage(m.action, *m.info.(*clientInfo))
	}
}

func (ccp *CosmosChainProcessor) handlePacketMessage(action string, pi provider.PacketInfo, c processor.IBCMessagesCache) {
	channelKey, err := processor.PacketInfoChannelKey(action, pi)
	if err != nil {
		ccp.log.Error("Unexpected error handling packet message",
			zap.String("action", action),
			zap.Uint64("sequence", pi.Sequence),
			zap.Any("channel", channelKey),
			zap.Error(err),
		)
		return
	}

	if !c.PacketFlow.ShouldRetainSequence(ccp.pathProcessors, channelKey, ccp.chainProvider.ChainId(), action, pi.Sequence) {
		ccp.log.Warn("Not retaining packet message",
			zap.String("action", action),
			zap.Uint64("sequence", pi.Sequence),
			zap.Any("channel", channelKey),
		)
		return
	}

	c.PacketFlow.Retain(channelKey, action, pi)
	ccp.logPacketMessage(action, pi)
}

func (ccp *CosmosChainProcessor) handleClientMessage(action string, ci clientInfo) {
	ccp.latestClientState.update(ci)
	ccp.logObservedIBCMessage(action, zap.String("client_id", ci.clientID))
}

func (ccp *CosmosChainProcessor) logObservedIBCMessage(m string, fields ...zap.Field) {
	ccp.log.With(zap.Stringer("action", processor.ShortAction(m))).Debug("Observed IBC message", fields...)
}

func (ccp *CosmosChainProcessor) logPacketMessage(message string, pi provider.PacketInfo) {
	if !ccp.log.Core().Enabled(zapcore.DebugLevel) {
		return
	}
	fields := []zap.Field{
		zap.Uint64("sequence", pi.Sequence),
		zap.String("src_channel", pi.SourceChannel),
		zap.String("src_port", pi.SourcePort),
		zap.String("dst_channel", pi.DestChannel),
		zap.String("dst_port", pi.DestPort),
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
