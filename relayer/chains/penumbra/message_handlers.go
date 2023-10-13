package penumbra

import (
	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/chains"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func (pcp *PenumbraChainProcessor) handleMessage(m chains.IbcMessage, c processor.IBCMessagesCache) {
	switch t := m.Info.(type) {
	case *chains.PacketInfo:
		pcp.handlePacketMessage(m.EventType, provider.PacketInfo(*t), c)
	case *chains.ChannelInfo:
		pcp.handleChannelMessage(m.EventType, provider.ChannelInfo(*t), c)
	case *chains.ConnectionInfo:
		pcp.handleConnectionMessage(m.EventType, provider.ConnectionInfo(*t), c)
	case *chains.ClientInfo:
		pcp.handleClientMessage(m.EventType, *t)
	}
}

func (pcp *PenumbraChainProcessor) handlePacketMessage(action string, pi provider.PacketInfo, c processor.IBCMessagesCache) {
	channelKey, err := processor.PacketInfoChannelKey(action, pi)
	if err != nil {
		pcp.log.Error("Unexpected error handling packet message",
			zap.String("action", action),
			zap.Uint64("sequence", pi.Sequence),
			zap.Any("channel", channelKey),
			zap.Error(err),
		)
		return
	}

	if !c.PacketFlow.ShouldRetainSequence(pcp.pathProcessors, channelKey, pcp.chainProvider.ChainId(), action, pi.Sequence) {
		pcp.log.Warn("Not retaining packet message",
			zap.String("action", action),
			zap.Uint64("sequence", pi.Sequence),
			zap.Any("channel", channelKey),
		)
		return
	}

	c.PacketFlow.Retain(channelKey, action, pi)
	pcp.logPacketMessage(action, pi)
}

func (pcp *PenumbraChainProcessor) handleChannelMessage(eventType string, ci provider.ChannelInfo, ibcMessagesCache processor.IBCMessagesCache) {
	pcp.channelConnections[ci.ChannelID] = ci.ConnID
	channelKey := processor.ChannelInfoChannelKey(ci)

	if eventType == chantypes.EventTypeChannelOpenInit {
		found := false
		for k := range pcp.channelStateCache {
			// Don't add a channelKey to the channelStateCache without counterparty channel ID
			// since we already have the channelKey in the channelStateCache which includes the
			// counterparty channel ID.
			if k.MsgInitKey() == channelKey {
				found = true
				break
			}
		}
		if !found {
			pcp.channelStateCache.SetOpen(channelKey, false, ci.Order)
		}
	} else {
		switch eventType {
		case chantypes.EventTypeChannelOpenTry:
			pcp.channelStateCache.SetOpen(channelKey, false, ci.Order)
		case chantypes.EventTypeChannelOpenAck, chantypes.EventTypeChannelOpenConfirm:
			pcp.channelStateCache.SetOpen(channelKey, true, ci.Order)
		case chantypes.EventTypeChannelClosed, chantypes.EventTypeChannelCloseConfirm:
			for k := range pcp.channelStateCache {
				if k.PortID == ci.PortID && k.ChannelID == ci.ChannelID {
					pcp.channelStateCache.SetOpen(channelKey, false, ci.Order)
					break
				}
			}
		}
		// Clear out MsgInitKeys once we have the counterparty channel ID
		delete(pcp.channelStateCache, channelKey.MsgInitKey())
	}

	ibcMessagesCache.ChannelHandshake.Retain(channelKey, eventType, ci)

	pcp.logChannelMessage(eventType, ci)
}

func (pcp *PenumbraChainProcessor) handleConnectionMessage(eventType string, ci provider.ConnectionInfo, ibcMessagesCache processor.IBCMessagesCache) {
	pcp.connectionClients[ci.ConnID] = ci.ClientID
	connectionKey := processor.ConnectionInfoConnectionKey(ci)
	if eventType == conntypes.EventTypeConnectionOpenInit {
		found := false
		for k := range pcp.connectionStateCache {
			// Don't add a connectionKey to the connectionStateCache without counterparty connection ID
			// since we already have the connectionKey in the connectionStateCache which includes the
			// counterparty connection ID.
			if k.MsgInitKey() == connectionKey {
				found = true
				break
			}
		}
		if !found {
			pcp.connectionStateCache[connectionKey] = false
		}
	} else {
		// Clear out MsgInitKeys once we have the counterparty connection ID
		delete(pcp.connectionStateCache, connectionKey.MsgInitKey())
		open := (eventType == conntypes.EventTypeConnectionOpenAck || eventType == conntypes.EventTypeConnectionOpenConfirm)
		pcp.connectionStateCache[connectionKey] = open
	}
	ibcMessagesCache.ConnectionHandshake.Retain(connectionKey, eventType, ci)

	pcp.logConnectionMessage(eventType, ci)
}

func (pcp *PenumbraChainProcessor) handleClientMessage(eventType string, ci chains.ClientInfo) {
	pcp.latestClientState.update(ci)
	pcp.logObservedIBCMessage(eventType, zap.String("client_id", ci.ClientID))
}

func (pcp *PenumbraChainProcessor) logObservedIBCMessage(m string, fields ...zap.Field) {
	pcp.log.With(zap.String("event_type", m)).Debug("Observed IBC message", fields...)
}

func (pcp *PenumbraChainProcessor) logPacketMessage(message string, pi provider.PacketInfo) {
	if !pcp.log.Core().Enabled(zapcore.DebugLevel) {
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
	pcp.logObservedIBCMessage(message, fields...)
}

func (pcp *PenumbraChainProcessor) logChannelMessage(message string, ci provider.ChannelInfo) {
	pcp.logObservedIBCMessage(message,
		zap.String("channel_id", ci.ChannelID),
		zap.String("port_id", ci.PortID),
		zap.String("counterparty_channel_id", ci.CounterpartyChannelID),
		zap.String("counterparty_port_id", ci.CounterpartyPortID),
		zap.String("connection_id", ci.ConnID),
	)
}

func (pcp *PenumbraChainProcessor) logConnectionMessage(message string, ci provider.ConnectionInfo) {
	pcp.logObservedIBCMessage(message,
		zap.String("client_id", ci.ClientID),
		zap.String("connection_id", ci.ConnID),
		zap.String("counterparty_client_id", ci.CounterpartyClientID),
		zap.String("counterparty_connection_id", ci.CounterpartyConnID),
	)
}
