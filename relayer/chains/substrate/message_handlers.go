package substrate

import (
	conntypes "github.com/cosmos/ibc-go/v5/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func (ccp *SubstrateChainProcessor) handleMessage(m ibcMessage, c processor.IBCMessagesCache) {
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

func (ccp *SubstrateChainProcessor) handlePacketMessage(eventType string, pi provider.PacketInfo, c processor.IBCMessagesCache) {
	k, err := processor.PacketInfoChannelKey(eventType, pi)
	if err != nil {
		ccp.log.Error("Unexpected error handling packet message",
			zap.String("event_type", eventType),
			zap.Uint64("sequence", pi.Sequence),
			zap.Inline(k),
			zap.Error(err),
		)
		return
	}

	if eventType == chantypes.EventTypeRecvPacket && len(pi.Ack) == 0 {
		// ignore recv packet with empty ack bytes
		return
	}

	if !c.PacketFlow.ShouldRetainSequence(ccp.pathProcessors, k, ccp.chainProvider.ChainId(), eventType, pi.Sequence) {
		ccp.log.Debug("Not retaining packet message",
			zap.String("event_type", eventType),
			zap.Uint64("sequence", pi.Sequence),
			zap.Inline(k),
		)
		return
	}

	ccp.log.Debug("Retaining packet message",
		zap.String("event_type", eventType),
		zap.Uint64("sequence", pi.Sequence),
		zap.Inline(k),
	)

	c.PacketFlow.Retain(k, eventType, pi)
	ccp.logPacketMessage(eventType, pi)
}

func (ccp *SubstrateChainProcessor) handleChannelMessage(eventType string, ci provider.ChannelInfo, ibcMessagesCache processor.IBCMessagesCache) {
	ccp.channelConnections[ci.ChannelID] = ci.ConnID
	channelKey := processor.ChannelInfoChannelKey(ci)

	if eventType == chantypes.EventTypeChannelOpenInit {
		found := false
		for k := range ccp.channelStateCache {
			// Don't add a channelKey to the channelStateCache without counterparty channel ID
			// since we already have the channelKey in the channelStateCache which includes the
			// counterparty channel ID.
			if k.MsgInitKey() == channelKey {
				found = true
				break
			}
		}
		if !found {
			ccp.channelStateCache[channelKey] = false
		}
	} else {
		switch eventType {
		case chantypes.EventTypeChannelOpenTry:
			ccp.channelStateCache[channelKey] = false
		case chantypes.EventTypeChannelOpenAck, chantypes.EventTypeChannelOpenConfirm:
			ccp.channelStateCache[channelKey] = true
		case chantypes.EventTypeChannelCloseConfirm:
			for k := range ccp.channelStateCache {
				if k.PortID == ci.PortID && k.ChannelID == ci.ChannelID {
					ccp.channelStateCache[k] = false
					break
				}
			}
		}
		// Clear out MsgInitKeys once we have the counterparty channel ID
		delete(ccp.channelStateCache, channelKey.MsgInitKey())
	}

	ibcMessagesCache.ChannelHandshake.Retain(channelKey, eventType, ci)

	ccp.logChannelMessage(eventType, ci)
}

func (ccp *SubstrateChainProcessor) handleConnectionMessage(eventType string, ci provider.ConnectionInfo, ibcMessagesCache processor.IBCMessagesCache) {
	ccp.connectionClients[ci.ConnID] = ci.ClientID
	connectionKey := processor.ConnectionInfoConnectionKey(ci)
	if eventType == conntypes.EventTypeConnectionOpenInit {
		found := false
		for k := range ccp.connectionStateCache {
			// Don't add a connectionKey to the connectionStateCache without counterparty connection ID
			// since we already have the connectionKey in the connectionStateCache which includes the
			// counterparty connection ID.
			if k.MsgInitKey() == connectionKey {
				found = true
				break
			}
		}
		if !found {
			ccp.connectionStateCache[connectionKey] = false
		}
	} else {
		// Clear out MsgInitKeys once we have the counterparty connection ID
		delete(ccp.connectionStateCache, connectionKey.MsgInitKey())
		open := (eventType == conntypes.EventTypeConnectionOpenAck || eventType == conntypes.EventTypeConnectionOpenConfirm)
		ccp.connectionStateCache[connectionKey] = open
	}
	ibcMessagesCache.ConnectionHandshake.Retain(connectionKey, eventType, ci)

	ccp.logConnectionMessage(eventType, ci)
}

func (ccp *SubstrateChainProcessor) handleClientMessage(eventType string, ci clientInfo) {
	ccp.latestClientState.update(ci)
	ccp.logObservedIBCMessage(eventType, zap.String("client_id", ci.ClientID))
}

func (ccp *SubstrateChainProcessor) logObservedIBCMessage(m string, fields ...zap.Field) {
	ccp.log.With(zap.String("event_type", m)).Debug("Observed IBC message", fields...)
}

func (ccp *SubstrateChainProcessor) logPacketMessage(message string, pi provider.PacketInfo) {
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

func (ccp *SubstrateChainProcessor) logChannelMessage(message string, ci provider.ChannelInfo) {
	ccp.logObservedIBCMessage(message,
		zap.String("channel_id", ci.ChannelID),
		zap.String("port_id", ci.PortID),
		zap.String("counterparty_channel_id", ci.CounterpartyChannelID),
		zap.String("counterparty_port_id", ci.CounterpartyPortID),
		zap.String("connection_id", ci.ConnID),
	)
}

func (ccp *SubstrateChainProcessor) logConnectionMessage(message string, ci provider.ConnectionInfo) {
	ccp.logObservedIBCMessage(message,
		zap.String("client_id", ci.ClientID),
		zap.String("connection_id", ci.ConnID),
		zap.String("counterparty_client_id", ci.CounterpartyClientID),
		zap.String("counterparty_connection_id", ci.CounterpartyConnID),
	)
}
