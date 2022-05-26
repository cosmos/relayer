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
	mcp           *MockChainProcessor
	PacketInfo    *chantypes.Packet
	FoundMessages processor.ChannelMessageCache
}

var messageHandlers = map[string]func(MsgHandlerParams){
	processor.MsgTransfer:        handleMsgTransfer,
	processor.MsgRecvPacket:      handleMsgRecvPacket,
	processor.MsgAcknowledgement: handleMsgAcknowlegement,

	// TODO handlers for packet timeout, client, channel, and connection messages
}

func retainMessage(p MsgHandlerParams, channelKey processor.ChannelKey, message string, sequence uint64, ibcMessage provider.RelayerMessage) {
	if _, ok := p.FoundMessages[channelKey]; !ok {
		p.FoundMessages[channelKey] = make(processor.MessageCache)
	}
	if _, ok := p.FoundMessages[channelKey][message]; !ok {
		p.FoundMessages[channelKey][message] = make(processor.SequenceCache)
	}
	p.FoundMessages[channelKey][message][sequence] = ibcMessage
}

func handleMsgTransfer(p MsgHandlerParams) {
	channelKey := processor.ChannelKey{
		ChannelID:             p.PacketInfo.SourceChannel,
		PortID:                p.PacketInfo.SourcePort,
		CounterpartyChannelID: p.PacketInfo.DestinationChannel,
		CounterpartyPortID:    p.PacketInfo.DestinationPort,
	}
	retainMessage(p, channelKey, processor.MsgTransfer, p.PacketInfo.Sequence, cosmos.NewCosmosMessage(&chantypes.MsgRecvPacket{Packet: *p.PacketInfo}))
	p.mcp.log.Debug("observed MsgTransfer",
		zap.String("chain_id", p.mcp.chainID),
		zap.Uint64("sequence", p.PacketInfo.Sequence),
		zap.String("src_channel", p.PacketInfo.SourceChannel),
		zap.String("src_port", p.PacketInfo.SourcePort),
		zap.String("timeout_height", fmt.Sprintf("%d-%d", p.PacketInfo.TimeoutHeight.RevisionNumber, p.PacketInfo.TimeoutHeight.RevisionHeight)),
		zap.Uint64("timeout_timestamp", p.PacketInfo.TimeoutTimestamp),
	)
}

func handleMsgRecvPacket(p MsgHandlerParams) {
	channelKey := processor.ChannelKey{
		ChannelID:             p.PacketInfo.DestinationChannel,
		PortID:                p.PacketInfo.DestinationPort,
		CounterpartyChannelID: p.PacketInfo.SourceChannel,
		CounterpartyPortID:    p.PacketInfo.SourcePort,
	}
	retainMessage(p, channelKey, processor.MsgRecvPacket, p.PacketInfo.Sequence, cosmos.NewCosmosMessage(&chantypes.MsgAcknowledgement{Packet: *p.PacketInfo}))
	p.mcp.log.Debug("observed MsgRecvPacket",
		zap.String("chain_id", p.mcp.chainID),
		zap.Uint64("sequence", p.PacketInfo.Sequence),
		zap.String("src_channel", p.PacketInfo.SourceChannel),
		zap.String("src_port", p.PacketInfo.SourcePort),
		zap.String("dst_channel", p.PacketInfo.DestinationChannel),
		zap.String("dst_port", p.PacketInfo.DestinationPort),
	)
}

func handleMsgAcknowlegement(p MsgHandlerParams) {
	channelKey := processor.ChannelKey{
		ChannelID:             p.PacketInfo.SourceChannel,
		PortID:                p.PacketInfo.SourcePort,
		CounterpartyChannelID: p.PacketInfo.DestinationChannel,
		CounterpartyPortID:    p.PacketInfo.DestinationPort,
	}
	retainMessage(p, channelKey, processor.MsgAcknowledgement, p.PacketInfo.Sequence, nil)

	p.mcp.log.Debug("observed MsgAcknowledgement",
		zap.String("chain_id", p.mcp.chainID),
		zap.Uint64("sequence", p.PacketInfo.Sequence),
		zap.String("src_channel", p.PacketInfo.SourceChannel),
		zap.String("src_port", p.PacketInfo.SourcePort),
		zap.String("dst_channel", p.PacketInfo.DestinationChannel),
		zap.String("dst_port", p.PacketInfo.DestinationPort),
	)
}
