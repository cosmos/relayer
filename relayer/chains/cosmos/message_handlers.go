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
	case processor.MsgTransfer, processor.MsgRecvPacket, processor.MsgAcknowledgement,
		processor.MsgTimeout, processor.MsgTimeoutOnClose:
		ccp.handlePacketMessage(m.action, provider.PacketInfo(*m.info.(*packetInfo)), c)
	case processor.MsgChannelOpenInit, processor.MsgChannelOpenTry,
		processor.MsgChannelOpenAck, processor.MsgChannelOpenConfirm:
		ccp.handleChannelMessage(m.action, provider.ChannelInfo(*m.info.(*channelInfo)), c)
	case processor.MsgConnectionOpenInit, processor.MsgConnectionOpenTry,
		processor.MsgConnectionOpenAck, processor.MsgConnectionOpenConfirm:
		ccp.handleConnectionMessage(m.action, provider.ConnectionInfo(*m.info.(*connectionInfo)), c)
	case processor.MsgCreateClient, processor.MsgUpdateClient, processor.MsgUpgradeClient, processor.MsgSubmitMisbehaviour:
		ccp.handleClientMessage(m.action, *m.info.(*clientInfo))
	}
}

func (ccp *CosmosChainProcessor) handlePacketMessage(
	action string,
	pi provider.PacketInfo,
	c processor.IBCMessagesCache,
) {
	k, err := processor.PacketInfoChannelKey(action, pi)
	if err != nil {
		ccp.log.Error("Unexpected error handling packet message",
			zap.String("action", action),
			zap.Uint64("sequence", pi.Sequence),
			zap.Inline(k),
			zap.Error(err),
		)
		return
	}

	if !c.PacketFlow.ShouldRetainSequence(ccp.pathProcessors, k, ccp.chainProvider.ChainId(), action, pi.Sequence) {
		ccp.log.Debug("Not retaining packet message",
			zap.String("action", action),
			zap.Uint64("sequence", pi.Sequence),
			zap.Inline(k),
		)
		return
	}

	ccp.log.Debug("Retaining packet message",
		zap.String("action", action),
		zap.Uint64("sequence", pi.Sequence),
		zap.Inline(k),
	)

	c.PacketFlow.Retain(k, action, pi)
	ccp.logPacketMessage(action, pi)
}

func (ccp *CosmosChainProcessor) handleChannelMessage(
	action string,
	ci provider.ChannelInfo,
	ibcMessagesCache processor.IBCMessagesCache,
) {
	ccp.channelConnections[ci.ChannelID] = ci.ConnID
	channelKey := processor.ChannelInfoChannelKey(ci)
	switch action {
	case processor.MsgChannelOpenInit, processor.MsgChannelOpenTry:
		ccp.channelStateCache[channelKey] = false
	case processor.MsgChannelOpenAck, processor.MsgChannelOpenConfirm:
		ccp.channelStateCache[channelKey] = true
	case processor.MsgChannelCloseInit, processor.MsgChannelCloseConfirm:
		for k := range ccp.channelStateCache {
			if k.PortID == ci.PortID && k.ChannelID == ci.ChannelID {
				ccp.channelStateCache[k] = false
				break
			}
		}
	}
	ibcMessagesCache.ChannelHandshake.Retain(channelKey, action, ci)

	ccp.logChannelMessage(action, ci)
}

func (ccp *CosmosChainProcessor) handleConnectionMessage(
	action string,
	ci provider.ConnectionInfo,
	ibcMessagesCache processor.IBCMessagesCache,
) {
	ccp.connectionClients[ci.ConnID] = ci.ClientID
	connectionKey := processor.ConnectionInfoConnectionKey(ci)
	open := (action == processor.MsgConnectionOpenAck || action == processor.MsgConnectionOpenConfirm)
	ccp.connectionStateCache[connectionKey] = open
	ibcMessagesCache.ConnectionHandshake.Retain(connectionKey, action, ci)

	ccp.logConnectionMessage(action, ci)
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

func (ccp *CosmosChainProcessor) logChannelMessage(message string, ci provider.ChannelInfo) {
	ccp.logObservedIBCMessage(message,
		zap.String("channel_id", ci.ChannelID),
		zap.String("port_id", ci.PortID),
		zap.String("counterparty_channel_id", ci.CounterpartyChannelID),
		zap.String("counterparty_port_id", ci.CounterpartyPortID),
		zap.String("connection_id", ci.ConnID),
	)
}

func (ccp *CosmosChainProcessor) logConnectionMessage(message string, ci provider.ConnectionInfo) {
	ccp.logObservedIBCMessage(message,
		zap.String("client_id", ci.ClientID),
		zap.String("connection_id", ci.ConnID),
		zap.String("counterparty_client_id", ci.CounterpartyClientID),
		zap.String("counterparty_connection_id", ci.CounterpartyConnID),
	)
}
