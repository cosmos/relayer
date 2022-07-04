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
	case processor.MsgChannelOpenInit, processor.MsgChannelOpenTry, processor.MsgChannelOpenAck, processor.MsgChannelOpenConfirm:
		ccp.handleChannelMessage(m.action, m.info.(provider.ChannelInfo), ibcMessagesCache)
	case processor.MsgConnectionOpenInit, processor.MsgConnectionOpenTry, processor.MsgConnectionOpenAck, processor.MsgConnectionOpenConfirm:
		ccp.handleConnectionMessage(m.action, m.info.(provider.ConnectionInfo), ibcMessagesCache)
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

	ibcMessagesCache.PacketFlow.Retain(channelKey, action, pi.Sequence, pi)
	ccp.logPacketMessage(processor.ShortAction(action), pi)
}

func (ccp *CosmosChainProcessor) handleChannelMessage(action string, ci provider.ChannelInfo, ibcMessagesCache processor.IBCMessagesCache) {
	ccp.channelConnections[ci.ChannelID] = ci.ConnectionID
	switch action {
	case processor.MsgChannelOpenInit:
		// CounterpartyConnectionID is needed to construct MsgChannelOpenTry.
		for k := range ccp.connectionStateCache {
			if k.ConnectionID == ci.ConnectionID {
				ci.CounterpartyConnectionID = k.CounterpartyConnectionID
				break
			}
		}
		ccp.channelStateCache[processor.ChannelInfoChannelKey(ci)] = false
	case processor.MsgChannelOpenTry:
		ccp.channelStateCache[processor.ChannelInfoChannelKey(ci)] = false
	case processor.MsgChannelOpenAck, processor.MsgChannelOpenConfirm:
		ccp.channelStateCache[processor.ChannelInfoChannelKey(ci)] = true
	case processor.MsgChannelCloseInit, processor.MsgChannelCloseConfirm:
		for k := range ccp.channelStateCache {
			if k.PortID == ci.PortID && k.ChannelID == ci.ChannelID {
				ccp.channelStateCache[k] = false
				break
			}
		}
	}
	ibcMessagesCache.ChannelHandshake.Retain(action, ci)

	ccp.logChannelMessage(processor.ShortAction(action), ci)
}

func (ccp *CosmosChainProcessor) handleConnectionMessage(action string, ci provider.ConnectionInfo, ibcMessagesCache processor.IBCMessagesCache) {
	ccp.connectionClients[ci.ConnectionID] = ci.ClientID
	switch action {
	case processor.MsgConnectionOpenAck, processor.MsgConnectionOpenConfirm:
		ccp.connectionStateCache[processor.ConnectionInfoConnectionKey(ci)] = true
	default:
		ccp.connectionStateCache[processor.ConnectionInfoConnectionKey(ci)] = false
	}
	ibcMessagesCache.ConnectionHandshake.Retain(action, ci)

	ccp.logConnectionMessage(processor.ShortAction(action), ci)
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

func (ccp *CosmosChainProcessor) logChannelMessage(message string, ci provider.ChannelInfo) {
	ccp.logObservedIBCMessage(message,
		zap.String("channel_id", ci.ChannelID),
		zap.String("port_id", ci.PortID),
		zap.String("counterparty_channel_id", ci.CounterpartyChannelID),
		zap.String("counterparty_port_id", ci.CounterpartyPortID),
		zap.String("connection_id", ci.ConnectionID),
	)
}

func (ccp *CosmosChainProcessor) logConnectionMessage(message string, ci provider.ConnectionInfo) {
	ccp.logObservedIBCMessage(message,
		zap.String("client_id", ci.ClientID),
		zap.String("connection_id", ci.ConnectionID),
		zap.String("counterparty_client_id", ci.CounterpartyClientID),
		zap.String("counterparty_connection_id", ci.CounterpartyConnectionID),
	)
}
