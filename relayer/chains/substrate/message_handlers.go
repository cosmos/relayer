package substrate

import (
	conntypes "github.com/cosmos/ibc-go/v5/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func (scp *SubstrateChainProcessor) logObservedIBCMessage(m string, fields ...zap.Field) {
	scp.log.With(zap.String("event_type", m)).Debug("Observed IBC message", fields...)
}

func (scp *SubstrateChainProcessor) handleClientMessage(eventType string, ci clientInfo) {
	scp.latestClientState.update(ci)
	scp.logObservedIBCMessage(eventType, zap.String("client_id", ci.ClientID))
}

func (scp *SubstrateChainProcessor) logPacketMessage(message string, pi provider.PacketInfo) {
	if !scp.log.Core().Enabled(zapcore.DebugLevel) {
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
	scp.logObservedIBCMessage(message, fields...)
}

func (scp *SubstrateChainProcessor) logChannelMessage(message string, ci provider.ChannelInfo) {
	scp.logObservedIBCMessage(message,
		zap.String("channel_id", ci.ChannelID),
		zap.String("port_id", ci.PortID),
		zap.String("counterparty_channel_id", ci.CounterpartyChannelID),
		zap.String("counterparty_port_id", ci.CounterpartyPortID),
		zap.String("connection_id", ci.ConnID),
	)
}

func (scp *SubstrateChainProcessor) logConnectionMessage(message string, ci provider.ConnectionInfo) {
	scp.logObservedIBCMessage(message,
		zap.String("client_id", ci.ClientID),
		zap.String("connection_id", ci.ConnID),
		zap.String("counterparty_client_id", ci.CounterpartyClientID),
		zap.String("counterparty_connection_id", ci.CounterpartyConnID),
	)
}

func (scp *SubstrateChainProcessor) handlePacketMessage(
	eventType string,
	pi provider.PacketInfo,
	c processor.IBCMessagesCache,
) {
	k, err := processor.PacketInfoChannelKey(eventType, pi)
	if err != nil {
		scp.log.Error("Unexpected error handling packet message",
			zap.String("event_type", eventType),
			zap.Uint64("sequence", pi.Sequence),
			zap.Inline(k),
			zap.Error(err),
		)
		return
	}

	if !c.PacketFlow.ShouldRetainSequence(scp.pathProcessors, k, scp.chainProvider.ChainId(), eventType, pi.Sequence) {
		scp.log.Debug("Not retaining packet message",
			zap.String("event_type", eventType),
			zap.Uint64("sequence", pi.Sequence),
			zap.Inline(k),
		)
		return
	}

	scp.log.Debug("Retaining packet message",
		zap.String("event_type", eventType),
		zap.Uint64("sequence", pi.Sequence),
		zap.Inline(k),
	)

	c.PacketFlow.Retain(k, eventType, pi)
	scp.logPacketMessage(eventType, pi)
}

func (scp *SubstrateChainProcessor) handleChannelMessage(
	eventType string,
	ci provider.ChannelInfo,
	messagesCache processor.IBCMessagesCache,
) {
	scp.channelConnections[ci.ChannelID] = ci.ConnID
	channelKey := processor.ChannelInfoChannelKey(ci)
	switch eventType {
	case chantypes.EventTypeChannelOpenInit, chantypes.EventTypeChannelOpenTry:
		scp.channelStateCache[channelKey] = false
	case chantypes.EventTypeChannelOpenAck, chantypes.EventTypeChannelOpenConfirm:
		scp.channelStateCache[channelKey] = true
	case chantypes.EventTypeChannelCloseConfirm:
		for k := range scp.channelStateCache {
			if k.PortID == ci.PortID && k.ChannelID == ci.ChannelID {
				scp.channelStateCache[k] = false
				break
			}
		}
	}
	messagesCache.ChannelHandshake.Retain(channelKey, eventType, ci)

	scp.logChannelMessage(eventType, ci)
}

func (scp *SubstrateChainProcessor) handleConnectionMessage(
	eventType string,
	ci provider.ConnectionInfo,
	messagesCache processor.IBCMessagesCache,
) {
	scp.connectionClients[ci.ConnID] = ci.ClientID
	connectionKey := processor.ConnectionInfoConnectionKey(ci)
	open := (eventType == conntypes.EventTypeConnectionOpenAck || eventType == conntypes.EventTypeConnectionOpenConfirm)
	scp.connectionStateCache[connectionKey] = open
	messagesCache.ConnectionHandshake.Retain(connectionKey, eventType, ci)

	scp.logConnectionMessage(eventType, ci)
}

func (scp *SubstrateChainProcessor) handleMessage(m ibcMessage, c processor.IBCMessagesCache) {
	switch t := m.info.(type) {
	case *clientInfo:
		scp.handleClientMessage(m.eventType, *t)
	case *packetInfo:
		scp.handlePacketMessage(m.eventType, provider.PacketInfo(*t), c)
	case *channelInfo:
		scp.handleChannelMessage(m.eventType, provider.ChannelInfo(*t), c)
	case *connectionInfo:
		scp.handleConnectionMessage(m.eventType, provider.ConnectionInfo(*t), c)
	}
}
