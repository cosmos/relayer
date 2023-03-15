package icon

import (
	"context"

	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"

	"github.com/icon-project/ibc-relayer/relayer/processor"
	"github.com/icon-project/ibc-relayer/relayer/provider"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// TODO: implement for all the message types
func (icp *IconChainProcessor) handleMessage(ctx context.Context, m ibcMessage, c processor.IBCMessagesCache) {
	switch t := m.info.(type) {
	case *packetInfo:
		icp.handlePacketMessage(m.eventType, provider.PacketInfo(*t), c)
		// case *channelInfo:
		// 	ccp.handleChannelMessage(m.eventType, provider.ChannelInfo(*t), c)
		// case *connectionInfo:
		// 	ccp.handleConnectionMessage(m.eventType, provider.ConnectionInfo(*t), c)
		// case *clientInfo:
		// 	ccp.handleClientMessage(ctx, m.eventType, *t)
	}
}

func (icp *IconChainProcessor) handlePacketMessage(eventType string, pi provider.PacketInfo, c processor.IBCMessagesCache) {
	// TODO: implement for packet messages
	k, err := processor.PacketInfoChannelKey(eventType, pi)
	if err != nil {
		icp.log.Error("Unexpected error handling packet message",
			zap.String("event_type", eventType),
			zap.Uint64("sequence", pi.Sequence),
			zap.Inline(k),
			zap.Error(err),
		)
		return
	}

	if !c.PacketFlow.ShouldRetainSequence(icp.pathProcessors, k, icp.chainProvider.ChainId(), eventType, pi.Sequence) {
		icp.log.Debug("Not retaining packet message",
			zap.String("event_type", eventType),
			zap.Uint64("sequence", pi.Sequence),
			zap.Inline(k),
		)
		return
	}

	icp.log.Debug("Retaining packet message",
		zap.String("event_type", eventType),
		zap.Uint64("sequence", pi.Sequence),
		zap.Inline(k),
	)

	c.PacketFlow.Retain(k, eventType, pi)
	icp.logPacketMessage(eventType, pi)

}

func (icp *IconChainProcessor) handleChannelMessage(eventType string, ci provider.ChannelInfo, ibcMessagesCache processor.IBCMessagesCache) {
	icp.channelConnections[ci.ChannelID] = ci.ConnID
	channelKey := processor.ChannelInfoChannelKey(ci)

	if eventType == chantypes.EventTypeChannelOpenInit {
		found := false
		for k := range icp.channelStateCache {
			// Don't add a channelKey to the channelStateCache without counterparty channel ID
			// since we already have the channelKey in the channelStateCache which includes the
			// counterparty channel ID.
			if k.MsgInitKey() == channelKey {
				found = true
				break
			}
		}
		if !found {
			icp.channelStateCache[channelKey] = false
		}
	} else {
		switch eventType {
		case chantypes.EventTypeChannelOpenTry:
			icp.channelStateCache[channelKey] = false
		case chantypes.EventTypeChannelOpenAck, chantypes.EventTypeChannelOpenConfirm:
			icp.channelStateCache[channelKey] = true
		case chantypes.EventTypeChannelCloseConfirm:
			for k := range icp.channelStateCache {
				if k.PortID == ci.PortID && k.ChannelID == ci.ChannelID {
					icp.channelStateCache[k] = false
					break
				}
			}
		}
		// Clear out MsgInitKeys once we have the counterparty channel ID
		delete(icp.channelStateCache, channelKey.MsgInitKey())
	}

	ibcMessagesCache.ChannelHandshake.Retain(channelKey, eventType, ci)

	icp.logChannelMessage(eventType, ci)

}

func (icp *IconChainProcessor) handleConnectionMessage(eventType string, ci provider.ConnectionInfo, ibcMessagesCache processor.IBCMessagesCache) {
	icp.connectionClients[ci.ConnID] = ci.ClientID
	connectionKey := processor.ConnectionInfoConnectionKey(ci)
	if eventType == conntypes.EventTypeConnectionOpenInit {
		found := false
		for k := range icp.connectionStateCache {
			// Don't add a connectionKey to the connectionStateCache without counterparty connection ID
			// since we already have the connectionKey in the connectionStateCache which includes the
			// counterparty connection ID.
			if k.MsgInitKey() == connectionKey {
				found = true
				break
			}
		}
		if !found {
			icp.connectionStateCache[connectionKey] = false
		}
	} else {
		// Clear out MsgInitKeys once we have the counterparty connection ID
		delete(icp.connectionStateCache, connectionKey.MsgInitKey())
		open := (eventType == conntypes.EventTypeConnectionOpenAck || eventType == conntypes.EventTypeConnectionOpenConfirm)
		icp.connectionStateCache[connectionKey] = open
	}
	ibcMessagesCache.ConnectionHandshake.Retain(connectionKey, eventType, ci)

	icp.logConnectionMessage(eventType, ci)

}

func (icp *IconChainProcessor) handleClientMessage(ctx context.Context, eventType string, ci clientInfo) {
	// TODO:
	icp.latestClientState.update(ctx, ci, icp)
	icp.logObservedIBCMessage(eventType, zap.String("client_id", ci.clientID))

}

func (ccp *IconChainProcessor) logObservedIBCMessage(m string, fields ...zap.Field) {
	ccp.log.With(zap.String("event_type", m)).Debug("Observed IBC message", fields...)
}

func (icp *IconChainProcessor) logPacketMessage(message string, pi provider.PacketInfo) {
	if !icp.log.Core().Enabled(zapcore.DebugLevel) {
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
	icp.logObservedIBCMessage(message, fields...)
}

func (icp *IconChainProcessor) logChannelMessage(message string, ci provider.ChannelInfo) {
	icp.logObservedIBCMessage(message,
		zap.String("channel_id", ci.ChannelID),
		zap.String("port_id", ci.PortID),
		zap.String("counterparty_channel_id", ci.CounterpartyChannelID),
		zap.String("counterparty_port_id", ci.CounterpartyPortID),
		zap.String("connection_id", ci.ConnID),
	)
}

func (icp *IconChainProcessor) logConnectionMessage(message string, ci provider.ConnectionInfo) {
	icp.logObservedIBCMessage(message,
		zap.String("client_id", ci.ClientID),
		zap.String("connection_id", ci.ConnID),
		zap.String("counterparty_client_id", ci.CounterpartyClientID),
		zap.String("counterparty_connection_id", ci.CounterpartyConnID),
	)
}
