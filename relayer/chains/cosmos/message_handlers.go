package cosmos

import (
	"fmt"

	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/ibc"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	"go.uber.org/zap"
)

type MsgHandlerParams struct {
	CCP           *CosmosChainProcessor
	Height        int64
	PacketInfo    *ibc.PacketInfo
	ChannelInfo   *ibc.ChannelInfo
	ClientInfo    *ibc.ClientInfo
	FoundMessages map[ibc.ChannelKey]map[string]map[uint64]provider.RelayerMessage
}

var messageHandlers = map[string]func(MsgHandlerParams){
	ibc.MsgTransfer:        handleMsgTransfer,
	ibc.MsgRecvPacket:      handleMsgRecvPacket,
	ibc.MsgAcknowledgement: handleMsgAcknowlegement,
	ibc.MsgTimeout:         handleMsgTimeout,
	ibc.MsgTimeoutOnClose:  handleMsgTimeoutOnClose,

	ibc.MsgCreateClient:       handleMsgCreateClient,
	ibc.MsgUpdateClient:       handleMsgUpdateClient,
	ibc.MsgUpgradeClient:      handleMsgUpgradeClient,
	ibc.MsgSubmitMisbehaviour: handleMsgSubmitMisbehaviour,

	ibc.MsgChannelCloseConfirm: handleMsgChannelCloseConfirm,
	ibc.MsgChannelCloseInit:    handleMsgChannelCloseInit,
	ibc.MsgChannelOpenAck:      handleMsgChannelOpenAck,
	ibc.MsgChannelOpenConfirm:  handleMsgChannelOpenConfirm,
	ibc.MsgChannelOpenInit:     handleMsgChannelOpenInit,
	ibc.MsgChannelOpenTry:      handleMsgChannelOpenTry,
}

// retains message if it is applicable to the channels for path processors that are subscribed to this chain processor
// returns true if it's applicable
func isPacketApplicable(message string, p MsgHandlerParams, channelKey ibc.ChannelKey) bool {
	if p.PacketInfo == nil || !p.CCP.IsRelevantChannel(channelKey) {
		return false
	}
	if _, ok := p.FoundMessages[channelKey]; !ok {
		p.FoundMessages[channelKey] = make(map[string]map[uint64]provider.RelayerMessage)
	}
	if _, ok := p.FoundMessages[channelKey][message]; !ok {
		p.FoundMessages[channelKey][message] = make(map[uint64]provider.RelayerMessage)
	}
	for sequence := range p.FoundMessages[channelKey][message] {
		if sequence == p.PacketInfo.Sequence {
			// already have this sequence number
			// there can be multiple MsgRecvPacket, MsgAcknowlegement, MsgTimeout, and MsgTimeoutOnClose for the same packet
			return false
		}
	}

	return true
}

func retainApplicableMessage(message string, p MsgHandlerParams, channelKey ibc.ChannelKey, ibcMsg provider.RelayerMessage) {
	p.FoundMessages[channelKey][message][p.PacketInfo.Sequence] = ibcMsg
}

// BEGIN packet msg handlers
func handleMsgTransfer(p MsgHandlerParams) {
	// source chain processor will call this handler
	// source channel used as key because MsgTransfer is sent to source chain
	channelKey := ibc.ChannelKey{
		ChannelID:             p.PacketInfo.SourceChannelID,
		PortID:                p.PacketInfo.SourcePortID,
		CounterpartyChannelID: p.PacketInfo.DestinationChannelID,
		CounterpartyPortID:    p.PacketInfo.DestinationPortID,
	}
	if !isPacketApplicable(ibc.MsgTransfer, p, channelKey) {
		return
	}
	retainApplicableMessage(ibc.MsgTransfer, p, channelKey, cosmos.NewCosmosMessage(&chantypes.MsgRecvPacket{
		Packet: chantypes.Packet{
			Sequence:           p.PacketInfo.Sequence,
			SourcePort:         p.PacketInfo.SourcePortID,
			SourceChannel:      p.PacketInfo.SourceChannelID,
			DestinationPort:    p.PacketInfo.DestinationPortID,
			DestinationChannel: p.PacketInfo.DestinationChannelID,
			Data:               p.PacketInfo.Data,
			TimeoutHeight:      p.PacketInfo.TimeoutHeight,
			TimeoutTimestamp:   p.PacketInfo.TimeoutTimestamp,
		},
	}))
	p.CCP.log.Debug("observed MsgTransfer",
		zap.String("chainID", p.CCP.ChainProvider.ChainId()),
		zap.Uint64("sequence", p.PacketInfo.Sequence),
		zap.String("srcChannel", p.PacketInfo.SourceChannelID),
		zap.String("srcPort", p.PacketInfo.SourcePortID),
		zap.String("timeoutHeight", fmt.Sprintf("%d-%d", p.PacketInfo.TimeoutHeight.RevisionNumber, p.PacketInfo.TimeoutHeight.RevisionHeight)),
		zap.Uint64("timeoutTimestamp", p.PacketInfo.TimeoutTimestamp),
	)
}

func handleMsgRecvPacket(p MsgHandlerParams) {
	// destination chain processor will call this handler
	// destination channel used because MsgRecvPacket is sent to destination chain
	channelKey := ibc.ChannelKey{
		ChannelID:             p.PacketInfo.DestinationChannelID,
		PortID:                p.PacketInfo.DestinationPortID,
		CounterpartyChannelID: p.PacketInfo.SourceChannelID,
		CounterpartyPortID:    p.PacketInfo.SourcePortID,
	}
	if !isPacketApplicable(ibc.MsgRecvPacket, p, channelKey) {
		return
	}
	retainApplicableMessage(ibc.MsgRecvPacket, p, channelKey, cosmos.NewCosmosMessage(&chantypes.MsgAcknowledgement{
		Packet: chantypes.Packet{
			Sequence:           p.PacketInfo.Sequence,
			SourcePort:         p.PacketInfo.SourcePortID,
			SourceChannel:      p.PacketInfo.SourceChannelID,
			DestinationPort:    p.PacketInfo.DestinationPortID,
			DestinationChannel: p.PacketInfo.DestinationChannelID,
			Data:               p.PacketInfo.Data,
			TimeoutHeight:      p.PacketInfo.TimeoutHeight,
			TimeoutTimestamp:   p.PacketInfo.TimeoutTimestamp,
		},
		Acknowledgement: p.PacketInfo.Ack,
	}))
	p.CCP.log.Debug("observed MsgRecvPacket",
		zap.String("chainID", p.CCP.ChainProvider.ChainId()),
		zap.Uint64("sequence", p.PacketInfo.Sequence),
		zap.String("srcChannel", p.PacketInfo.SourceChannelID),
		zap.String("srcPort", p.PacketInfo.SourcePortID),
		zap.String("dstChannel", p.PacketInfo.DestinationChannelID),
		zap.String("dstPort", p.PacketInfo.DestinationPortID),
	)
}

func handleMsgAcknowlegement(p MsgHandlerParams) {
	// source chain processor will call this handler
	// source channel used as key because MsgAcknowlegement is sent to source chain
	channelKey := ibc.ChannelKey{
		ChannelID:             p.PacketInfo.SourceChannelID,
		PortID:                p.PacketInfo.SourcePortID,
		CounterpartyChannelID: p.PacketInfo.DestinationChannelID,
		CounterpartyPortID:    p.PacketInfo.DestinationPortID,
	}
	if !isPacketApplicable(ibc.MsgAcknowledgement, p, channelKey) {
		return
	}
	retainApplicableMessage(ibc.MsgAcknowledgement, p, channelKey, nil)
	p.CCP.log.Debug("observed MsgAcknowledgement",
		zap.String("chainID", p.CCP.ChainProvider.ChainId()),
		zap.Uint64("sequence", p.PacketInfo.Sequence),
		zap.String("srcChannel", p.PacketInfo.SourceChannelID),
		zap.String("srcPort", p.PacketInfo.SourcePortID),
		zap.String("dstChannel", p.PacketInfo.DestinationChannelID),
		zap.String("dstPort", p.PacketInfo.DestinationPortID),
	)
}

func handleMsgTimeout(p MsgHandlerParams) {
	// source chain processor will call this handler
	// source channel used as key because MsgTimeout is sent to source chain
	channelKey := ibc.ChannelKey{
		ChannelID:             p.PacketInfo.SourceChannelID,
		PortID:                p.PacketInfo.SourcePortID,
		CounterpartyChannelID: p.PacketInfo.DestinationChannelID,
		CounterpartyPortID:    p.PacketInfo.DestinationPortID,
	}
	if !isPacketApplicable(ibc.MsgTimeout, p, channelKey) {
		return
	}
	retainApplicableMessage(ibc.MsgTimeout, p, channelKey, nil)
	p.CCP.log.Debug("observed MsgTimeout",
		zap.String("chainID", p.CCP.ChainProvider.ChainId()),
		zap.Uint64("sequence", p.PacketInfo.Sequence),
		zap.String("srcChannel", p.PacketInfo.SourceChannelID),
		zap.String("srcPort", p.PacketInfo.SourcePortID),
		zap.String("dstChannel", p.PacketInfo.DestinationChannelID),
		zap.String("dstPort", p.PacketInfo.DestinationPortID),
	)
}

func handleMsgTimeoutOnClose(p MsgHandlerParams) {
	// source channel used because timeout is sent to source chain
	channelKey := ibc.ChannelKey{
		ChannelID:             p.PacketInfo.SourceChannelID,
		PortID:                p.PacketInfo.SourcePortID,
		CounterpartyChannelID: p.PacketInfo.DestinationChannelID,
		CounterpartyPortID:    p.PacketInfo.DestinationPortID,
	}
	if !isPacketApplicable(ibc.MsgTimeoutOnClose, p, channelKey) {
		return
	}
	retainApplicableMessage(ibc.MsgTimeoutOnClose, p, channelKey, nil)
	p.CCP.log.Debug("observed MsgTimeoutOnClose",
		zap.String("chainID", p.CCP.ChainProvider.ChainId()),
		zap.Uint64("sequence", p.PacketInfo.Sequence),
		zap.String("srcChannel", p.PacketInfo.SourceChannelID),
		zap.String("srcPort", p.PacketInfo.SourcePortID),
		zap.String("dstChannel", p.PacketInfo.DestinationChannelID),
		zap.String("dstPort", p.PacketInfo.DestinationPortID),
	)
}

// END packet msg handlers

// BEGIN client msg handlers

func handleMsgCreateClient(p MsgHandlerParams) {
	if !handleClientInfo(p) {
		return
	}
	p.CCP.log.Debug("observed MsgCreateClient",
		zap.String("chainID", p.CCP.ChainProvider.ChainId()),
		zap.String("clientID", p.ClientInfo.ClientID),
	)
}

func handleMsgUpdateClient(p MsgHandlerParams) {
	if !handleClientInfo(p) {
		return
	}
	p.CCP.log.Debug("observed MsgUpdateClient",
		zap.String("chainID", p.CCP.ChainProvider.ChainId()),
		zap.String("clientID", p.ClientInfo.ClientID),
	)
}

func handleMsgUpgradeClient(p MsgHandlerParams) {
	if !handleClientInfo(p) {
		return
	}
	p.CCP.log.Debug("observed MsgUpgradeClient",
		zap.String("chainID", p.CCP.ChainProvider.ChainId()),
		zap.String("clientID", p.ClientInfo.ClientID),
	)
}

func handleMsgSubmitMisbehaviour(p MsgHandlerParams) {
	if !handleClientInfo(p) {
		return
	}
	p.CCP.log.Debug("observed MsgSubmitMisbehaviour",
		zap.String("chainID", p.CCP.ChainProvider.ChainId()),
		zap.String("clientID", p.ClientInfo.ClientID),
	)
}

func handleClientInfo(p MsgHandlerParams) bool {
	if p.ClientInfo == nil {
		p.CCP.log.Warn("client info is nil")
		return false
	}
	if !p.CCP.IsRelevantClient(p.ClientInfo.ClientID) {
		// p.CCP.log.Debug("irrelevant client", zap.String("clientID", p.ClientInfo.ClientID))
		return false
	}
	p.CCP.clientHeightLock.Lock()
	defer p.CCP.clientHeightLock.Unlock()
	p.CCP.clientHeight[p.ClientInfo.ClientID] = p.ClientInfo.ConsensusHeight
	return true
}

// END client msg handlers

// BEGIN channel msg handlers

func handleMsgChannelCloseConfirm(p MsgHandlerParams) {
	if p.ChannelInfo == nil || !p.CCP.IsRelevantChannel(ibc.ChannelKey{
		ChannelID:             p.ChannelInfo.ChannelID,
		PortID:                p.ChannelInfo.PortID,
		CounterpartyChannelID: p.ChannelInfo.CounterpartyChannelID,
		CounterpartyPortID:    p.ChannelInfo.CounterpartyPortID,
	}) {
		return
	}
	if !handleChannelClose(p) {
		return
	}
	// todo send msgchannelcloseack to counterparty if autocompletechannels is true or passes channelfilter
	p.CCP.log.Debug("observed MsgChannelCloseConfirm",
		zap.String("chainID", p.CCP.ChainProvider.ChainId()),
		zap.String("channelID", p.ChannelInfo.ChannelID),
		zap.String("portID", p.ChannelInfo.PortID),
		zap.String("counterpartyChannelID", p.ChannelInfo.CounterpartyChannelID),
		zap.String("counterpartyPortID", p.ChannelInfo.CounterpartyPortID),
		zap.String("connectionID", p.ChannelInfo.ConnectionID),
	)
}

func handleMsgChannelCloseInit(p MsgHandlerParams) {
	if !handleChannelClose(p) {
		return
	}
	// todo send msgchannelclosetry to counterparty if autocompletechannels is true or passes channelfilter
	p.CCP.log.Debug("observed MsgChannelCloseInit",
		zap.String("chainID", p.CCP.ChainProvider.ChainId()),
		zap.String("channelID", p.ChannelInfo.ChannelID),
		zap.String("portID", p.ChannelInfo.PortID),
		zap.String("counterpartyChannelID", p.ChannelInfo.CounterpartyChannelID),
		zap.String("counterpartyPortID", p.ChannelInfo.CounterpartyPortID),
		zap.String("connectionID", p.ChannelInfo.ConnectionID),
	)
}

func handleMsgChannelOpenAck(p MsgHandlerParams) {
	if !handleChannelOpen(p) {
		return
	}
	p.CCP.log.Debug("observed MsgChannelOpenAck",
		zap.String("chainID", p.CCP.ChainProvider.ChainId()),
		zap.String("channelID", p.ChannelInfo.ChannelID),
		zap.String("portID", p.ChannelInfo.PortID),
		zap.String("counterpartyChannelID", p.ChannelInfo.CounterpartyChannelID),
		zap.String("counterpartyPortID", p.ChannelInfo.CounterpartyPortID),
		zap.String("connectionID", p.ChannelInfo.ConnectionID),
	)
}

func handleMsgChannelOpenConfirm(p MsgHandlerParams) {
	if !handleChannelOpen(p) {
		return
	}
	// todo send MsgChannelOpenAck to counterparty if autocompletechannels is true or passes channelfilter
	p.CCP.log.Debug("observed MsgChannelOpenConfirm",
		zap.String("chainID", p.CCP.ChainProvider.ChainId()),
		zap.String("channelID", p.ChannelInfo.ChannelID),
		zap.String("portID", p.ChannelInfo.PortID),
		zap.String("counterpartyChannelID", p.ChannelInfo.CounterpartyChannelID),
		zap.String("counterpartyPortID", p.ChannelInfo.CounterpartyPortID),
		zap.String("connectionID", p.ChannelInfo.ConnectionID),
	)
}

func handleMsgChannelOpenInit(p MsgHandlerParams) {
	if p.ChannelInfo == nil || !p.CCP.IsRelevantChannel(ibc.ChannelKey{
		ChannelID:             p.ChannelInfo.ChannelID,
		PortID:                p.ChannelInfo.PortID,
		CounterpartyChannelID: p.ChannelInfo.CounterpartyChannelID,
		CounterpartyPortID:    p.ChannelInfo.CounterpartyPortID,
	}) {
		return
	}
	// TODO run opentry on counterparty if autocompletechannels is true or passes channelfilter
	p.CCP.log.Debug("observed MsgChannelOpenInit",
		zap.String("chainID", p.CCP.ChainProvider.ChainId()),
		zap.String("channelID", p.ChannelInfo.ChannelID),
		zap.String("portID", p.ChannelInfo.PortID),
		zap.String("counterpartyChannelID", p.ChannelInfo.CounterpartyChannelID),
		zap.String("counterpartyPortID", p.ChannelInfo.CounterpartyPortID),
		zap.String("connectionID", p.ChannelInfo.ConnectionID),
	)
}

func handleMsgChannelOpenTry(p MsgHandlerParams) {
	if p.ChannelInfo == nil || !p.CCP.IsRelevantChannel(ibc.ChannelKey{
		ChannelID:             p.ChannelInfo.ChannelID,
		PortID:                p.ChannelInfo.PortID,
		CounterpartyChannelID: p.ChannelInfo.CounterpartyChannelID,
		CounterpartyPortID:    p.ChannelInfo.CounterpartyPortID,
	}) {
		return
	}
	// TODO run openconfirm on counterparty if autocompletechannels is true or passes channelfilter
	p.CCP.log.Debug("observed MsgChannelOpenTry",
		zap.String("chainID", p.CCP.ChainProvider.ChainId()),
		zap.String("channelID", p.ChannelInfo.ChannelID),
		zap.String("portID", p.ChannelInfo.PortID),
		zap.String("counterpartyChannelID", p.ChannelInfo.CounterpartyChannelID),
		zap.String("counterpartyPortID", p.ChannelInfo.CounterpartyPortID),
		zap.String("connectionID", p.ChannelInfo.ConnectionID),
	)
}

// returns isRelevant
func handleChannelClose(p MsgHandlerParams) bool {
	if p.ChannelInfo == nil {
		return false
	}
	channelKey := ibc.ChannelKey{
		ChannelID:             p.ChannelInfo.ChannelID,
		PortID:                p.ChannelInfo.PortID,
		CounterpartyChannelID: p.ChannelInfo.CounterpartyChannelID,
		CounterpartyPortID:    p.ChannelInfo.CounterpartyPortID,
	}
	p.CCP.channelOpenStateLock.Lock()
	defer p.CCP.channelOpenStateLock.Unlock()
	p.CCP.channelOpenState[channelKey] = false
	return p.CCP.IsRelevantChannel(channelKey)
}

// returns isRelevant
func handleChannelOpen(p MsgHandlerParams) bool {
	if p.ChannelInfo == nil {
		return false
	}
	channelKey := ibc.ChannelKey{
		ChannelID:             p.ChannelInfo.ChannelID,
		PortID:                p.ChannelInfo.PortID,
		CounterpartyChannelID: p.ChannelInfo.CounterpartyChannelID,
		CounterpartyPortID:    p.ChannelInfo.CounterpartyPortID,
	}
	p.CCP.channelOpenStateLock.Lock()
	defer p.CCP.channelOpenStateLock.Unlock()
	p.CCP.channelOpenState[channelKey] = true
	return p.CCP.IsRelevantChannel(channelKey)
}

// END channel msg handlers

// BEGIN connection msg handlers

//TODO

// here, don't care about a allow/deny list, will do all

// on connection open init, send connection open try to counterparty
// on connection open try, send connection open confirm to counterparty
// on connection open confirm,  send connection ack confirm to counterparty

// END connection msg handlers
