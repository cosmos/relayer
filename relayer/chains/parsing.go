package chains

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// IbcMessage is the type used for parsing all possible properties of IBC messages
type IbcMessage struct {
	EventType string
	Info      ibcMessageInfo
}

type ibcMessageInfo interface {
	ParseAttrs(log *zap.Logger, attrs []sdk.Attribute)
	MarshalLogObject(enc zapcore.ObjectEncoder) error
}

func parseBase64Event(log *zap.Logger, event abci.Event) sdk.StringEvent {
	evt := sdk.StringEvent{Type: event.Type}
	for _, attr := range event.Attributes {
		key, err := base64.StdEncoding.DecodeString(attr.Key)
		if err != nil {
			log.Error("Failed to decode legacy key as base64", zap.String("base64", attr.Key), zap.Error(err))
			continue
		}
		value, err := base64.StdEncoding.DecodeString(attr.Value)
		if err != nil {
			log.Error("Failed to decode legacy value as base64", zap.String("base64", attr.Value), zap.Error(err))
			continue
		}
		evt.Attributes = append(evt.Attributes, sdk.Attribute{
			Key:   string(key),
			Value: string(value),
		})
	}
	return evt
}

// IbcMessagesFromEvents parses all events within a transaction to find IBC messages
func IbcMessagesFromEvents(
	log *zap.Logger,
	events []abci.Event,
	chainID string,
	height uint64,
	base64Encoded bool,
) (messages []IbcMessage) {
	for _, event := range events {
		var evt sdk.StringEvent
		if base64Encoded {
			evt = parseBase64Event(log, event)
		} else {
			evt = sdk.StringifyEvent(event)
		}
		m := parseIBCMessageFromEvent(log, evt, chainID, height)
		if m == nil || m.Info == nil {
			// Not an IBC message, don't need to log here
			continue
		}
		messages = append(messages, *m)
	}
	return messages
}

type messageInfo interface {
	ibcMessageInfo
	ParseAttrs(log *zap.Logger, attrs []sdk.Attribute)
}

func parseIBCMessageFromEvent(
	log *zap.Logger,
	event sdk.StringEvent,
	chainID string,
	height uint64,
) *IbcMessage {
	var msgInfo messageInfo
	switch event.Type {
	case chantypes.EventTypeSendPacket, chantypes.EventTypeRecvPacket, chantypes.EventTypeWriteAck,
		chantypes.EventTypeAcknowledgePacket, chantypes.EventTypeTimeoutPacket,
		chantypes.EventTypeTimeoutPacketOnClose:
		msgInfo = &PacketInfo{Height: height}
	case chantypes.EventTypeChannelOpenInit, chantypes.EventTypeChannelOpenTry,
		chantypes.EventTypeChannelOpenAck, chantypes.EventTypeChannelOpenConfirm,
		chantypes.EventTypeChannelCloseInit, chantypes.EventTypeChannelClosed, chantypes.EventTypeChannelCloseConfirm:
		msgInfo = &ChannelInfo{Height: height}
	case conntypes.EventTypeConnectionOpenInit, conntypes.EventTypeConnectionOpenTry,
		conntypes.EventTypeConnectionOpenAck, conntypes.EventTypeConnectionOpenConfirm:
		msgInfo = &ConnectionInfo{Height: height}
	case clienttypes.EventTypeCreateClient, clienttypes.EventTypeUpdateClient,
		clienttypes.EventTypeUpgradeClient, clienttypes.EventTypeSubmitMisbehaviour,
		clienttypes.EventTypeUpdateClientProposal:
		msgInfo = new(ClientInfo)
	case string(processor.ClientICQTypeRequest), string(processor.ClientICQTypeResponse):
		msgInfo = &ClientICQInfo{
			Height: height,
			Source: chainID,
		}
	default:
		return nil
	}
	msgInfo.ParseAttrs(log, event.Attributes)
	return &IbcMessage{
		EventType: event.Type,
		Info:      msgInfo,
	}
}

func (msg *IbcMessage) parseIBCPacketReceiveMessageFromEvent(
	log *zap.Logger,
	event sdk.StringEvent,
	chainID string,
	height uint64,
) *IbcMessage {
	var pi *PacketInfo
	if msg.Info == nil {
		pi = &PacketInfo{Height: height}
		msg.Info = pi
	} else {
		pi = msg.Info.(*PacketInfo)
	}
	pi.ParseAttrs(log, event.Attributes)
	if event.Type != chantypes.EventTypeWriteAck {
		msg.EventType = event.Type
	}
	return msg
}

// ClientInfo contains the consensus height of the counterparty chain for a client.
type ClientInfo struct {
	ClientID        string
	ConsensusHeight clienttypes.Height
	Header          []byte
}

func NewClientInfo(
	clientID string,
	consensusHeight clienttypes.Height,
	header []byte,
) *ClientInfo {
	return &ClientInfo{
		clientID, consensusHeight, header,
	}
}

func (c ClientInfo) ClientState(trustingPeriod time.Duration) provider.ClientState {
	return provider.ClientState{
		ClientID:        c.ClientID,
		ConsensusHeight: c.ConsensusHeight,
		TrustingPeriod:  trustingPeriod,
		Header:          c.Header,
	}
}

func (res *ClientInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("client_id", res.ClientID)
	enc.AddUint64("consensus_height", res.ConsensusHeight.RevisionHeight)
	enc.AddUint64("consensus_height_revision", res.ConsensusHeight.RevisionNumber)
	return nil
}

func (res *ClientInfo) ParseAttrs(log *zap.Logger, attributes []sdk.Attribute) {
	for _, attr := range attributes {
		res.parseClientAttribute(log, attr)
	}
}

func (res *ClientInfo) parseClientAttribute(log *zap.Logger, attr sdk.Attribute) {
	switch attr.Key {
	case clienttypes.AttributeKeyClientID:
		res.ClientID = attr.Value
	case clienttypes.AttributeKeyConsensusHeight:
		revisionSplit := strings.Split(attr.Value, "-")
		if len(revisionSplit) != 2 {
			log.Error("Error parsing client consensus height",
				zap.String("client_id", res.ClientID),
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
		res.ConsensusHeight = clienttypes.Height{
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
		res.Header = data
	}
}

// alias type to the provider types, used for adding parser methods
type PacketInfo provider.PacketInfo

func (res *PacketInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddUint64("sequence", res.Sequence)
	enc.AddString("src_channel", res.SourceChannel)
	enc.AddString("src_port", res.SourcePort)
	enc.AddString("dst_channel", res.DestChannel)
	enc.AddString("dst_port", res.DestPort)
	return nil
}

// parsePacketInfo is treated differently from the others since it can be constructed from the accumulation of multiple events
func (res *PacketInfo) ParseAttrs(log *zap.Logger, attrs []sdk.Attribute) {
	for _, attr := range attrs {
		res.parsePacketAttribute(log, attr)
	}
}

func (res *PacketInfo) parsePacketAttribute(log *zap.Logger, attr sdk.Attribute) {
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
type ChannelInfo provider.ChannelInfo

func (res *ChannelInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("channel_id", res.ChannelID)
	enc.AddString("port_id", res.PortID)
	enc.AddString("counterparty_channel_id", res.CounterpartyChannelID)
	enc.AddString("counterparty_port_id", res.CounterpartyPortID)
	return nil
}

func (res *ChannelInfo) ParseAttrs(log *zap.Logger, attrs []sdk.Attribute) {
	for _, attr := range attrs {
		res.parseChannelAttribute(attr)
	}
}

// parseChannelAttribute parses channel attributes from an event.
// If the attribute has already been parsed into the channelInfo,
// it will not overwrite, and return true to inform the caller that
// the attribute already exists.
func (res *ChannelInfo) parseChannelAttribute(attr sdk.Attribute) {
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
type ConnectionInfo provider.ConnectionInfo

func (res *ConnectionInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("connection_id", res.ConnID)
	enc.AddString("client_id", res.ClientID)
	enc.AddString("counterparty_connection_id", res.CounterpartyConnID)
	enc.AddString("counterparty_client_id", res.CounterpartyClientID)
	return nil
}

func (res *ConnectionInfo) ParseAttrs(log *zap.Logger, attrs []sdk.Attribute) {
	for _, attr := range attrs {
		res.parseConnectionAttribute(attr)
	}
}

func (res *ConnectionInfo) parseConnectionAttribute(attr sdk.Attribute) {
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

type ClientICQInfo struct {
	Source     string
	Connection string
	Chain      string
	QueryID    provider.ClientICQQueryID
	Type       string
	Request    []byte
	Height     uint64
}

func (res *ClientICQInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("connection_id", res.Connection)
	enc.AddString("chain_id", res.Chain)
	enc.AddString("query_id", string(res.QueryID))
	enc.AddString("type", res.Type)
	enc.AddString("request", hex.EncodeToString(res.Request))
	enc.AddUint64("height", res.Height)

	return nil
}

func (res *ClientICQInfo) ParseAttrs(log *zap.Logger, attrs []sdk.Attribute) {
	for _, attr := range attrs {
		if err := res.parseAttribute(attr); err != nil {
			panic(fmt.Errorf("failed to parse attributes from client ICQ message: %w", err))
		}
	}
}

func (res *ClientICQInfo) parseAttribute(attr sdk.Attribute) (err error) {
	switch attr.Key {
	case "connection_id":
		res.Connection = attr.Value
	case "chain_id":
		res.Chain = attr.Value
	case "query_id":
		res.QueryID = provider.ClientICQQueryID(attr.Value)
	case "type":
		res.Type = attr.Value
	case "request":
		res.Request, err = hex.DecodeString(attr.Value)
		if err != nil {
			return err
		}
	case "height":
		res.Height, err = strconv.ParseUint(attr.Value, 10, 64)
		if err != nil {
			return err
		}
	}
	return nil
}
