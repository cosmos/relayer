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
func (ccp *CosmosChainProcessor) processTransaction(tx *abci.ResponseDeliverTx) []TransactionMessage {
	messages := []TransactionMessage{}
	parsedLogs, err := sdk.ParseABCILogs(tx.Log)
	if err != nil {
		return messages
	}
	for _, messageLog := range parsedLogs {
		var packetInfo *PacketInfo
		var channelInfo *ChannelInfo
		var clientInfo *ClientInfo
		var action string
		ibcMessageFound := false
		for _, event := range messageLog.Events {
			switch event.Type {
			case "message":
				for _, attr := range event.Attributes {
					switch attr.Key {
					case "action":
						action = attr.Value
					}
				}
			case "create_client", "update_client", "upgrade_client", "submit_misbehaviour":
				ibcMessageFound = true
				clientInfo = ccp.parseClientInfo(event.Attributes)
			case "send_packet", "recv_packet",
				"acknowledge_packet", "timeout_packet", "write_acknowledgement":
				ibcMessageFound = true
				packetInfo = ccp.parsePacketInfo(event.Attributes)
			case "connection_open_init", "connection_open_try", "connection_open_ack",
				"connection_open_confirm", "channel_open_init", "channel_open_try",
				"channel_open_ack", "channel_open_confirm", "channel_close_init":
				ibcMessageFound = true
				channelInfo = ccp.parseChannelInfo(event.Attributes)
			}
		}
		if !ibcMessageFound {
			continue
		}
		messages = append(messages, TransactionMessage{
			Action:      action,
			PacketInfo:  packetInfo,
			ChannelInfo: channelInfo,
			ClientInfo:  clientInfo,
		})
	}

	return messages
}

func (ccp *CosmosChainProcessor) parseClientInfo(attributes []sdk.Attribute) *ClientInfo {
	clientInfo := ClientInfo{}
	for _, attr := range attributes {
		switch attr.Key {
		case "client_id":
			clientInfo.ClientID = attr.Value
		case "consensus_height":
			revisionSplit := strings.Split(attr.Value, "-")
			if len(revisionSplit) != 2 {
				ccp.log.Error("error parsing client consensus height",
					zap.String("chain_id", ccp.ChainProvider.ChainId()),
					zap.String("client_id", clientInfo.ClientID),
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
			clientInfo.ConsensusHeight = clienttypes.Height{
				RevisionNumber: revisionNumber,
				RevisionHeight: revisionHeight,
			}
		}
	}
	return &clientInfo
}

func (ccp *CosmosChainProcessor) parsePacketInfo(attributes []sdk.Attribute) *PacketInfo {
	packetInfo := PacketInfo{Packet: chantypes.Packet{}}
	for _, attr := range attributes {
		var err error
		switch attr.Key {
		case "packet_sequence":
			packetInfo.Packet.Sequence, err = strconv.ParseUint(attr.Value, 10, 64)
			if err != nil {
				ccp.log.Error("error parsing packet sequence",
					zap.String("chain_id", ccp.ChainProvider.ChainId()),
					zap.String("value", attr.Value),
					zap.Error(err),
				)
				continue
			}
		case "packet_timeout_timestamp":
			packetInfo.Packet.TimeoutTimestamp, err = strconv.ParseUint(attr.Value, 10, 64)
			if err != nil {
				ccp.log.Error("error parsing packet timestamp",
					zap.String("chain_id", ccp.ChainProvider.ChainId()),
					zap.Uint64("sequence", packetInfo.Packet.Sequence),
					zap.String("value", attr.Value),
					zap.Error(err),
				)
				continue
			}
		case "packet_data":
			packetInfo.Packet.Data = []byte(attr.Value)
		case "packet_data_hex":
			data, err := hex.DecodeString(attr.Value)
			if err == nil {
				ccp.log.Error("error parsing packet data",
					zap.String("chain_id", ccp.ChainProvider.ChainId()),
					zap.Uint64("sequence", packetInfo.Packet.Sequence),
					zap.Error(err),
				)
				packetInfo.Packet.Data = data
			}
		case "packet_ack":
			packetInfo.Ack = []byte(attr.Value)
		case "packet_ack_hex":
			data, err := hex.DecodeString(attr.Value)
			if err == nil {
				ccp.log.Error("error parsing packet ack",
					zap.String("chain_id", ccp.ChainProvider.ChainId()),
					zap.Uint64("sequence", packetInfo.Packet.Sequence),
					zap.String("value", attr.Value),
					zap.Error(err),
				)
				packetInfo.Ack = data
			}
		case "packet_timeout_height":
			timeoutSplit := strings.Split(attr.Value, "-")
			if len(timeoutSplit) != 2 {
				ccp.log.Error("error parsing packet height timeout",
					zap.String("chain_id", ccp.ChainProvider.ChainId()),
					zap.Uint64("sequence", packetInfo.Packet.Sequence),
					zap.String("value", attr.Value),
				)
				continue
			}
			revisionNumber, err := strconv.ParseUint(timeoutSplit[0], 10, 64)
			if err != nil {
				ccp.log.Error("error parsing packet timeout height revision number",
					zap.String("chain_id", ccp.ChainProvider.ChainId()),
					zap.Uint64("sequence", packetInfo.Packet.Sequence),
					zap.String("value", timeoutSplit[0]),
					zap.Error(err),
				)
				continue
			}
			revisionHeight, err := strconv.ParseUint(timeoutSplit[1], 10, 64)
			if err != nil {
				ccp.log.Error("error parsing packet timeout height revision height",
					zap.String("chain_id", ccp.ChainProvider.ChainId()),
					zap.Uint64("sequence", packetInfo.Packet.Sequence),
					zap.String("value", timeoutSplit[1]),
					zap.Error(err),
				)
				continue
			}
			packetInfo.Packet.TimeoutHeight = clienttypes.Height{
				RevisionNumber: revisionNumber,
				RevisionHeight: revisionHeight,
			}
		case "packet_src_port":
			packetInfo.Packet.SourcePort = attr.Value
		case "packet_src_channel":
			packetInfo.Packet.SourceChannel = attr.Value
		case "packet_dst_port":
			packetInfo.Packet.DestinationPort = attr.Value
		case "packet_dst_channel":
			packetInfo.Packet.DestinationChannel = attr.Value
		}
	}
	return &packetInfo
}

func (ccp *CosmosChainProcessor) parseChannelInfo(attributes []sdk.Attribute) *ChannelInfo {
	channelInfo := ChannelInfo{}
	for _, attr := range attributes {
		switch attr.Key {
		case "port_id":
			channelInfo.PortID = attr.Value
		case "connection_id":
			channelInfo.ConnectionID = attr.Value
		case "client_id":
			channelInfo.ClientID = attr.Value
		case "channel_id":
			channelInfo.ChannelID = attr.Value
		case "counterparty_port_id":
			channelInfo.CounterpartyPortID = attr.Value
		case "counterparty_connection_id":
			channelInfo.CounterpartyConnectionID = attr.Value
		case "counterparty_client_id":
			channelInfo.CounterpartyClientID = attr.Value
		case "counterparty_channel_id":
			channelInfo.CounterpartyChannelID = attr.Value
		}
	}
	return &channelInfo
}
