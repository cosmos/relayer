package cosmos

import (
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	"go.uber.org/zap"
)

// handleMsgChannelOpenInit will construct the start of the MsgChannelOpenTry
// for the counterparty chain. PathProcessor will determine if this is needed.
// For example, if a MsgChannelOpenTry or MsgChannelOpenConfirm is not detected on
// the counterparty chain, and a MsgChannelOpenAck is not detected yet on this chain,
// a MsgChannelOpenTry will be sent to the counterparty chain using this information
// with the channel open init proof from this chain added.
func (ccp *CosmosChainProcessor) handleMsgChannelOpenInit(p msgHandlerParams) bool {
	ci := p.messageInfo.(*channelInfo)
	k := ci.channelKey()
	p.ibcMessagesCache.ChannelHandshake.Retain(k, processor.MsgChannelOpenInit, cosmos.NewCosmosMessage(&chantypes.MsgChannelOpenTry{
		PortId:            k.PortID,
		PreviousChannelId: k.ChannelID,
		Channel: chantypes.Channel{
			Counterparty: chantypes.Counterparty{
				PortId:    k.CounterpartyPortID,
				ChannelId: k.CounterpartyChannelID,
			},
			ConnectionHops: []string{ci.connectionID},
		},
	}))
	// Setting false for open state because channel is not open until MsgChannelOpenAck on this chain.
	ccp.channelStateCache[k] = false
	ccp.logChannelMessage("MsgChannelOpenInit", ci)
	return true
}

// handleMsgChannelOpenTry will construct the start of the MsgChannelOpenAck
// for the counterparty chain. PathProcessor will determine if this is needed.
// For example, if a MsgChannelOpenAck is not detected on the counterparty chain, and
// a MsgChannelOpenConfirm is not detected yet on this chain, a MsgChannelOpenAck
// will be sent to the counterparty chain using this information with the
// channel open try proof from this chain added.
func (ccp *CosmosChainProcessor) handleMsgChannelOpenTry(p msgHandlerParams) bool {
	ci := p.messageInfo.(*channelInfo)
	// using flipped counterparty since counterparty initialized this handshake
	k := ci.channelKey().Counterparty()
	p.ibcMessagesCache.ChannelHandshake.Retain(k, processor.MsgChannelOpenTry, cosmos.NewCosmosMessage(&chantypes.MsgChannelOpenAck{
		PortId:                k.PortID,
		ChannelId:             k.ChannelID,
		CounterpartyChannelId: k.CounterpartyChannelID,
	}))
	// Setting false for open state because channel is not open until MsgChannelOpenConfirm on this chain.
	ccp.channelStateCache[k] = false
	ccp.logChannelMessage("MsgChannelOpenTry", ci)
	return true
}

// handleMsgChannelOpenAck will construct the start of the MsgChannelOpenConfirm
// for the counterparty chain. PathProcessor will determine if this is needed.
// For example, if a MsgChannelOpenConfirm is not detected on the counterparty chain,
// a MsgChannelOpenConfirm will be sent to the counterparty chain
// using this information with the channel open ack proof from this chain added.
func (ccp *CosmosChainProcessor) handleMsgChannelOpenAck(p msgHandlerParams) bool {
	ci := p.messageInfo.(*channelInfo)
	k := ci.channelKey()
	p.ibcMessagesCache.ChannelHandshake.Retain(k, processor.MsgChannelOpenAck, cosmos.NewCosmosMessage(&chantypes.MsgChannelOpenConfirm{
		PortId:    k.PortID,
		ChannelId: k.ChannelID,
	}))
	ccp.channelStateCache[k] = true
	ccp.logChannelMessage("MsgChannelOpenAck", ci)
	return true
}

// handleMsgChannelOpenConfirm will retain a nil message here because this is for
// book-keeping in the PathProcessor cache only. A message does not need to be constructed
// for the counterparty chain after the MsgChannelOpenConfirm is observed, but we want to
// tell the PathProcessor that the channel handshake is complete for this channel.
func (ccp *CosmosChainProcessor) handleMsgChannelOpenConfirm(p msgHandlerParams) bool {
	ci := p.messageInfo.(*channelInfo)
	// using flipped counterparty since counterparty initialized this handshake
	k := ci.channelKey().Counterparty()
	p.ibcMessagesCache.ChannelHandshake.Retain(k, processor.MsgChannelOpenConfirm, nil)
	ccp.channelStateCache[k] = true
	ccp.logChannelMessage("MsgChannelOpenConfirm", ci)
	return true
}

// handleMsgChannelCloseInit will construct the start of the MsgChannelCloseConfirm
// for the counterparty chain. PathProcessor will determine if this is needed.
// For example, if a MsgChannelCloseConfirm is not detected on the counterparty chain,
// a MsgChannelCloseConfirm will be sent to the counterparty chain
// using this information with the channel close init proof from this chain added.
func (ccp *CosmosChainProcessor) handleMsgChannelCloseInit(p msgHandlerParams) bool {
	ci := p.messageInfo.(*channelInfo)
	k := ci.channelKey()
	p.ibcMessagesCache.ChannelHandshake.Retain(k, processor.MsgChannelCloseInit, cosmos.NewCosmosMessage(&chantypes.MsgChannelCloseConfirm{
		PortId:    k.PortID,
		ChannelId: k.ChannelID,
	}))
	ccp.channelStateCache[k] = false
	ccp.logChannelMessage("MsgChannelCloseInit", ci)
	return true
}

// handleMsgChannelCloseConfirm will retain a nil message because this is for
// book-keeping in the PathProcessor cache only. A message does not need to be constructed
// for the counterparty chain after the MsgChannelCloseConfirm is observed, but we want to
// tell the PathProcessor that the channel close is complete for this channel.
func (ccp *CosmosChainProcessor) handleMsgChannelCloseConfirm(p msgHandlerParams) bool {
	ci := p.messageInfo.(*channelInfo)
	// using flipped counterparty since counterparty initialized this channel close
	k := ci.channelKey().Counterparty()
	p.ibcMessagesCache.ChannelHandshake.Retain(k, processor.MsgChannelCloseConfirm, nil)
	ccp.channelStateCache[k] = false
	ccp.logChannelMessage("MsgChannelCloseConfirm", ci)
	return true
}

func (ccp *CosmosChainProcessor) logChannelMessage(message string, channelInfo *channelInfo) {
	ccp.logObservedIBCMessage(message,
		zap.String("channel_id", channelInfo.channelID),
		zap.String("port_id", channelInfo.portID),
		zap.String("counterparty_channel_id", channelInfo.counterpartyChannelID),
		zap.String("counterparty_port_id", channelInfo.counterpartyPortID),
		zap.String("connection_id", channelInfo.connectionID),
	)
}
