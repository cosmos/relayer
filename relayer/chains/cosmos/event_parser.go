package cosmos

import (
	"encoding/hex"
	"strconv"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v5/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	abci "github.com/tendermint/tendermint/abci/types"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ibcMessage is the type used for parsing all possible properties of IBC messages
type ibcMessage struct {
	eventType string
	info      ibcMessageInfo
}

type ibcMessageInfo interface {
	parseAttrs(log *zap.Logger, attrs []sdk.Attribute)
	MarshalLogObject(enc zapcore.ObjectEncoder) error
}

func (ccp *CosmosChainProcessor) ibcMessagesFromBlockEvents(
	beginBlockEvents, endBlockEvents []abci.Event,
	height uint64,
) (res []ibcMessage) {
	chainID := ccp.chainProvider.ChainId()
	res = append(res, ibcMessagesFromEvents(ccp.log, beginBlockEvents, chainID, height)...)
	res = append(res, ibcMessagesFromEvents(ccp.log, endBlockEvents, chainID, height)...)
	return res
}

type packetKey struct {
	sequence uint64
	channel  processor.ChannelKey
}

// ibcMessagesFromTransaction parses all events within a transaction to find IBC messages
func ibcMessagesFromEvents(
	log *zap.Logger,
	events []abci.Event,
	chainID string,
	height uint64,
) (messages []ibcMessage) {
	for _, event := range events {
		evt := sdk.StringifyEvent(event)
		m := parseIBCMessageFromEvent(log, evt, chainID, height)
		if m == nil || m.info == nil {
			// Not an IBC message, don't need to log here
			continue
		}
		messages = append(messages, *m)
	}
	return messages
}

func parseIBCMessageFromEvent(
	log *zap.Logger,
	event sdk.StringEvent,
	chainID string,
	height uint64,
) *ibcMessage {
	switch event.Type {
	case chantypes.EventTypeSendPacket, chantypes.EventTypeRecvPacket, chantypes.EventTypeWriteAck,
		chantypes.EventTypeAcknowledgePacket, chantypes.EventTypeTimeoutPacket,
		chantypes.EventTypeTimeoutPacketOnClose:
		pi := &packetInfo{Height: height}
		pi.parseAttrs(log, event.Attributes)
		return &ibcMessage{
			eventType: event.Type,
			info:      pi,
		}
	case chantypes.EventTypeChannelOpenInit, chantypes.EventTypeChannelOpenTry,
		chantypes.EventTypeChannelOpenAck, chantypes.EventTypeChannelOpenConfirm,
		chantypes.EventTypeChannelCloseInit, chantypes.EventTypeChannelCloseConfirm:
		ci := &channelInfo{Height: height}
		ci.parseAttrs(log, event.Attributes)
		return &ibcMessage{
			eventType: event.Type,
			info:      ci,
		}
	case conntypes.EventTypeConnectionOpenInit, conntypes.EventTypeConnectionOpenTry,
		conntypes.EventTypeConnectionOpenAck, conntypes.EventTypeConnectionOpenConfirm:
		ci := &connectionInfo{Height: height}
		ci.parseAttrs(log, event.Attributes)
		return &ibcMessage{
			eventType: event.Type,
			info:      ci,
		}
	case clienttypes.EventTypeCreateClient, clienttypes.EventTypeUpdateClient,
		clienttypes.EventTypeUpgradeClient, clienttypes.EventTypeSubmitMisbehaviour,
		clienttypes.EventTypeUpdateClientProposal:
		ci := new(clientInfo)
		ci.parseAttrs(log, event.Attributes)
		return &ibcMessage{
			eventType: event.Type,
			info:      ci,
		}
	}
	return nil
}

func (msg *ibcMessage) parseIBCPacketReceiveMessageFromEvent(
	log *zap.Logger,
	event sdk.StringEvent,
	chainID string,
	height uint64,
) *ibcMessage {
	var pi *packetInfo
	if msg.info == nil {
		pi = &packetInfo{Height: height}
		msg.info = pi
	} else {
		pi = msg.info.(*packetInfo)
	}
	pi.parseAttrs(log, event.Attributes)
	if event.Type != chantypes.EventTypeWriteAck {
		msg.eventType = event.Type
	}
	return msg
}

// clientInfo contains the consensus height of the counterparty chain for a client.
type clientInfo struct {
	clientID        string
	consensusHeight clienttypes.Height
	header          []byte
}

func (c clientInfo) ClientState() provider.ClientState {
	return provider.ClientState{
		ClientID:        c.clientID,
		ConsensusHeight: c.consensusHeight,
	}
}

func (res *clientInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("client_id", res.clientID)
	enc.AddUint64("consensus_height", res.consensusHeight.RevisionHeight)
	enc.AddUint64("consensus_height_revision", res.consensusHeight.RevisionNumber)
	return nil
}

func (res *clientInfo) parseAttrs(log *zap.Logger, attributes []sdk.Attribute) {
	for _, attr := range attributes {
		res.parseClientAttribute(log, attr)
	}
}

func (res *clientInfo) parseClientAttribute(log *zap.Logger, attr sdk.Attribute) {
	switch attr.Key {
	case clienttypes.AttributeKeyClientID:
		res.clientID = attr.Value
	case clienttypes.AttributeKeyConsensusHeight:
		revisionSplit := strings.Split(attr.Value, "-")
		if len(revisionSplit) != 2 {
			log.Error("Error parsing client consensus height",
				zap.String("client_id", res.clientID),
				zap.String("value", attr.Value),
			)
			return
		}
		revisionNumberString := revisionSplit[0]
		revisionNumber, err := strconv.ParseUint(revisionNumberString, 10, 64)
		if err != nil {
			log.Error("Error parsing client consensus height revision number",
				zap.Error(err),
			)
			return
		}
		revisionHeightString := revisionSplit[1]
		revisionHeight, err := strconv.ParseUint(revisionHeightString, 10, 64)
		if err != nil {
			log.Error("Error parsing client consensus height revision height",
				zap.Error(err),
			)
			return
		}
		res.consensusHeight = clienttypes.Height{
			RevisionNumber: revisionNumber,
			RevisionHeight: revisionHeight,
		}
	case clienttypes.AttributeKeyHeader:
		data, err := hex.DecodeString(attr.Value)
		if err != nil {
			log.Error("Error parsing client header",
				zap.String("header", attr.Value),
				zap.Error(err),
			)
			return
		}
		res.header = data
	}
}

// alias type to the provider types, used for adding parser methods
type packetInfo provider.PacketInfo

func (res *packetInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddUint64("sequence", res.Sequence)
	enc.AddString("src_channel", res.SourceChannel)
	enc.AddString("src_port", res.SourcePort)
	enc.AddString("dst_channel", res.DestChannel)
	enc.AddString("dst_port", res.DestPort)
	return nil
}

// parsePacketInfo is treated differently from the others since it can be constructed from the accumulation of multiple events
func (res *packetInfo) parseAttrs(log *zap.Logger, attrs []sdk.Attribute) {
	for _, attr := range attrs {
		res.parsePacketAttribute(log, attr)
	}
}

func (res *packetInfo) parsePacketAttribute(log *zap.Logger, attr sdk.Attribute) {
	var err error
	switch attr.Key {
	case chantypes.AttributeKeySequence:
		res.Sequence, err = strconv.ParseUint(attr.Value, 10, 64)
		if err != nil {
			log.Error("Error parsing packet sequence",
				zap.String("value", attr.Value),
				zap.Error(err),
			)
			return
		}
	case chantypes.AttributeKeyTimeoutTimestamp:
		res.TimeoutTimestamp, err = strconv.ParseUint(attr.Value, 10, 64)
		if err != nil {
			log.Error("Error parsing packet timestamp",
				zap.Uint64("sequence", res.Sequence),
				zap.String("value", attr.Value),
				zap.Error(err),
			)
			return
		}
	// NOTE: deprecated per IBC spec
	case chantypes.AttributeKeyData:
		res.Data = []byte(attr.Value)
	case chantypes.AttributeKeyDataHex:
		data, err := hex.DecodeString(attr.Value)
		if err != nil {
			log.Error("Error parsing packet data",
				zap.Uint64("sequence", res.Sequence),
				zap.Error(err),
			)
			return
		}
		res.Data = data
	// NOTE: deprecated per IBC spec
	case chantypes.AttributeKeyAck:
		res.Ack = []byte(attr.Value)
	case chantypes.AttributeKeyAckHex:
		data, err := hex.DecodeString(attr.Value)
		if err != nil {
			log.Error("Error parsing packet ack",
				zap.Uint64("sequence", res.Sequence),
				zap.String("value", attr.Value),
				zap.Error(err),
			)
			return
		}
		res.Ack = data
	case chantypes.AttributeKeyTimeoutHeight:
		timeoutSplit := strings.Split(attr.Value, "-")
		if len(timeoutSplit) != 2 {
			log.Error("Error parsing packet height timeout",
				zap.Uint64("sequence", res.Sequence),
				zap.String("value", attr.Value),
			)
			return
		}
		revisionNumber, err := strconv.ParseUint(timeoutSplit[0], 10, 64)
		if err != nil {
			log.Error("Error parsing packet timeout height revision number",
				zap.Uint64("sequence", res.Sequence),
				zap.String("value", timeoutSplit[0]),
				zap.Error(err),
			)
			return
		}
		revisionHeight, err := strconv.ParseUint(timeoutSplit[1], 10, 64)
		if err != nil {
			log.Error("Error parsing packet timeout height revision height",
				zap.Uint64("sequence", res.Sequence),
				zap.String("value", timeoutSplit[1]),
				zap.Error(err),
			)
			return
		}
		res.TimeoutHeight = clienttypes.Height{
			RevisionNumber: revisionNumber,
			RevisionHeight: revisionHeight,
		}
	case chantypes.AttributeKeySrcPort:
		res.SourcePort = attr.Value
	case chantypes.AttributeKeySrcChannel:
		res.SourceChannel = attr.Value
	case chantypes.AttributeKeyDstPort:
		res.DestPort = attr.Value
	case chantypes.AttributeKeyDstChannel:
		res.DestChannel = attr.Value
	case chantypes.AttributeKeyChannelOrdering:
		res.ChannelOrder = attr.Value
	}
}

// alias type to the provider types, used for adding parser methods
type channelInfo provider.ChannelInfo

func (res *channelInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("channel_id", res.ChannelID)
	enc.AddString("port_id", res.PortID)
	enc.AddString("counterparty_channel_id", res.CounterpartyChannelID)
	enc.AddString("counterparty_port_id", res.CounterpartyPortID)
	return nil
}

func (res *channelInfo) parseAttrs(log *zap.Logger, attrs []sdk.Attribute) {
	for _, attr := range attrs {
		res.parseChannelAttribute(attr)
	}
}

// parseChannelAttribute parses channel attributes from an event.
// If the attribute has already been parsed into the channelInfo,
// it will not overwrite, and return true to inform the caller that
// the attribute already exists.
func (res *channelInfo) parseChannelAttribute(attr sdk.Attribute) {
	switch attr.Key {
	case chantypes.AttributeKeyPortID:
		res.PortID = attr.Value
	case chantypes.AttributeKeyChannelID:
		res.ChannelID = attr.Value
	case chantypes.AttributeCounterpartyPortID:
		res.CounterpartyPortID = attr.Value
	case chantypes.AttributeCounterpartyChannelID:
		res.CounterpartyChannelID = attr.Value
	case chantypes.AttributeKeyConnectionID:
		res.ConnID = attr.Value
	case chantypes.AttributeVersion:
		res.Version = attr.Value
	}
}

// alias type to the provider types, used for adding parser methods
type connectionInfo provider.ConnectionInfo

func (res *connectionInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("connection_id", res.ConnID)
	enc.AddString("client_id", res.ClientID)
	enc.AddString("counterparty_connection_id", res.CounterpartyConnID)
	enc.AddString("counterparty_client_id", res.CounterpartyClientID)
	return nil
}

func (res *connectionInfo) parseAttrs(log *zap.Logger, attrs []sdk.Attribute) {
	for _, attr := range attrs {
		res.parseConnectionAttribute(attr)
	}
}

func (res *connectionInfo) parseConnectionAttribute(attr sdk.Attribute) {
	switch attr.Key {
	case conntypes.AttributeKeyConnectionID:
		res.ConnID = attr.Value
	case conntypes.AttributeKeyClientID:
		res.ClientID = attr.Value
	case conntypes.AttributeKeyCounterpartyConnectionID:
		res.CounterpartyConnID = attr.Value
	case conntypes.AttributeKeyCounterpartyClientID:
		res.CounterpartyClientID = attr.Value
	}
}
