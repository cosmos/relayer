package penumbra

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v5/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/cosmos/relayer/v2/relayer/provider/penumbra"
	"go.uber.org/zap"
)

// ibcMessage is the type used for parsing all possible properties of IBC messages
type ibcMessage struct {
	eventType string
	info      ibcMessageInfo
}

type ibcMessageInfo interface {
	parseAttrs(log *zap.Logger, attrs []penumbra.EventAttribute)
}

// ibcMessagesFromTransaction parses all events within a transaction to find IBC messages
func (pcp *PenumbraChainProcessor) ibcMessagesFromTransaction(tx *penumbra.ExecTxResult, height uint64) []ibcMessage {
	parsedEvents, err := pcp.parsePenumbraIBCEvents(tx.Events, height)
	if err != nil {
		pcp.log.Info("Failed to parse tx events", zap.Error(err))
		return nil
	}
	return parsedEvents
}

func (pcp *PenumbraChainProcessor) parsePenumbraIBCEvents(events []penumbra.Event, height uint64) ([]ibcMessage, error) {
	fmt.Println("parsing events for height", height)
	var messages []ibcMessage
	for _, ev := range events {
		var info ibcMessageInfo
		var packetAccumulator *packetInfo
		eventType := ev.Type

		switch ev.Type {
		case clienttypes.EventTypeCreateClient, clienttypes.EventTypeUpdateClient,
			clienttypes.EventTypeUpgradeClient, clienttypes.EventTypeSubmitMisbehaviour,
			clienttypes.EventTypeUpdateClientProposal:
			clientInfo := new(clientInfo)
			clientInfo.parseAttrs(pcp.log, ev.Attributes)
			info = clientInfo
		case chantypes.EventTypeSendPacket, chantypes.EventTypeRecvPacket,
			chantypes.EventTypeAcknowledgePacket, chantypes.EventTypeTimeoutPacket,
			chantypes.EventTypeTimeoutPacketOnClose, chantypes.EventTypeWriteAck:
			if packetAccumulator == nil {
				packetAccumulator = &packetInfo{Height: height}
			}
			packetAccumulator.parseAttrs(pcp.log, ev.Attributes)
			info = packetAccumulator
		case conntypes.EventTypeConnectionOpenInit, conntypes.EventTypeConnectionOpenTry,
			conntypes.EventTypeConnectionOpenAck, conntypes.EventTypeConnectionOpenConfirm:
			connectionInfo := &connectionInfo{Height: height}
			connectionInfo.parseAttrs(pcp.log, ev.Attributes)
			info = connectionInfo
		case chantypes.EventTypeChannelOpenInit, chantypes.EventTypeChannelOpenTry,
			chantypes.EventTypeChannelOpenAck, chantypes.EventTypeChannelOpenConfirm,
			chantypes.EventTypeChannelCloseInit, chantypes.EventTypeChannelCloseConfirm:
			channelInfo := &channelInfo{Height: height}
			channelInfo.parseAttrs(pcp.log, ev.Attributes)
			info = channelInfo
		}

		if info == nil {
			// Not an IBC message, don't need to log here
			continue
		}
		messages = append(messages, ibcMessage{
			eventType: eventType,
			info:      info,
		})
	}

	return messages, nil
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

func (res *clientInfo) parseAttrs(log *zap.Logger, attributes []penumbra.EventAttribute) {
	for _, attr := range attributes {
		res.parseClientAttribute(log, attr)
	}
}

func (res *clientInfo) parseClientAttribute(log *zap.Logger, attr penumbra.EventAttribute) {
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

// parsePacketInfo is treated differently from the others since it can be constructed from the accumulation of multiple events
func (res *packetInfo) parseAttrs(log *zap.Logger, attrs []penumbra.EventAttribute) {
	for _, attr := range attrs {
		res.parsePacketAttribute(log, attr)
	}
}

func (res *packetInfo) parsePacketAttribute(log *zap.Logger, attr penumbra.EventAttribute) {
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
	}
}

// alias type to the provider types, used for adding parser methods
type channelInfo provider.ChannelInfo

func (res *channelInfo) parseAttrs(log *zap.Logger, attrs []penumbra.EventAttribute) {
	for _, attr := range attrs {
		res.parseChannelAttribute(attr)
	}
}

func (res *channelInfo) parseChannelAttribute(attr penumbra.EventAttribute) {
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
	}
}

// alias type to the provider types, used for adding parser methods
type connectionInfo provider.ConnectionInfo

func (res *connectionInfo) parseAttrs(log *zap.Logger, attrs []penumbra.EventAttribute) {
	for _, attr := range attrs {
		res.parseConnectionAttribute(attr)
	}
}

func (res *connectionInfo) parseConnectionAttribute(attr penumbra.EventAttribute) {
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
