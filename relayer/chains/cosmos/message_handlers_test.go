package cosmos

import (
	"os"
	"testing"

	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestHandleMsgTransfer(t *testing.T) {
	const (
		sequence         = uint64(1)
		chainID1         = "test-chain-1"
		chainID2         = "test-chain-2"
		srcChannel       = "channel-0"
		dstChannel       = "channel-1"
		srcPort          = "transfer"
		dstPort          = "transfer"
		timeoutHeight    = 100
		timeoutRevision  = 1
		timeoutTimestamp = uint64(2054566111724000000)
	)
	var (
		packetData    = []byte{0x1, 0x2, 0x3, 0x4}
		log           = zap.NewNop()
		pathEnd1      = processor.PathEnd{ChainID: chainID1}
		pathEnd2      = processor.PathEnd{ChainID: chainID2}
		pathProcessor = processor.NewPathProcessor(log, pathEnd1, pathEnd2)
		provider      = cosmos.CosmosProvider{PCfg: cosmos.CosmosProviderConfig{ChainID: chainID1}}
	)

	ccp, err := NewCosmosChainProcessor(log, &provider, "", os.Stdin, os.Stdout, []*processor.PathProcessor{pathProcessor})
	require.NoError(t, err, "error constructing cosmos chain processor")

	applicable := pathProcessor.SetChainProviderIfApplicable(&provider)
	require.True(t, applicable, "error setting path processor reference to chain processor")

	foundMessages := make(processor.ChannelMessageCache)

	packetInfo := &packetInfo{
		packet: chantypes.Packet{
			Data:               packetData,
			Sequence:           sequence,
			SourceChannel:      srcChannel,
			SourcePort:         srcPort,
			DestinationChannel: dstChannel,
			DestinationPort:    dstPort,
			TimeoutHeight: clienttypes.Height{
				RevisionHeight: timeoutHeight,
				RevisionNumber: timeoutRevision,
			},
			TimeoutTimestamp: timeoutTimestamp,
		},
	}

	ccp.handleMsgTransfer(MsgHandlerParams{messageInfo: packetInfo, foundMessages: foundMessages})

	require.Len(t, foundMessages, 1)

	channelKey := processor.ChannelKey{
		ChannelID:             srcChannel,
		PortID:                srcPort,
		CounterpartyChannelID: dstChannel,
		CounterpartyPortID:    dstPort,
	}

	channelMessages, ok := foundMessages[channelKey]
	require.True(t, ok, "unable to find messages for channel key")

	require.Len(t, channelMessages, 1)

	transferMessages, ok := channelMessages[processor.MsgTransfer]
	require.True(t, ok, "unable to find messages for MsgTransfer type")

	require.Len(t, transferMessages, 1)

	sequenceMessage, ok := transferMessages[sequence]
	require.True(t, ok, "unable to find message for sequence")

	cosmosMsg := cosmos.CosmosMsg(sequenceMessage)
	require.NotNil(t, cosmosMsg, "error parsing message as CosmosMsg")

	msgRecvPacket, ok := cosmosMsg.(*chantypes.MsgRecvPacket)
	require.True(t, ok, "unable to read message as MsgRecvPacket")

	require.Empty(t, cmp.Diff(packetInfo.packet, msgRecvPacket.Packet), "MsgRecvPacket data does not match MsgTransfer data")
}

func TestHandleMsgRecvPacket(t *testing.T) {
	const (
		sequence   = uint64(1)
		chainID1   = "test-chain-1"
		chainID2   = "test-chain-2"
		srcChannel = "channel-0"
		dstChannel = "channel-1"
		srcPort    = "transfer"
		dstPort    = "transfer"
	)
	var (
		packetData    = []byte{0x1, 0x2, 0x3, 0x4}
		packetAck     = []byte{0x2, 0x3, 0x4, 0x5}
		log           = zap.NewNop()
		pathEnd1      = processor.PathEnd{ChainID: chainID1}
		pathEnd2      = processor.PathEnd{ChainID: chainID2}
		pathProcessor = processor.NewPathProcessor(log, pathEnd1, pathEnd2)
		provider      = cosmos.CosmosProvider{PCfg: cosmos.CosmosProviderConfig{ChainID: chainID1}}
	)

	ccp, err := NewCosmosChainProcessor(log, &provider, "", os.Stdin, os.Stdout, []*processor.PathProcessor{pathProcessor})
	require.NoError(t, err, "error constructing cosmos chain processor")

	applicable := pathProcessor.SetChainProviderIfApplicable(&provider)
	require.True(t, applicable, "error setting path processor reference to chain processor")

	foundMessages := make(processor.ChannelMessageCache)

	packetInfo := &packetInfo{
		packet: chantypes.Packet{
			Data:               packetData,
			Sequence:           sequence,
			SourceChannel:      srcChannel,
			SourcePort:         srcPort,
			DestinationChannel: dstChannel,
			DestinationPort:    dstPort,
		},
		ack: packetAck,
	}

	ccp.handleMsgRecvPacket(MsgHandlerParams{messageInfo: packetInfo, foundMessages: foundMessages})

	require.Len(t, foundMessages, 1)

	// flipped on purpose since MsgRecvPacket is committed on counterparty chain
	channelKey := processor.ChannelKey{
		ChannelID:             dstChannel,
		PortID:                dstPort,
		CounterpartyChannelID: srcChannel,
		CounterpartyPortID:    srcPort,
	}

	channelMessages, ok := foundMessages[channelKey]
	require.True(t, ok, "unable to find messages for channel key")

	require.Len(t, channelMessages, 1)

	transferMessages, ok := channelMessages[processor.MsgRecvPacket]
	require.True(t, ok, "unable to find messages for MsgRecvPacket type")

	require.Len(t, transferMessages, 1)

	sequenceMessage, ok := transferMessages[sequence]
	require.True(t, ok, "unable to find message for sequence")

	cosmosMsg := cosmos.CosmosMsg(sequenceMessage)
	require.NotNil(t, cosmosMsg, "error parsing message as CosmosMsg")

	msgRecvPacket, ok := cosmosMsg.(*chantypes.MsgAcknowledgement)
	require.True(t, ok, "unable to read message as MsgAcknowledgement")

	require.Empty(t, cmp.Diff(packetInfo.packet, msgRecvPacket.Packet), "MsgAcknowledgement data does not match MsgRecvPacket data")
}

func TestHandleMsgAcknowledgement(t *testing.T) {
	const (
		sequence   = uint64(1)
		chainID1   = "test-chain-1"
		chainID2   = "test-chain-2"
		srcChannel = "channel-0"
		dstChannel = "channel-1"
		srcPort    = "transfer"
		dstPort    = "transfer"
	)
	var (
		packetData    = []byte{0x1, 0x2, 0x3, 0x4}
		log           = zap.NewNop()
		pathEnd1      = processor.PathEnd{ChainID: chainID1}
		pathEnd2      = processor.PathEnd{ChainID: chainID2}
		pathProcessor = processor.NewPathProcessor(log, pathEnd1, pathEnd2)
		provider      = cosmos.CosmosProvider{PCfg: cosmos.CosmosProviderConfig{ChainID: chainID1}}
	)

	ccp, err := NewCosmosChainProcessor(log, &provider, "", os.Stdin, os.Stdout, []*processor.PathProcessor{pathProcessor})
	require.NoError(t, err, "error constructing cosmos chain processor")

	applicable := pathProcessor.SetChainProviderIfApplicable(&provider)
	require.True(t, applicable, "error setting path processor reference to chain processor")

	foundMessages := make(processor.ChannelMessageCache)

	packetInfo := &packetInfo{
		packet: chantypes.Packet{
			Data:               packetData,
			Sequence:           sequence,
			SourceChannel:      srcChannel,
			SourcePort:         srcPort,
			DestinationChannel: dstChannel,
			DestinationPort:    dstPort,
		},
	}

	ccp.handleMsgAcknowlegement(MsgHandlerParams{messageInfo: packetInfo, foundMessages: foundMessages})

	require.Len(t, foundMessages, 1)

	channelKey := processor.ChannelKey{
		ChannelID:             srcChannel,
		PortID:                srcPort,
		CounterpartyChannelID: dstChannel,
		CounterpartyPortID:    dstPort,
	}

	channelMessages, ok := foundMessages[channelKey]
	require.True(t, ok, "unable to find messages for channel key")

	require.Len(t, channelMessages, 1)

	transferMessages, ok := channelMessages[processor.MsgAcknowledgement]
	require.True(t, ok, "unable to find messages for MsgAcknowledgement type")

	require.Len(t, transferMessages, 1)

	sequenceMessage, ok := transferMessages[sequence]
	require.True(t, ok, "unable to find message for sequence")

	require.Nil(t, sequenceMessage, "message is not nil, expected nil since no messages need to be constructed for counterparty")
}

func TestHandleMsgTimeout(t *testing.T) {
	const (
		sequence   = uint64(1)
		chainID1   = "test-chain-1"
		chainID2   = "test-chain-2"
		srcChannel = "channel-0"
		dstChannel = "channel-1"
		srcPort    = "transfer"
		dstPort    = "transfer"
	)
	var (
		packetData    = []byte{0x1, 0x2, 0x3, 0x4}
		log           = zap.NewNop()
		pathEnd1      = processor.PathEnd{ChainID: chainID1}
		pathEnd2      = processor.PathEnd{ChainID: chainID2}
		pathProcessor = processor.NewPathProcessor(log, pathEnd1, pathEnd2)
		provider      = cosmos.CosmosProvider{PCfg: cosmos.CosmosProviderConfig{ChainID: chainID1}}
	)

	ccp, err := NewCosmosChainProcessor(log, &provider, "", os.Stdin, os.Stdout, []*processor.PathProcessor{pathProcessor})
	require.NoError(t, err, "error constructing cosmos chain processor")

	applicable := pathProcessor.SetChainProviderIfApplicable(&provider)
	require.True(t, applicable, "error setting path processor reference to chain processor")

	foundMessages := make(processor.ChannelMessageCache)

	packetInfo := &packetInfo{
		packet: chantypes.Packet{
			Data:               packetData,
			Sequence:           sequence,
			SourceChannel:      srcChannel,
			SourcePort:         srcPort,
			DestinationChannel: dstChannel,
			DestinationPort:    dstPort,
		},
	}

	ccp.handleMsgTimeout(MsgHandlerParams{messageInfo: packetInfo, foundMessages: foundMessages})

	require.Len(t, foundMessages, 1)

	channelKey := processor.ChannelKey{
		ChannelID:             srcChannel,
		PortID:                srcPort,
		CounterpartyChannelID: dstChannel,
		CounterpartyPortID:    dstPort,
	}

	channelMessages, ok := foundMessages[channelKey]
	require.True(t, ok, "unable to find messages for channel key")

	require.Len(t, channelMessages, 1)

	transferMessages, ok := channelMessages[processor.MsgTimeout]
	require.True(t, ok, "unable to find messages for MsgTimeout type")

	require.Len(t, transferMessages, 1)

	sequenceMessage, ok := transferMessages[sequence]
	require.True(t, ok, "unable to find message for sequence")

	require.Nil(t, sequenceMessage, "message is not nil, expected nil since no messages need to be constructed for counterparty")
}

func TestHandleMsgTimeoutOnClose(t *testing.T) {
	const (
		sequence   = uint64(1)
		chainID1   = "test-chain-1"
		chainID2   = "test-chain-2"
		srcChannel = "channel-0"
		dstChannel = "channel-1"
		srcPort    = "transfer"
		dstPort    = "transfer"
	)
	var (
		packetData    = []byte{0x1, 0x2, 0x3, 0x4}
		log           = zap.NewNop()
		pathEnd1      = processor.PathEnd{ChainID: chainID1}
		pathEnd2      = processor.PathEnd{ChainID: chainID2}
		pathProcessor = processor.NewPathProcessor(log, pathEnd1, pathEnd2)
		provider      = cosmos.CosmosProvider{PCfg: cosmos.CosmosProviderConfig{ChainID: chainID1}}
	)

	ccp, err := NewCosmosChainProcessor(log, &provider, "", os.Stdin, os.Stdout, []*processor.PathProcessor{pathProcessor})
	require.NoError(t, err, "error constructing cosmos chain processor")

	applicable := pathProcessor.SetChainProviderIfApplicable(&provider)
	require.True(t, applicable, "error setting path processor reference to chain processor")

	foundMessages := make(processor.ChannelMessageCache)

	packetInfo := &packetInfo{
		packet: chantypes.Packet{
			Data:               packetData,
			Sequence:           sequence,
			SourceChannel:      srcChannel,
			SourcePort:         srcPort,
			DestinationChannel: dstChannel,
			DestinationPort:    dstPort,
		},
	}

	ccp.handleMsgTimeoutOnClose(MsgHandlerParams{messageInfo: packetInfo, foundMessages: foundMessages})

	require.Len(t, foundMessages, 1)

	channelKey := processor.ChannelKey{
		ChannelID:             srcChannel,
		PortID:                srcPort,
		CounterpartyChannelID: dstChannel,
		CounterpartyPortID:    dstPort,
	}

	channelMessages, ok := foundMessages[channelKey]
	require.True(t, ok, "unable to find messages for channel key")

	require.Len(t, channelMessages, 1)

	transferMessages, ok := channelMessages[processor.MsgTimeoutOnClose]
	require.True(t, ok, "unable to find messages for MsgTimeoutOnClose type")

	require.Len(t, transferMessages, 1)

	sequenceMessage, ok := transferMessages[sequence]
	require.True(t, ok, "unable to find message for sequence")

	require.Nil(t, sequenceMessage, "message is not nil, expected nil since no messages need to be constructed for counterparty")
}
