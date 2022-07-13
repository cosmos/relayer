package cosmos

import (
	conntypes "github.com/cosmos/ibc-go/v3/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func (ccp *PenumbraChainProcessor) handleMessage(m ibcMessage, c processor.IBCMessagesCache) {
	switch t := m.info.(type) {
	case *packetInfo:
		ccp.handlePacketMessage(m.eventType, provider.PacketInfo(*t), c)
	case *channelInfo:
		ccp.handleChannelMessage(m.eventType, provider.ChannelInfo(*t), c)
	case *connectionInfo:
		ccp.handleConnectionMessage(m.eventType, provider.ConnectionInfo(*t), c)
	case *clientInfo:
		ccp.handleClientMessage(m.eventType, *t)
	}
}

func (ccp *PenumbraChainProcessor) handlePacketMessage(action string, pi provider.PacketInfo, c processor.IBCMessagesCache) {
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

func (ccp *PenumbraChainProcessor) handleChannelMessage(eventType string, ci provider.ChannelInfo, ibcMessagesCache processor.IBCMessagesCache) {
	ccp.channelConnections[ci.ChannelID] = ci.ConnID
	channelKey := processor.ChannelInfoChannelKey(ci)
	switch eventType {
	case chantypes.EventTypeChannelOpenInit, chantypes.EventTypeChannelOpenTry:
		ccp.channelStateCache[channelKey] = false
	case chantypes.EventTypeChannelOpenAck, chantypes.EventTypeChannelOpenConfirm:
		ccp.channelStateCache[channelKey] = true
	case chantypes.EventTypeChannelCloseInit, chantypes.EventTypeChannelCloseConfirm:
		for k := range ccp.channelStateCache {
			if k.PortID == ci.PortID && k.ChannelID == ci.ChannelID {
				ccp.channelStateCache[k] = false
				break
			}
		}
	}
	ibcMessagesCache.ChannelHandshake.Retain(channelKey, eventType, ci)

	ccp.logChannelMessage(eventType, ci)
}

func (ccp *PenumbraChainProcessor) handleConnectionMessage(eventType string, ci provider.ConnectionInfo, ibcMessagesCache processor.IBCMessagesCache) {
	ccp.connectionClients[ci.ConnID] = ci.ClientID
	connectionKey := processor.ConnectionInfoConnectionKey(ci)
	open := (eventType == conntypes.EventTypeConnectionOpenAck || eventType == conntypes.EventTypeConnectionOpenConfirm)
	ccp.connectionStateCache[connectionKey] = open
	ibcMessagesCache.ConnectionHandshake.Retain(connectionKey, eventType, ci)

	ccp.logConnectionMessage(eventType, ci)
}

func (ccp *PenumbraChainProcessor) handleClientMessage(eventType string, ci clientInfo) {
	ccp.latestClientState.update(ci)
	ccp.logObservedIBCMessage(eventType, zap.String("client_id", ci.clientID))
}

func (ccp *PenumbraChainProcessor) logObservedIBCMessage(m string, fields ...zap.Field) {
	ccp.log.With(zap.String("event_type", m)).Debug("Observed IBC message", fields...)
}

func (ccp *PenumbraChainProcessor) logPacketMessage(message string, pi provider.PacketInfo) {
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

func (ccp *PenumbraChainProcessor) logChannelMessage(message string, ci provider.ChannelInfo) {
	ccp.logObservedIBCMessage(message,
		zap.String("channel_id", ci.ChannelID),
		zap.String("port_id", ci.PortID),
		zap.String("counterparty_channel_id", ci.CounterpartyChannelID),
		zap.String("counterparty_port_id", ci.CounterpartyPortID),
		zap.String("connection_id", ci.ConnID),
	)
}

func (ccp *PenumbraChainProcessor) logConnectionMessage(message string, ci provider.ConnectionInfo) {
	ccp.logObservedIBCMessage(message,
		zap.String("client_id", ci.ClientID),
		zap.String("connection_id", ci.ConnID),
		zap.String("counterparty_client_id", ci.CounterpartyClientID),
		zap.String("counterparty_connection_id", ci.CounterpartyConnID),
	)
}
