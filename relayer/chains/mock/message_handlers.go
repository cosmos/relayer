package mock

import (
	"fmt"

	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	"go.uber.org/zap"
)

type MsgHandlerParams struct {
	mcp              *MockChainProcessor
	packetInfo       *chantypes.Packet
	ibcMessagesCache processor.IBCMessagesCache
}

var messageHandlers = map[string]func(MsgHandlerParams){
	processor.MsgTransfer:        handleMsgTransfer,
	processor.MsgRecvPacket:      handleMsgRecvPacket,
	processor.MsgAcknowledgement: handleMsgAcknowledgement,

	// TODO handlers for packet timeout, client, channel, and connection messages
}

func retainMessage(p MsgHandlerParams, channelKey processor.ChannelKey, message string, sequence uint64, ibcMessage provider.RelayerMessage) {
	if _, ok := p.ibcMessagesCache.PacketFlow[channelKey]; !ok {
		p.ibcMessagesCache.PacketFlow[channelKey] = make(processor.PacketMessagesCache)
	}
	if _, ok := p.ibcMessagesCache.PacketFlow[channelKey][message]; !ok {
		p.ibcMessagesCache.PacketFlow[channelKey][message] = make(processor.PacketSequenceCache)
	}
	p.ibcMessagesCache.PacketFlow[channelKey][message][sequence] = ibcMessage
}

func handleMsgTransfer(p MsgHandlerParams) {
	channelKey := processor.ChannelKey{
		ChannelID:             p.packetInfo.SourceChannel,
		PortID:                p.packetInfo.SourcePort,
		CounterpartyChannelID: p.packetInfo.DestinationChannel,
		CounterpartyPortID:    p.packetInfo.DestinationPort,
	}
	retainMessage(p, channelKey, processor.MsgTransfer, p.packetInfo.Sequence, cosmos.NewCosmosMessage(&chantypes.MsgRecvPacket{Packet: *p.packetInfo}))
	p.mcp.log.Debug("observed MsgTransfer",
		zap.String("chain_id", p.mcp.chainID),
		zap.Uint64("sequence", p.packetInfo.Sequence),
		zap.String("src_channel", p.packetInfo.SourceChannel),
		zap.String("src_port", p.packetInfo.SourcePort),
		zap.String("timeout_height", fmt.Sprintf("%d-%d", p.packetInfo.TimeoutHeight.RevisionNumber, p.packetInfo.TimeoutHeight.RevisionHeight)),
		zap.Uint64("timeout_timestamp", p.packetInfo.TimeoutTimestamp),
	)
}

func handleMsgRecvPacket(p MsgHandlerParams) {
	channelKey := processor.ChannelKey{
		ChannelID:             p.packetInfo.DestinationChannel,
		PortID:                p.packetInfo.DestinationPort,
		CounterpartyChannelID: p.packetInfo.SourceChannel,
		CounterpartyPortID:    p.packetInfo.SourcePort,
	}
	retainMessage(p, channelKey, processor.MsgRecvPacket, p.packetInfo.Sequence, cosmos.NewCosmosMessage(&chantypes.MsgAcknowledgement{Packet: *p.packetInfo}))
	p.mcp.log.Debug("observed MsgRecvPacket",
		zap.String("chain_id", p.mcp.chainID),
		zap.Uint64("sequence", p.packetInfo.Sequence),
		zap.String("src_channel", p.packetInfo.SourceChannel),
		zap.String("src_port", p.packetInfo.SourcePort),
		zap.String("dst_channel", p.packetInfo.DestinationChannel),
		zap.String("dst_port", p.packetInfo.DestinationPort),
	)
}

func handleMsgAcknowledgement(p MsgHandlerParams) {
	channelKey := processor.ChannelKey{
		ChannelID:             p.packetInfo.SourceChannel,
		PortID:                p.packetInfo.SourcePort,
		CounterpartyChannelID: p.packetInfo.DestinationChannel,
		CounterpartyPortID:    p.packetInfo.DestinationPort,
	}
	retainMessage(p, channelKey, processor.MsgAcknowledgement, p.packetInfo.Sequence, nil)

	p.mcp.log.Debug("observed MsgAcknowledgement",
		zap.String("chain_id", p.mcp.chainID),
		zap.Uint64("sequence", p.packetInfo.Sequence),
		zap.String("src_channel", p.packetInfo.SourceChannel),
		zap.String("src_port", p.packetInfo.SourcePort),
		zap.String("dst_channel", p.packetInfo.DestinationChannel),
		zap.String("dst_port", p.packetInfo.DestinationPort),
	)
}
