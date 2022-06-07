package cosmos

import (
	"fmt"

	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	"go.uber.org/zap"
)

type MsgHandlerParams struct {
	messageInfo   interface{}
	foundMessages processor.ChannelMessageCache
}

var messageHandlers = map[string]func(*CosmosChainProcessor, MsgHandlerParams){
	processor.MsgTransfer:        (*CosmosChainProcessor).handleMsgTransfer,
	processor.MsgRecvPacket:      (*CosmosChainProcessor).handleMsgRecvPacket,
	processor.MsgAcknowledgement: (*CosmosChainProcessor).handleMsgAcknowlegement,
	processor.MsgTimeout:         (*CosmosChainProcessor).handleMsgTimeout,
	processor.MsgTimeoutOnClose:  (*CosmosChainProcessor).handleMsgTimeoutOnClose,

	processor.MsgCreateClient:       (*CosmosChainProcessor).handleMsgCreateClient,
	processor.MsgUpdateClient:       (*CosmosChainProcessor).handleMsgUpdateClient,
	processor.MsgUpgradeClient:      (*CosmosChainProcessor).handleMsgUpgradeClient,
	processor.MsgSubmitMisbehaviour: (*CosmosChainProcessor).handleMsgSubmitMisbehaviour,

	processor.MsgConnectionOpenInit:    (*CosmosChainProcessor).handleMsgConnectionOpenInit,
	processor.MsgConnectionOpenTry:     (*CosmosChainProcessor).handleMsgConnectionOpenTry,
	processor.MsgConnectionOpenAck:     (*CosmosChainProcessor).handleMsgConnectionOpenAck,
	processor.MsgConnectionOpenConfirm: (*CosmosChainProcessor).handleMsgConnectionOpenConfirm,

	processor.MsgChannelCloseConfirm: (*CosmosChainProcessor).handleMsgChannelCloseConfirm,
	processor.MsgChannelCloseInit:    (*CosmosChainProcessor).handleMsgChannelCloseInit,
	processor.MsgChannelOpenAck:      (*CosmosChainProcessor).handleMsgChannelOpenAck,
	processor.MsgChannelOpenConfirm:  (*CosmosChainProcessor).handleMsgChannelOpenConfirm,
	processor.MsgChannelOpenInit:     (*CosmosChainProcessor).handleMsgChannelOpenInit,
	processor.MsgChannelOpenTry:      (*CosmosChainProcessor).handleMsgChannelOpenTry,
}

func getTypedMessage[T *packetInfo | *channelInfo | *clientInfo | *connectionInfo](messageInfo interface{}) T {
	typedInfo, ok := messageInfo.(T)
	if !ok {
		panic("invalid message info provided")
	}
	return typedInfo
}

// isPacketApplicable returns true if packet is applicable to the channels for path processors that are subscribed to this chain processor
func (ccp *CosmosChainProcessor) isPacketApplicable(message string, packetInfo *packetInfo, foundMessages processor.ChannelMessageCache, channelKey processor.ChannelKey) bool {
	if !ccp.isRelayedChannel(channelKey) {
		return false
	}
	if _, ok := foundMessages[channelKey]; !ok {
		return true
	}
	if _, ok := foundMessages[channelKey][message]; !ok {
		return true
	}
	for sequence := range foundMessages[channelKey][message] {
		if sequence == packetInfo.packet.Sequence {
			// already have this sequence number
			// there can be multiple MsgRecvPacket, MsgAcknowlegement, MsgTimeout, and MsgTimeoutOnClose for the same packet
			// from different relayers.
			return false
		}
	}

	return true
}

// retainApplicableMessage assumes the packet is applicable to the channels for a path processor that is subscribed to this chain processor.
// It creates cache path if it doesn't exist, then caches message.
func retainPacketMessage(message string, packetInfo *packetInfo, foundMessages processor.ChannelMessageCache, channelKey processor.ChannelKey, ibcMsg provider.RelayerMessage) {
	if _, ok := foundMessages[channelKey]; !ok {
		foundMessages[channelKey] = make(processor.MessageCache)
	}
	if _, ok := foundMessages[channelKey][message]; !ok {
		foundMessages[channelKey][message] = make(processor.SequenceCache)
	}
	foundMessages[channelKey][message][packetInfo.packet.Sequence] = ibcMsg
}

// BEGIN packet msg handlers
func (ccp *CosmosChainProcessor) handleMsgTransfer(p MsgHandlerParams) {
	packetInfo := getTypedMessage[*packetInfo](p.messageInfo)
	// source chain processor will call this handler
	// source channel used as key because MsgTransfer is sent to source chain
	channelKey := processor.ChannelKey{
		ChannelID:             packetInfo.packet.SourceChannel,
		PortID:                packetInfo.packet.SourcePort,
		CounterpartyChannelID: packetInfo.packet.DestinationChannel,
		CounterpartyPortID:    packetInfo.packet.DestinationPort,
	}
	if !ccp.isPacketApplicable(processor.MsgTransfer, packetInfo, p.foundMessages, channelKey) {
		return
	}
	// Construct the start of the MsgRecvPacket for the counterparty chain.
	// PathProcessor will determine if this is needed.
	// For example, if a MsgRecvPacket is not detected on the counterparty chain, and
	// a MsgAcknowledgement, MsgTimeout, or MsgTimeout is not detected yet on this chain,
	// and the packet timeout has not expired, a MsgRecvPacket will be sent to the counterparty chain
	// using this information with the packet commitment proof from this chain added.
	retainPacketMessage(processor.MsgTransfer, packetInfo, p.foundMessages, channelKey, cosmos.NewCosmosMessage(&chantypes.MsgRecvPacket{
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
}

func (ccp *CosmosChainProcessor) handleMsgRecvPacket(p MsgHandlerParams) {
	packetInfo := getTypedMessage[*packetInfo](p.messageInfo)
	// destination chain processor will call this handler
	// destination channel used because MsgRecvPacket is sent to destination chain
	channelKey := processor.ChannelKey{
		ChannelID:             packetInfo.packet.DestinationChannel,
		PortID:                packetInfo.packet.DestinationPort,
		CounterpartyChannelID: packetInfo.packet.SourceChannel,
		CounterpartyPortID:    packetInfo.packet.SourcePort,
	}
	if !ccp.isPacketApplicable(processor.MsgRecvPacket, packetInfo, p.foundMessages, channelKey) {
		return
	}
	// Construct the start of the MsgAcknowledgement for the counterparty chain.
	// PathProcessor will determine if this is needed.
	// For example, if a MsgAcknowledgement is not detected yet on the counterparty chain,
	// a MsgAcknowledgement will be sent to the counterparty chain
	// using this information with the packet acknowledgement commitment proof from this chain added.
	retainPacketMessage(processor.MsgRecvPacket, packetInfo, p.foundMessages, channelKey, cosmos.NewCosmosMessage(&chantypes.MsgAcknowledgement{
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
}

func (ccp *CosmosChainProcessor) handleMsgAcknowlegement(p MsgHandlerParams) {
	packetInfo := getTypedMessage[*packetInfo](p.messageInfo)
	// source chain processor will call this handler
	// source channel used as key because MsgAcknowlegement is sent to source chain
	channelKey := processor.ChannelKey{
		ChannelID:             packetInfo.packet.SourceChannel,
		PortID:                packetInfo.packet.SourcePort,
		CounterpartyChannelID: packetInfo.packet.DestinationChannel,
		CounterpartyPortID:    packetInfo.packet.DestinationPort,
	}
	if !ccp.isPacketApplicable(processor.MsgAcknowledgement, packetInfo, p.foundMessages, channelKey) {
		return
	}
	// Retaining a nil message here because this is for book-keeping in the PathProcessor cache only.
	// A message does not need to be constructed for the counterparty chain after the MsgAcknowledgement is observed,
	// but we want to tell the PathProcessor that the packet flow is complete for this sequence.
	retainPacketMessage(processor.MsgAcknowledgement, packetInfo, p.foundMessages, channelKey, nil)
	ccp.logPacketMessage("MsgAcknowledgement", packetInfo)
}

func (ccp *CosmosChainProcessor) handleMsgTimeout(p MsgHandlerParams) {
	packetInfo := getTypedMessage[*packetInfo](p.messageInfo)
	// source chain processor will call this handler
	// source channel used as key because MsgTimeout is sent to source chain
	channelKey := processor.ChannelKey{
		ChannelID:             packetInfo.packet.SourceChannel,
		PortID:                packetInfo.packet.SourcePort,
		CounterpartyChannelID: packetInfo.packet.DestinationChannel,
		CounterpartyPortID:    packetInfo.packet.DestinationPort,
	}
	if !ccp.isPacketApplicable(processor.MsgTimeout, packetInfo, p.foundMessages, channelKey) {
		return
	}
	// Retaining a nil message here because this is for book-keeping in the PathProcessor cache only.
	// A message does not need to be constructed for the counterparty chain after the MsgTimeout is observed,
	// but we want to tell the PathProcessor that the packet flow is complete for this sequence.
	retainPacketMessage(processor.MsgTimeout, packetInfo, p.foundMessages, channelKey, nil)
	ccp.logPacketMessage("MsgTimeout", packetInfo)
}

func (ccp *CosmosChainProcessor) handleMsgTimeoutOnClose(p MsgHandlerParams) {
	packetInfo := getTypedMessage[*packetInfo](p.messageInfo)
	// source channel used because timeout is sent to source chain
	channelKey := processor.ChannelKey{
		ChannelID:             packetInfo.packet.SourceChannel,
		PortID:                packetInfo.packet.SourcePort,
		CounterpartyChannelID: packetInfo.packet.DestinationChannel,
		CounterpartyPortID:    packetInfo.packet.DestinationPort,
	}
	if !ccp.isPacketApplicable(processor.MsgTimeoutOnClose, packetInfo, p.foundMessages, channelKey) {
		return
	}
	// Retaining a nil message here because this is for book-keeping in the PathProcessor cache only.
	// A message does not need to be constructed for the counterparty chain after the MsgTimeoutOnClose is observed,
	// but we want to tell the PathProcessor that the packet flow is complete for this sequence.
	retainPacketMessage(processor.MsgTimeoutOnClose, packetInfo, p.foundMessages, channelKey, nil)
	ccp.logPacketMessage("MsgTimeoutOnClose", packetInfo)
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

// BEGIN client msg handlers

func (ccp *CosmosChainProcessor) handleMsgCreateClient(p MsgHandlerParams) {
	clientInfo := getTypedMessage[*clientInfo](p.messageInfo)
	// save the latest consensus height and header for this client
	ccp.latestClientState.UpdateLatestClientState(*clientInfo)
	ccp.logClientMessage("MsgCreateClient", clientInfo)
}

func (ccp *CosmosChainProcessor) handleMsgUpdateClient(p MsgHandlerParams) {
	clientInfo := getTypedMessage[*clientInfo](p.messageInfo)
	// save the latest consensus height and header for this client
	ccp.latestClientState.UpdateLatestClientState(*clientInfo)
	ccp.logClientMessage("MsgUpdateClient", clientInfo)
}

func (ccp *CosmosChainProcessor) handleMsgUpgradeClient(p MsgHandlerParams) {
	clientInfo := getTypedMessage[*clientInfo](p.messageInfo)
	// save the latest consensus height and header for this client
	ccp.latestClientState.UpdateLatestClientState(*clientInfo)
	ccp.logClientMessage("MsgUpgradeClient", clientInfo)
}

func (ccp *CosmosChainProcessor) handleMsgSubmitMisbehaviour(p MsgHandlerParams) {
	clientInfo := getTypedMessage[*clientInfo](p.messageInfo)
	// save the latest consensus height and header for this client
	ccp.latestClientState.UpdateLatestClientState(*clientInfo)
	ccp.logClientMessage("MsgSubmitMisbehaviour", clientInfo)
}

func (ccp *CosmosChainProcessor) logClientMessage(message string, clientInfo *clientInfo) {
	ccp.log.Debug("observed ibc message",
		zap.String("message", message),
		zap.String("client_id", clientInfo.clientID),
	)
}

// END client msg handlers

// BEGIN connection msg handlers

func (ccp *CosmosChainProcessor) handleMsgConnectionOpenInit(p MsgHandlerParams) {
	connectionInfo := getTypedMessage[*connectionInfo](p.messageInfo)
	ccp.logConnectionMessage("MsgConnectionOpenInit", connectionInfo)
}

func (ccp *CosmosChainProcessor) handleMsgConnectionOpenTry(p MsgHandlerParams) {
	connectionInfo := getTypedMessage[*connectionInfo](p.messageInfo)
	ccp.logConnectionMessage("MsgConnectionOpenTry", connectionInfo)
}

func (ccp *CosmosChainProcessor) handleMsgConnectionOpenAck(p MsgHandlerParams) {
	connectionInfo := getTypedMessage[*connectionInfo](p.messageInfo)
	ccp.logConnectionMessage("MsgConnectionOpenAck", connectionInfo)
}

func (ccp *CosmosChainProcessor) handleMsgConnectionOpenConfirm(p MsgHandlerParams) {
	connectionInfo := getTypedMessage[*connectionInfo](p.messageInfo)
	ccp.logConnectionMessage("MsgConnectionOpenConfirm", connectionInfo)
}

func (ccp *CosmosChainProcessor) logConnectionMessage(message string, connectionInfo *connectionInfo) {
	ccp.log.Debug("observed ibc message",
		zap.String("message", message),
		zap.String("client_id", connectionInfo.clientID),
		zap.String("connection_id", connectionInfo.connectionID),
		zap.String("counterparty_client_id", connectionInfo.counterpartyClientID),
		zap.String("counterparty_connection_id", connectionInfo.counterpartyConnectionID),
	)
}

// END connection msg handlers

// BEGIN channel msg handlers

func (ccp *CosmosChainProcessor) handleMsgChannelOpenInit(p MsgHandlerParams) {
	channelInfo := getTypedMessage[*channelInfo](p.messageInfo)
	ccp.logChannelMessage("MsgChannelOpenInit", channelInfo)
}

func (ccp *CosmosChainProcessor) handleMsgChannelOpenTry(p MsgHandlerParams) {
	channelInfo := getTypedMessage[*channelInfo](p.messageInfo)
	ccp.logChannelMessage("MsgChannelOpenTry", channelInfo)
}

func (ccp *CosmosChainProcessor) handleMsgChannelOpenAck(p MsgHandlerParams) {
	channelInfo := getTypedMessage[*channelInfo](p.messageInfo)
	channelKey := processor.ChannelKey{
		ChannelID: channelInfo.channelID,
		PortID: channelInfo.portID,
		CounterpartyChannelID: channelInfo.counterpartyChannelID,
		CounterpartyPortID: channelInfo.counterpartyPortID,
	}
	ccp.channelOpenState[channelKey] = true
	ccp.logChannelMessage("MsgChannelOpenAck", channelInfo)
}

func (ccp *CosmosChainProcessor) handleMsgChannelOpenConfirm(p MsgHandlerParams) {
	channelInfo := getTypedMessage[*channelInfo](p.messageInfo)
	// using flipped counterparty since counterparty initialized this handshake
	channelKey := processor.ChannelKey{
		ChannelID: channelInfo.counterpartyChannelID,
		PortID: channelInfo.counterpartyPortID,
		CounterpartyChannelID: channelInfo.channelID,
		CounterpartyPortID: channelInfo.portID,
	}
	ccp.channelOpenState[channelKey] = true
	ccp.logChannelMessage("MsgChannelOpenConfirm", channelInfo)
}

func (ccp *CosmosChainProcessor) handleMsgChannelCloseInit(p MsgHandlerParams) {
	channelInfo := getTypedMessage[*channelInfo](p.messageInfo)
	channelKey := processor.ChannelKey{
		ChannelID: channelInfo.channelID,
		PortID: channelInfo.portID,
		CounterpartyChannelID: channelInfo.counterpartyChannelID,
		CounterpartyPortID: channelInfo.counterpartyPortID,
	}
	ccp.channelOpenState[channelKey] = false
	ccp.logChannelMessage("MsgChannelCloseInit", channelInfo)
}

func (ccp *CosmosChainProcessor) handleMsgChannelCloseConfirm(p MsgHandlerParams) {
	channelInfo := getTypedMessage[*channelInfo](p.messageInfo)
	// using flipped counterparty since counterparty initialized this channel close
	channelKey := processor.ChannelKey{
		ChannelID: channelInfo.counterpartyChannelID,
		PortID: channelInfo.counterpartyPortID,
		CounterpartyChannelID: channelInfo.channelID,
		CounterpartyPortID: channelInfo.portID,
	}
	ccp.channelOpenState[channelKey] = false
	ccp.logChannelMessage("MsgChannelCloseConfirm", channelInfo)
}

func (ccp *CosmosChainProcessor) logChannelMessage(message string, channelInfo *channelInfo) {
	ccp.log.Debug("observed ibc message",
		zap.String("message", message),
		zap.String("channel_id", channelInfo.channelID),
		zap.String("port_id", channelInfo.portID),
		zap.String("counterparty_channel_id", channelInfo.counterpartyChannelID),
		zap.String("counterparty_port_id", channelInfo.counterpartyPortID),
		zap.String("connection_id", channelInfo.connectionID),
	)
}

// END channel msg handlers
