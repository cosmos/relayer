package cosmos

import (
	"encoding/hex"
	"strconv"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	abci "github.com/tendermint/tendermint/abci/types"
	"go.uber.org/zap"
)

// processTransaction parses all events within a transaction to find IBC messages
func (ccp *CosmosChainProcessor) processTransaction(tx *abci.ResponseDeliverTx) []transactionMessage {
	messages := []transactionMessage{}
	parsedLogs, err := sdk.ParseABCILogs(tx.Log)
	if err != nil {
		return messages
	}
	for _, messageLog := range parsedLogs {
		var packet *packetInfo
		var channel *channelInfo
		var client *clientInfo
		var connection *connectionInfo
		var messageType string
		ibcMessageFound := false
		for _, event := range messageLog.Events {
			switch event.Type {
			case "message":
				for _, attr := range event.Attributes {
					switch attr.Key {
					case "action":
						messageType = attr.Value
					}
				}
			case "create_client", "update_client", "upgrade_client", "submit_misbehaviour":
				ibcMessageFound = true
				client = ccp.parseClientInfo(event.Attributes)
			case "send_packet", "recv_packet",
				"acknowledge_packet", "timeout_packet", "write_acknowledgement":
				ibcMessageFound = true
				packet = ccp.parsePacketInfo(event.Attributes)
			case "connection_open_init", "connection_open_try", "connection_open_ack",
				"connection_open_confirm":
				ibcMessageFound = true
				connection = ccp.parseConnectionInfo(event.Attributes)
			case "channel_open_init", "channel_open_try",
				"channel_open_ack", "channel_open_confirm", "channel_close_init", "channel_close_confirm":
				ibcMessageFound = true
				channel = ccp.parseChannelInfo(event.Attributes)
			}
		}
		if !ibcMessageFound {
			continue
		}
		messages = append(messages, transactionMessage{
			messageType:    messageType,
			packetInfo:     packet,
			channelInfo:    channel,
			clientInfo:     client,
			connectionInfo: connection,
		})
	}

	return messages
}

func (ccp *CosmosChainProcessor) parseClientInfo(attributes []sdk.Attribute) *clientInfo {
	res := clientInfo{}
	for _, attr := range attributes {
		switch attr.Key {
		case "client_id":
			res.clientID = attr.Value
		case "consensus_height":
			revisionSplit := strings.Split(attr.Value, "-")
			if len(revisionSplit) != 2 {
				ccp.log.Error("error parsing client consensus height",
					zap.String("chain_id", ccp.ChainProvider.ChainId()),
					zap.String("client_id", res.clientID),
					zap.String("value", attr.Value),
				)
				continue
			}
			revisionNumberString := revisionSplit[0]
			revisionNumber, err := strconv.ParseUint(revisionNumberString, 10, 64)
			if err != nil {
				ccp.log.Error("error parsing client consensus height revision number",
					zap.String("chain_id", ccp.ChainProvider.ChainId()),
					zap.Error(err),
				)
				continue
			}
			revisionHeightString := revisionSplit[1]
			revisionHeight, err := strconv.ParseUint(revisionHeightString, 10, 64)
			if err != nil {
				ccp.log.Error("error parsing client consensus height revision height",
					zap.String("chain_id", ccp.ChainProvider.ChainId()),
					zap.Error(err),
				)
				continue
			}
			res.consensusHeight = clienttypes.Height{
				RevisionNumber: revisionNumber,
				RevisionHeight: revisionHeight,
			}
		}
	}
	return &res
}

func (ccp *CosmosChainProcessor) parsePacketInfo(attributes []sdk.Attribute) *packetInfo {
	res := packetInfo{packet: chantypes.Packet{}}
	for _, attr := range attributes {
		var err error
		switch attr.Key {
		case "packet_sequence":
			res.packet.Sequence, err = strconv.ParseUint(attr.Value, 10, 64)
			if err != nil {
				ccp.log.Error("error parsing packet sequence",
					zap.String("chain_id", ccp.ChainProvider.ChainId()),
					zap.String("value", attr.Value),
					zap.Error(err),
				)
				continue
			}
		case "packet_timeout_timestamp":
			res.packet.TimeoutTimestamp, err = strconv.ParseUint(attr.Value, 10, 64)
			if err != nil {
				ccp.log.Error("error parsing packet timestamp",
					zap.String("chain_id", ccp.ChainProvider.ChainId()),
					zap.Uint64("sequence", res.packet.Sequence),
					zap.String("value", attr.Value),
					zap.Error(err),
				)
				continue
			}
		case "packet_data":
			res.packet.Data = []byte(attr.Value)
		case "packet_data_hex":
			data, err := hex.DecodeString(attr.Value)
			if err == nil {
				ccp.log.Error("error parsing packet data",
					zap.String("chain_id", ccp.ChainProvider.ChainId()),
					zap.Uint64("sequence", res.packet.Sequence),
					zap.Error(err),
				)
				res.packet.Data = data
			}
		case "packet_ack":
			res.ack = []byte(attr.Value)
		case "packet_ack_hex":
			data, err := hex.DecodeString(attr.Value)
			if err == nil {
				ccp.log.Error("error parsing packet ack",
					zap.String("chain_id", ccp.ChainProvider.ChainId()),
					zap.Uint64("sequence", res.packet.Sequence),
					zap.String("value", attr.Value),
					zap.Error(err),
				)
				res.ack = data
			}
		case "packet_timeout_height":
			timeoutSplit := strings.Split(attr.Value, "-")
			if len(timeoutSplit) != 2 {
				ccp.log.Error("error parsing packet height timeout",
					zap.String("chain_id", ccp.ChainProvider.ChainId()),
					zap.Uint64("sequence", res.packet.Sequence),
					zap.String("value", attr.Value),
				)
				continue
			}
			revisionNumber, err := strconv.ParseUint(timeoutSplit[0], 10, 64)
			if err != nil {
				ccp.log.Error("error parsing packet timeout height revision number",
					zap.String("chain_id", ccp.ChainProvider.ChainId()),
					zap.Uint64("sequence", res.packet.Sequence),
					zap.String("value", timeoutSplit[0]),
					zap.Error(err),
				)
				continue
			}
			revisionHeight, err := strconv.ParseUint(timeoutSplit[1], 10, 64)
			if err != nil {
				ccp.log.Error("error parsing packet timeout height revision height",
					zap.String("chain_id", ccp.ChainProvider.ChainId()),
					zap.Uint64("sequence", res.packet.Sequence),
					zap.String("value", timeoutSplit[1]),
					zap.Error(err),
				)
				continue
			}
			res.packet.TimeoutHeight = clienttypes.Height{
				RevisionNumber: revisionNumber,
				RevisionHeight: revisionHeight,
			}
		case "packet_src_port":
			res.packet.SourcePort = attr.Value
		case "packet_src_channel":
			res.packet.SourceChannel = attr.Value
		case "packet_dst_port":
			res.packet.DestinationPort = attr.Value
		case "packet_dst_channel":
			res.packet.DestinationChannel = attr.Value
		}
	}
	return &res
}

func (ccp *CosmosChainProcessor) parseChannelInfo(attributes []sdk.Attribute) *channelInfo {
	res := channelInfo{}
	for _, attr := range attributes {
		switch attr.Key {
		case "port_id":
			res.portID = attr.Value
		case "channel_id":
			res.channelID = attr.Value
		case "counterparty_port_id":
			res.counterpartyPortID = attr.Value
		case "counterparty_channel_id":
			res.counterpartyChannelID = attr.Value
		}
	}
	return &res
}

func (ccp *CosmosChainProcessor) parseConnectionInfo(attributes []sdk.Attribute) *connectionInfo {
	res := connectionInfo{}
	for _, attr := range attributes {
		switch attr.Key {
		case "connection_id":
			res.connectionID = attr.Value
		case "client_id":
			res.clientID = attr.Value
		case "counterparty_connection_id":
			res.counterpartyConnectionID = attr.Value
		case "counterparty_client_id":
			res.counterpartyClientID = attr.Value
		}
	}
	return &res
}
