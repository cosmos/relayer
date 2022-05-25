package mock

import (
	"fmt"

	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/ibc"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	"go.uber.org/zap"
)

type MsgHandlerParams struct {
	mcp           *MockChainProcessor
	PacketInfo    *chantypes.Packet
	FoundMessages map[ibc.ChannelKey]map[string]map[uint64]provider.RelayerMessage
}

var messageHandlers = map[string]func(MsgHandlerParams){
	ibc.MsgTransfer:        handleMsgTransfer,
	ibc.MsgRecvPacket:      handleMsgRecvPacket,
	ibc.MsgAcknowledgement: handleMsgAcknowlegement,

	// TODO handlers for packet timeout, client, channel, and connection messages
}

func retainMessage(p MsgHandlerParams, channelKey ibc.ChannelKey, message string, sequence uint64, ibcMessage provider.RelayerMessage) {
	if _, ok := p.FoundMessages[channelKey]; !ok {
		p.FoundMessages[channelKey] = make(map[string]map[uint64]provider.RelayerMessage)
	}
	if _, ok := p.FoundMessages[channelKey][message]; !ok {
		p.FoundMessages[channelKey][message] = make(map[uint64]provider.RelayerMessage)
	}
	p.FoundMessages[channelKey][message][sequence] = ibcMessage
}

func handleMsgTransfer(p MsgHandlerParams) {
	channelKey := ibc.ChannelKey{
		ChannelID:             p.PacketInfo.SourceChannel,
		PortID:                p.PacketInfo.SourcePort,
		CounterpartyChannelID: p.PacketInfo.DestinationChannel,
		CounterpartyPortID:    p.PacketInfo.DestinationPort,
	}
	retainMessage(p, channelKey, ibc.MsgTransfer, p.PacketInfo.Sequence, cosmos.NewCosmosMessage(&chantypes.MsgRecvPacket{Packet: *p.PacketInfo}))
	p.mcp.log.Debug("observed MsgTransfer",
		zap.String("chainID", p.mcp.chainID),
		zap.Uint64("sequence", p.PacketInfo.Sequence),
		zap.String("srcChannel", p.PacketInfo.SourceChannel),
		zap.String("srcPort", p.PacketInfo.SourcePort),
		zap.String("timeoutHeight", fmt.Sprintf("%d-%d", p.PacketInfo.TimeoutHeight.RevisionNumber, p.PacketInfo.TimeoutHeight.RevisionHeight)),
		zap.Uint64("timeoutTimestamp", p.PacketInfo.TimeoutTimestamp),
	)
}

func handleMsgRecvPacket(p MsgHandlerParams) {
	channelKey := ibc.ChannelKey{
		ChannelID:             p.PacketInfo.DestinationChannel,
		PortID:                p.PacketInfo.DestinationPort,
		CounterpartyChannelID: p.PacketInfo.SourceChannel,
		CounterpartyPortID:    p.PacketInfo.SourcePort,
	}
	retainMessage(p, channelKey, ibc.MsgRecvPacket, p.PacketInfo.Sequence, cosmos.NewCosmosMessage(&chantypes.MsgAcknowledgement{Packet: *p.PacketInfo}))
	p.mcp.log.Debug("observed MsgRecvPacket",
		zap.String("chainID", p.mcp.chainID),
		zap.Uint64("sequence", p.PacketInfo.Sequence),
		zap.String("srcChannel", p.PacketInfo.SourceChannel),
		zap.String("srcPort", p.PacketInfo.SourcePort),
		zap.String("dstChannel", p.PacketInfo.DestinationChannel),
		zap.String("dstPort", p.PacketInfo.DestinationPort),
	)
}

func handleMsgAcknowlegement(p MsgHandlerParams) {
	channelKey := ibc.ChannelKey{
		ChannelID:             p.PacketInfo.SourceChannel,
		PortID:                p.PacketInfo.SourcePort,
		CounterpartyChannelID: p.PacketInfo.DestinationChannel,
		CounterpartyPortID:    p.PacketInfo.DestinationPort,
	}
	retainMessage(p, channelKey, ibc.MsgAcknowledgement, p.PacketInfo.Sequence, nil)

	p.mcp.log.Debug("observed MsgAcknowledgement",
		zap.String("chainID", p.mcp.chainID),
		zap.Uint64("sequence", p.PacketInfo.Sequence),
		zap.String("srcChannel", p.PacketInfo.SourceChannel),
		zap.String("srcPort", p.PacketInfo.SourcePort),
		zap.String("dstChannel", p.PacketInfo.DestinationChannel),
		zap.String("dstPort", p.PacketInfo.DestinationPort),
	)
}
