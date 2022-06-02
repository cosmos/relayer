package cosmos

import (
	"encoding/hex"
	"strconv"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v3/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	abci "github.com/tendermint/tendermint/abci/types"
	"go.uber.org/zap"
)

// ibcMessagesFromTransaction parses all events within a transaction to find IBC messages
func (ccp *CosmosChainProcessor) ibcMessagesFromTransaction(tx *abci.ResponseDeliverTx) []ibcMessage {
	parsedLogs, err := sdk.ParseABCILogs(tx.Log)
	if err != nil {
		ccp.log.Info("Failed to parse abci logs", zap.Error(err))
		return nil
	}
	return parseABCILogs(ccp.log, ccp.chainProvider.ChainId(), parsedLogs)
}

func parseABCILogs(log *zap.Logger, chainID string, logs sdk.ABCIMessageLogs) (messages []ibcMessage) {
	for _, messageLog := range logs {
		var messageInfo interface{}
		var messageType string
		var packetAccumulator *packetInfo
		for _, event := range messageLog.Events {
			switch event.Type {
			case "message":
				for _, attr := range event.Attributes {
					switch attr.Key {
					case "action":
						messageType = attr.Value
					}
				}
			case clienttypes.EventTypeCreateClient, clienttypes.EventTypeUpdateClient,
				clienttypes.EventTypeUpgradeClient, clienttypes.EventTypeSubmitMisbehaviour,
				clienttypes.EventTypeUpdateClientProposal:
				messageInfo = parseClientInfo(log, chainID, event.Attributes)
			case chantypes.EventTypeSendPacket, chantypes.EventTypeRecvPacket,
				chantypes.EventTypeAcknowledgePacket, chantypes.EventTypeTimeoutPacket,
				chantypes.EventTypeTimeoutPacketOnClose, chantypes.EventTypeWriteAck:
				packetAccumulator = packetAccumulator.parsePacketInfo(log, chainID, event.Attributes)
			case conntypes.EventTypeConnectionOpenInit, conntypes.EventTypeConnectionOpenTry,
				conntypes.EventTypeConnectionOpenAck, conntypes.EventTypeConnectionOpenConfirm:
				messageInfo = parseConnectionInfo(event.Attributes)
			case chantypes.EventTypeChannelOpenInit, chantypes.EventTypeChannelOpenTry,
				chantypes.EventTypeChannelOpenAck, chantypes.EventTypeChannelOpenConfirm,
				chantypes.EventTypeChannelCloseInit, chantypes.EventTypeChannelCloseConfirm:
				messageInfo = parseChannelInfo(event.Attributes)
			}
		}
		if packetAccumulator != nil {
			messageInfo = *packetAccumulator
		}

		if messageInfo == nil {
			// Not an IBC message, don't need to log here
			continue
		}
		if messageType == "" {
			log.Error("Unexpected ibc message parser state: message info is populated but type is empty",
				zap.String("chain_id", chainID),
				zap.Any("message", messageInfo),
			)
			continue
		}
		messages = append(messages, ibcMessage{
			messageType: messageType,
			messageInfo: messageInfo,
		})
	}

	return messages
}

func parseClientInfo(log *zap.Logger, chainID string, attributes []sdk.Attribute) (res clientInfo) {
	for _, attr := range attributes {
		res = res.parseClientAttribute(log, chainID, attr)
	}
	return res
}

func (res clientInfo) parseClientAttribute(log *zap.Logger, chainID string, attr sdk.Attribute) clientInfo {
	switch attr.Key {
	case clienttypes.AttributeKeyClientID:
		res.clientID = attr.Value
	case clienttypes.AttributeKeyConsensusHeight:
		revisionSplit := strings.Split(attr.Value, "-")
		if len(revisionSplit) != 2 {
			log.Error("Error parsing client consensus height",
				zap.String("chain_id", chainID),
				zap.String("client_id", res.clientID),
				zap.String("value", attr.Value),
			)
			return res
		}
		revisionNumberString := revisionSplit[0]
		revisionNumber, err := strconv.ParseUint(revisionNumberString, 10, 64)
		if err != nil {
			log.Error("Error parsing client consensus height revision number",
				zap.String("chain_id", chainID),
				zap.Error(err),
			)
			return res
		}
		revisionHeightString := revisionSplit[1]
		revisionHeight, err := strconv.ParseUint(revisionHeightString, 10, 64)
		if err != nil {
			log.Error("Error parsing client consensus height revision height",
				zap.String("chain_id", chainID),
				zap.Error(err),
			)
			return res
		}
		res.consensusHeight = clienttypes.Height{
			RevisionNumber: revisionNumber,
			RevisionHeight: revisionHeight,
		}
	case clienttypes.AttributeKeyHeader:
		data, err := hex.DecodeString(attr.Value)
		if err != nil {
			log.Error("Error parsing client header",
				zap.String("chain_id", chainID),
				zap.String("header", attr.Value),
				zap.Error(err),
			)
			return res
		}
		res.header = data
	}
	return res
}

// parsePacketInfo is treated differently from the others since it can be constructed from the accumulation of multiple events
func (res *packetInfo) parsePacketInfo(log *zap.Logger, chainID string, attributes []sdk.Attribute) *packetInfo {
	if res == nil {
		res = new(packetInfo)
	}
	for _, attr := range attributes {
		res = res.parsePacketAttribute(log, chainID, attr)
	}
	return res
}

func (res *packetInfo) parsePacketAttribute(log *zap.Logger, chainID string, attr sdk.Attribute) *packetInfo {
	var err error
	switch attr.Key {
	case chantypes.AttributeKeySequence:
		res.packet.Sequence, err = strconv.ParseUint(attr.Value, 10, 64)
		if err != nil {
			log.Error("Error parsing packet sequence",
				zap.String("chain_id", chainID),
				zap.String("value", attr.Value),
				zap.Error(err),
			)
			return res
		}
	case chantypes.AttributeKeyTimeoutTimestamp:
		res.packet.TimeoutTimestamp, err = strconv.ParseUint(attr.Value, 10, 64)
		if err != nil {
			log.Error("Error parsing packet timestamp",
				zap.String("chain_id", chainID),
				zap.Uint64("sequence", res.packet.Sequence),
				zap.String("value", attr.Value),
				zap.Error(err),
			)
			return res
		}
	// NOTE: deprecated per IBC spec
	case chantypes.AttributeKeyData:
		res.packet.Data = []byte(attr.Value)
	case chantypes.AttributeKeyDataHex:
		data, err := hex.DecodeString(attr.Value)
		if err != nil {
			log.Error("Error parsing packet data",
				zap.String("chain_id", chainID),
				zap.Uint64("sequence", res.packet.Sequence),
				zap.Error(err),
			)
			return res
		}
		res.packet.Data = data
	// NOTE: deprecated per IBC spec
	case chantypes.AttributeKeyAck:
		res.ack = []byte(attr.Value)
	case chantypes.AttributeKeyAckHex:
		data, err := hex.DecodeString(attr.Value)
		if err != nil {
			log.Error("Error parsing packet ack",
				zap.String("chain_id", chainID),
				zap.Uint64("sequence", res.packet.Sequence),
				zap.String("value", attr.Value),
				zap.Error(err),
			)
			return res
		}
		res.ack = data
	case chantypes.AttributeKeyTimeoutHeight:
		timeoutSplit := strings.Split(attr.Value, "-")
		if len(timeoutSplit) != 2 {
			log.Error("Error parsing packet height timeout",
				zap.String("chain_id", chainID),
				zap.Uint64("sequence", res.packet.Sequence),
				zap.String("value", attr.Value),
			)
			return res
		}
		revisionNumber, err := strconv.ParseUint(timeoutSplit[0], 10, 64)
		if err != nil {
			log.Error("Error parsing packet timeout height revision number",
				zap.String("chain_id", chainID),
				zap.Uint64("sequence", res.packet.Sequence),
				zap.String("value", timeoutSplit[0]),
				zap.Error(err),
			)
			return res
		}
		revisionHeight, err := strconv.ParseUint(timeoutSplit[1], 10, 64)
		if err != nil {
			log.Error("Error parsing packet timeout height revision height",
				zap.String("chain_id", chainID),
				zap.Uint64("sequence", res.packet.Sequence),
				zap.String("value", timeoutSplit[1]),
				zap.Error(err),
			)
			return res
		}
		res.packet.TimeoutHeight = clienttypes.Height{
			RevisionNumber: revisionNumber,
			RevisionHeight: revisionHeight,
		}
	case chantypes.AttributeKeySrcPort:
		res.packet.SourcePort = attr.Value
	case chantypes.AttributeKeySrcChannel:
		res.packet.SourceChannel = attr.Value
	case chantypes.AttributeKeyDstPort:
		res.packet.DestinationPort = attr.Value
	case chantypes.AttributeKeyDstChannel:
		res.packet.DestinationChannel = attr.Value
	case chantypes.AttributeKeyChannelOrdering:
		res.channelOrdering = attr.Value
	case chantypes.AttributeKeyConnection:
		res.connectionID = attr.Value
	}
	return res
}

func parseChannelInfo(attributes []sdk.Attribute) (res channelInfo) {
	for _, attr := range attributes {
		res = res.parseChannelAttribute(attr)
	}
	return res
}

func (res channelInfo) parseChannelAttribute(attr sdk.Attribute) channelInfo {
	switch attr.Key {
	case chantypes.AttributeKeyPortID:
		res.portID = attr.Value
	case chantypes.AttributeKeyChannelID:
		res.channelID = attr.Value
	case chantypes.AttributeCounterpartyPortID:
		res.counterpartyPortID = attr.Value
	case chantypes.AttributeCounterpartyChannelID:
		res.counterpartyChannelID = attr.Value
	case chantypes.AttributeKeyConnectionID:
		res.connectionID = attr.Value
	}
	return res
}

func parseConnectionInfo(attributes []sdk.Attribute) (res connectionInfo) {
	for _, attr := range attributes {
		res = res.parseConnectionAttribute(attr)
	}
	return res
}

func (res connectionInfo) parseConnectionAttribute(attr sdk.Attribute) connectionInfo {
	switch attr.Key {
	case conntypes.AttributeKeyConnectionID:
		res.connectionID = attr.Value
	case conntypes.AttributeKeyClientID:
		res.clientID = attr.Value
	case conntypes.AttributeKeyCounterpartyConnectionID:
		res.counterpartyConnectionID = attr.Value
	case conntypes.AttributeKeyCounterpartyClientID:
		res.counterpartyClientID = attr.Value
	}
	return res
}
