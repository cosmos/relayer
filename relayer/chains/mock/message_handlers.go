package mock

import (
	"fmt"

	chantypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

type msgHandlerParams struct {
	mcp              *MockChainProcessor
	packetInfo       *chantypes.Packet
	ibcMessagesCache processor.IBCMessagesCache
}

var messageHandlers = map[string]func(msgHandlerParams){
	chantypes.EventTypeSendPacket:        handleMsgTransfer,
	chantypes.EventTypeRecvPacket:        handleMsgRecvPacket,
	chantypes.EventTypeAcknowledgePacket: handleMsgAcknowledgement,

	// TODO handlers for packet timeout, client, channel, and connection messages
}

func handleMsgTransfer(p msgHandlerParams) {
	channelKey := processor.ChannelKey{
		ChannelID:             p.packetInfo.SourceChannel,
		PortID:                p.packetInfo.SourcePort,
		CounterpartyChannelID: p.packetInfo.DestinationChannel,
		CounterpartyPortID:    p.packetInfo.DestinationPort,
	}
	p.ibcMessagesCache.PacketFlow.Retain(channelKey, chantypes.EventTypeSendPacket, provider.PacketInfo{
		Sequence:      p.packetInfo.Sequence,
		Data:          p.packetInfo.Data,
		TimeoutHeight: p.packetInfo.TimeoutHeight,
	})
	p.mcp.log.Debug("observed MsgTransfer",
		zap.String("chain_id", p.mcp.chainID),
		zap.Uint64("sequence", p.packetInfo.Sequence),
		zap.String("src_channel", p.packetInfo.SourceChannel),
		zap.String("src_port", p.packetInfo.SourcePort),
		zap.String("timeout_height", fmt.Sprintf("%d-%d", p.packetInfo.TimeoutHeight.RevisionNumber, p.packetInfo.TimeoutHeight.RevisionHeight)),
		zap.Uint64("timeout_timestamp", p.packetInfo.TimeoutTimestamp),
	)
}

func handleMsgRecvPacket(p msgHandlerParams) {
	channelKey := processor.ChannelKey{
		ChannelID:             p.packetInfo.DestinationChannel,
		PortID:                p.packetInfo.DestinationPort,
		CounterpartyChannelID: p.packetInfo.SourceChannel,
		CounterpartyPortID:    p.packetInfo.SourcePort,
	}
	p.ibcMessagesCache.PacketFlow.Retain(channelKey, chantypes.EventTypeRecvPacket, provider.PacketInfo{
		Sequence: p.packetInfo.Sequence,
		Data:     p.packetInfo.Data,
	})
	p.mcp.log.Debug("observed MsgRecvPacket",
		zap.String("chain_id", p.mcp.chainID),
		zap.Uint64("sequence", p.packetInfo.Sequence),
		zap.String("src_channel", p.packetInfo.SourceChannel),
		zap.String("src_port", p.packetInfo.SourcePort),
		zap.String("dst_channel", p.packetInfo.DestinationChannel),
		zap.String("dst_port", p.packetInfo.DestinationPort),
	)
}

func handleMsgAcknowledgement(p msgHandlerParams) {
	channelKey := processor.ChannelKey{
		ChannelID:             p.packetInfo.SourceChannel,
		PortID:                p.packetInfo.SourcePort,
		CounterpartyChannelID: p.packetInfo.DestinationChannel,
		CounterpartyPortID:    p.packetInfo.DestinationPort,
	}
	p.ibcMessagesCache.PacketFlow.Retain(channelKey, chantypes.EventTypeAcknowledgePacket, provider.PacketInfo{
		Sequence: p.packetInfo.Sequence,
		Data:     p.packetInfo.Data,
	})
	p.mcp.log.Debug("observed MsgAcknowledgement",
		zap.String("chain_id", p.mcp.chainID),
		zap.Uint64("sequence", p.packetInfo.Sequence),
		zap.String("src_channel", p.packetInfo.SourceChannel),
		zap.String("src_port", p.packetInfo.SourcePort),
		zap.String("dst_channel", p.packetInfo.DestinationChannel),
		zap.String("dst_port", p.packetInfo.DestinationPort),
	)
}
