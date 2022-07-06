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
)

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
		var info any
		var action string
		var packetAccumulator *provider.PacketInfo
		for _, event := range messageLog.Events {
			switch event.Type {
			case "message":
				for _, attr := range event.Attributes {
					switch attr.Key {
					case "action":
						action = attr.Value
					}
				}
			case clienttypes.EventTypeCreateClient, clienttypes.EventTypeUpdateClient,
				clienttypes.EventTypeUpgradeClient, clienttypes.EventTypeSubmitMisbehaviour,
				clienttypes.EventTypeUpdateClientProposal:
				info = parseClientInfo(log, event.Attributes)
			case chantypes.EventTypeSendPacket, chantypes.EventTypeRecvPacket,
				chantypes.EventTypeAcknowledgePacket, chantypes.EventTypeTimeoutPacket,
				chantypes.EventTypeTimeoutPacketOnClose, chantypes.EventTypeWriteAck:
				if packetAccumulator == nil {
					packetAccumulator = &provider.PacketInfo{Height: height}
				}
				parsePacketInfo(log, event.Attributes, packetAccumulator)
			case conntypes.EventTypeConnectionOpenInit, conntypes.EventTypeConnectionOpenTry,
				conntypes.EventTypeConnectionOpenAck, conntypes.EventTypeConnectionOpenConfirm:
				connectionInfo := provider.ConnectionInfo{Height: height}
				parseConnectionInfo(event.Attributes, &connectionInfo)
				info = connectionInfo
			case chantypes.EventTypeChannelOpenInit, chantypes.EventTypeChannelOpenTry,
				chantypes.EventTypeChannelOpenAck, chantypes.EventTypeChannelOpenConfirm,
				chantypes.EventTypeChannelCloseInit, chantypes.EventTypeChannelCloseConfirm:
				channelInfo := provider.ChannelInfo{Height: height}
				parseChannelInfo(event.Attributes, &channelInfo)
				info = channelInfo
			}
		}
		if packetAccumulator != nil {
			info = *packetAccumulator
		}

		if info == nil {
			// Not an IBC message, don't need to log here
			continue
		}
		if action == "" {
			log.Error("Unexpected ibc message parser state: message info is populated but action is empty",
				zap.Any("info", info),
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

func parseClientInfo(log *zap.Logger, attributes []sdk.Attribute) (res clientInfo) {
	for _, attr := range attributes {
		res = res.parseClientAttribute(log, attr)
	}
	return res
}

func (res clientInfo) parseClientAttribute(log *zap.Logger, attr sdk.Attribute) clientInfo {
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
			return res
		}
		revisionNumberString := revisionSplit[0]
		revisionNumber, err := strconv.ParseUint(revisionNumberString, 10, 64)
		if err != nil {
			log.Error("Error parsing client consensus height revision number",
				zap.Error(err),
			)
			return res
		}
		revisionHeightString := revisionSplit[1]
		revisionHeight, err := strconv.ParseUint(revisionHeightString, 10, 64)
		if err != nil {
			log.Error("Error parsing client consensus height revision height",
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
func parsePacketInfo(log *zap.Logger, attributes []sdk.Attribute, res *provider.PacketInfo) {
	for _, attr := range attributes {
		parsePacketAttribute(log, attr, res)
	}
}

func parsePacketAttribute(log *zap.Logger, attr sdk.Attribute, res *provider.PacketInfo) {
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
		res.Acknowledgement = []byte(attr.Value)
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
		res.Acknowledgement = data
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
		res.DestinationPort = attr.Value
	case chantypes.AttributeKeyDstChannel:
		res.DestinationChannel = attr.Value
	}
}

func parseChannelInfo(attributes []sdk.Attribute, res *provider.ChannelInfo) {
	for _, attr := range attributes {
		parseChannelAttribute(attr, res)
	}
}

func parseChannelAttribute(attr sdk.Attribute, res *provider.ChannelInfo) {
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
		res.ConnectionID = attr.Value
	}
}

func parseConnectionInfo(attributes []sdk.Attribute, res *provider.ConnectionInfo) {
	for _, attr := range attributes {
		parseConnectionAttribute(attr, res)
	}
}

func parseConnectionAttribute(attr sdk.Attribute, res *provider.ConnectionInfo) {
	switch attr.Key {
	case conntypes.AttributeKeyConnectionID:
		res.ConnectionID = attr.Value
	case conntypes.AttributeKeyClientID:
		res.ClientID = attr.Value
	case conntypes.AttributeKeyCounterpartyConnectionID:
		res.CounterpartyConnectionID = attr.Value
	case conntypes.AttributeKeyCounterpartyClientID:
		res.CounterpartyClientID = attr.Value
	}
}
