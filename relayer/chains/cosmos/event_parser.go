package cosmos

import (
	"encoding/hex"
	"strconv"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v3/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	abci "github.com/tendermint/tendermint/abci/types"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ibcMessage is the type used for parsing all possible properties of IBC messages
type ibcMessage struct {
	action string
	info   ibcMessageInfo
}

type ibcMessageInfo interface {
	parseAttrs(log *zap.Logger, attrs []sdk.Attribute)
	MarshalLogObject(enc zapcore.ObjectEncoder) error
}

// ibcMessagesFromTransaction parses all events within a transaction to find IBC messages
func (ccp *CosmosChainProcessor) ibcMessagesFromTransaction(tx *abci.ResponseDeliverTx, height uint64) []ibcMessage {
	parsedLogs, err := sdk.ParseABCILogs(tx.Log)
	if err != nil {
		ccp.log.Info("Failed to parse abci logs", zap.Error(err))
		return nil
	}
	return parseABCILogs(ccp.log, parsedLogs, height)
}

func parseABCILogs(log *zap.Logger, logs sdk.ABCIMessageLogs, height uint64) (messages []ibcMessage) {
	for _, messageLog := range logs {
		var info ibcMessageInfo
		var action string
		var packetAccumulator *packetInfo
		for _, event := range messageLog.Events {
			switch event.Type {
			case "message":
				for _, attr := range event.Attributes {
					if attr.Key == "action" {
						action = attr.Value
						break
					}
				}
			case clienttypes.EventTypeCreateClient, clienttypes.EventTypeUpdateClient,
				clienttypes.EventTypeUpgradeClient, clienttypes.EventTypeSubmitMisbehaviour,
				clienttypes.EventTypeUpdateClientProposal:
				clientInfo := new(clientInfo)
				clientInfo.parseAttrs(log, event.Attributes)
				info = clientInfo
			case chantypes.EventTypeSendPacket, chantypes.EventTypeRecvPacket,
				chantypes.EventTypeAcknowledgePacket, chantypes.EventTypeTimeoutPacket,
				chantypes.EventTypeTimeoutPacketOnClose, chantypes.EventTypeWriteAck:
				if packetAccumulator == nil {
					packetAccumulator = &packetInfo{Height: height}
				}
				packetAccumulator.parseAttrs(log, event.Attributes)
				info = packetAccumulator
			case conntypes.EventTypeConnectionOpenInit, conntypes.EventTypeConnectionOpenTry,
				conntypes.EventTypeConnectionOpenAck, conntypes.EventTypeConnectionOpenConfirm:
				connectionInfo := &connectionInfo{Height: height}
				connectionInfo.parseAttrs(log, event.Attributes)
				info = connectionInfo
			case chantypes.EventTypeChannelOpenInit, chantypes.EventTypeChannelOpenTry,
				chantypes.EventTypeChannelOpenAck, chantypes.EventTypeChannelOpenConfirm,
				chantypes.EventTypeChannelCloseInit, chantypes.EventTypeChannelCloseConfirm:
				channelInfo := &channelInfo{Height: height}
				channelInfo.parseAttrs(log, event.Attributes)
				info = channelInfo
			}
		}

		if info == nil {
			// Not an IBC message, don't need to log here
			continue
		}
		if action == "" {
			log.Error("Unexpected ibc message parser state: message info is populated but action is empty",
				zap.Inline(info),
			)
			continue
		}
		messages = append(messages, ibcMessage{
			action: action,
			info:   info,
		})
	}

	return messages
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

func (c *clientInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("client_id", c.clientID)
	enc.AddUint64("consensus_height", c.consensusHeight.RevisionHeight)
	enc.AddUint64("consensus_height_revision", c.consensusHeight.RevisionNumber)
	return nil
}

func (c *clientInfo) parseAttrs(log *zap.Logger, attributes []sdk.Attribute) {
	for _, attr := range attributes {
		c.parseClientAttribute(log, attr)
	}
}

func (c *clientInfo) parseClientAttribute(log *zap.Logger, attr sdk.Attribute) {
	switch attr.Key {
	case clienttypes.AttributeKeyClientID:
		c.clientID = attr.Value
	case clienttypes.AttributeKeyConsensusHeight:
		revisionSplit := strings.Split(attr.Value, "-")
		if len(revisionSplit) != 2 {
			log.Error("Error parsing client consensus height",
				zap.String("client_id", c.clientID),
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
		c.consensusHeight = clienttypes.Height{
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
		c.header = data
	}
}

// alias type to the provider types, used for adding parser methods
type packetInfo provider.PacketInfo

func (c *packetInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddUint64("sequence", c.Sequence)
	enc.AddString("src_channel", c.SourceChannel)
	enc.AddString("src_port", c.SourcePort)
	enc.AddString("dst_channel", c.DestChannel)
	enc.AddString("dst_port", c.DestPort)
	return nil
}

// parsePacketInfo is treated differently from the others since
// it can be constructed from the accumulation of multiple events.
func (c *packetInfo) parseAttrs(log *zap.Logger, attrs []sdk.Attribute) {
	for _, attr := range attrs {
		c.parsePacketAttribute(log, attr)
	}
}

func (c *packetInfo) parsePacketAttribute(log *zap.Logger, attr sdk.Attribute) {
	var err error
	switch attr.Key {
	case chantypes.AttributeKeySequence:
		c.Sequence, err = strconv.ParseUint(attr.Value, 10, 64)
		if err != nil {
			log.Error("Error parsing packet sequence",
				zap.String("value", attr.Value),
				zap.Error(err),
			)
			return
		}
	case chantypes.AttributeKeyTimeoutTimestamp:
		c.TimeoutTimestamp, err = strconv.ParseUint(attr.Value, 10, 64)
		if err != nil {
			log.Error("Error parsing packet timestamp",
				zap.Uint64("sequence", c.Sequence),
				zap.String("value", attr.Value),
				zap.Error(err),
			)
			return
		}
	// NOTE: deprecated per IBC spec
	case chantypes.AttributeKeyData:
		c.Data = []byte(attr.Value)
	case chantypes.AttributeKeyDataHex:
		data, err := hex.DecodeString(attr.Value)
		if err != nil {
			log.Error("Error parsing packet data",
				zap.Uint64("sequence", c.Sequence),
				zap.Error(err),
			)
			return
		}
		c.Data = data
	// NOTE: deprecated per IBC spec
	case chantypes.AttributeKeyAck:
		c.Ack = []byte(attr.Value)
	case chantypes.AttributeKeyAckHex:
		data, err := hex.DecodeString(attr.Value)
		if err != nil {
			log.Error("Error parsing packet ack",
				zap.Uint64("sequence", c.Sequence),
				zap.String("value", attr.Value),
				zap.Error(err),
			)
			return
		}
		c.Ack = data
	case chantypes.AttributeKeyTimeoutHeight:
		timeoutSplit := strings.Split(attr.Value, "-")
		if len(timeoutSplit) != 2 {
			log.Error("Error parsing packet height timeout",
				zap.Uint64("sequence", c.Sequence),
				zap.String("value", attr.Value),
			)
			return
		}
		revisionNumber, err := strconv.ParseUint(timeoutSplit[0], 10, 64)
		if err != nil {
			log.Error("Error parsing packet timeout height revision number",
				zap.Uint64("sequence", c.Sequence),
				zap.String("value", timeoutSplit[0]),
				zap.Error(err),
			)
			return
		}
		revisionHeight, err := strconv.ParseUint(timeoutSplit[1], 10, 64)
		if err != nil {
			log.Error("Error parsing packet timeout height revision height",
				zap.Uint64("sequence", c.Sequence),
				zap.String("value", timeoutSplit[1]),
				zap.Error(err),
			)
			return
		}
		c.TimeoutHeight = clienttypes.Height{
			RevisionNumber: revisionNumber,
			RevisionHeight: revisionHeight,
		}
	case chantypes.AttributeKeySrcPort:
		c.SourcePort = attr.Value
	case chantypes.AttributeKeySrcChannel:
		c.SourceChannel = attr.Value
	case chantypes.AttributeKeyDstPort:
		c.DestPort = attr.Value
	case chantypes.AttributeKeyDstChannel:
		c.DestChannel = attr.Value
	}
}

// alias type to the provider types, used for adding parser methods
type channelInfo provider.ChannelInfo

func (c *channelInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("channel_id", c.ChannelID)
	enc.AddString("port_id", c.PortID)
	enc.AddString("counterparty_channel_id", c.CounterpartyChannelID)
	enc.AddString("counterparty_port_id", c.CounterpartyPortID)
	return nil
}

func (c *channelInfo) parseAttrs(log *zap.Logger, attrs []sdk.Attribute) {
	for _, attr := range attrs {
		c.parseChannelAttribute(attr)
	}
}

func (c *channelInfo) parseChannelAttribute(attr sdk.Attribute) {
	switch attr.Key {
	case chantypes.AttributeKeyPortID:
		c.PortID = attr.Value
	case chantypes.AttributeKeyChannelID:
		c.ChannelID = attr.Value
	case chantypes.AttributeCounterpartyPortID:
		c.CounterpartyPortID = attr.Value
	case chantypes.AttributeCounterpartyChannelID:
		c.CounterpartyChannelID = attr.Value
	case chantypes.AttributeKeyConnectionID:
		c.ConnID = attr.Value
	}
}

// alias type to the provider types, used for adding parser methods
type connectionInfo provider.ConnectionInfo

func (c *connectionInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("connection_id", c.ConnID)
	enc.AddString("client_id", c.ClientID)
	enc.AddString("counterparty_connection_id", c.CounterpartyConnID)
	enc.AddString("counterparty_client_id", c.CounterpartyClientID)
	return nil
}

func (c *connectionInfo) parseAttrs(log *zap.Logger, attrs []sdk.Attribute) {
	for _, attr := range attrs {
		c.parseConnectionAttribute(attr)
	}
}

func (c *connectionInfo) parseConnectionAttribute(attr sdk.Attribute) {
	switch attr.Key {
	case conntypes.AttributeKeyConnectionID:
		c.ConnID = attr.Value
	case conntypes.AttributeKeyClientID:
		c.ClientID = attr.Value
	case conntypes.AttributeKeyCounterpartyConnectionID:
		c.CounterpartyConnID = attr.Value
	case conntypes.AttributeKeyCounterpartyClientID:
		c.CounterpartyClientID = attr.Value
	}
}
