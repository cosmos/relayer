package cosmos

import (
	conntypes "github.com/cosmos/ibc-go/v3/modules/core/03-connection/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	"go.uber.org/zap"
)

func (ccp *CosmosChainProcessor) handleMsgConnectionOpenInit(p msgHandlerParams) bool {
	ci := p.messageInfo.(*connectionInfo)
	k := ci.connectionKey()
	// Construct the start of the MsgConnectionOpenTry for the counterparty chain.
	// PathProcessor will determine if this is needed.
	// For example, if a MsgConnectionOpenTry or MsgConnectionOpenConfirm is not detected on the counterparty chain, and
	// a MsgConnectionOpenAck is not detected yet on this chain,
	// a MsgConnectionOpenTry will be sent to the counterparty chain
	// using this information with the connection init proof from this chain added.
	p.ibcMessagesCache.ConnectionHandshake.Retain(k, processor.MsgConnectionOpenInit, cosmos.NewCosmosMessage(&conntypes.MsgConnectionOpenTry{
		ClientId:             k.ClientID,
		PreviousConnectionId: k.ConnectionID,
		Counterparty: conntypes.Counterparty{
			ClientId:     k.CounterpartyClientID,
			ConnectionId: k.CounterpartyConnectionID,
		},
	}))
	ccp.connectionStateCache[k] = false
	ccp.logConnectionMessage("MsgConnectionOpenInit", ci)
	return true
}

func (ccp *CosmosChainProcessor) handleMsgConnectionOpenTry(p msgHandlerParams) bool {
	ci := p.messageInfo.(*connectionInfo)
	k := ci.connectionKey().Counterparty()
	// Construct the start of the MsgConnectionOpenAck for the counterparty chain.
	// PathProcessor will determine if this is needed.
	// For example, if a MsgConnectionOpenAck is not detected on the counterparty chain, and
	// a MsgConnectionOpenConfirm is not detected yet on this chain,
	// a MsgConnectionOpenAck will be sent to the counterparty chain
	// using this information with the connection try proof from this chain added.
	p.ibcMessagesCache.ConnectionHandshake.Retain(k, processor.MsgConnectionOpenTry, cosmos.NewCosmosMessage(&conntypes.MsgConnectionOpenAck{
		ConnectionId:             k.ConnectionID,
		CounterpartyConnectionId: k.CounterpartyConnectionID,
	}))
	ccp.connectionStateCache[k] = false
	ccp.logConnectionMessage("MsgConnectionOpenTry", ci)
	return true
}

func (ccp *CosmosChainProcessor) handleMsgConnectionOpenAck(p msgHandlerParams) bool {
	ci := p.messageInfo.(*connectionInfo)
	k := ci.connectionKey()
	// Construct the start of the MsgConnectionOpenConfirm for the counterparty chain.
	// PathProcessor will determine if this is needed.
	// For example, if a MsgConnectionOpenConfirm is not detected on the counterparty chain,
	// a MsgConnectionOpenConfirm will be sent to the counterparty chain
	// using this information with the connection ack proof from this chain added.
	p.ibcMessagesCache.ConnectionHandshake.Retain(k, processor.MsgConnectionOpenAck, cosmos.NewCosmosMessage(&conntypes.MsgConnectionOpenConfirm{
		ConnectionId: k.ConnectionID,
	}))
	ccp.connectionStateCache[k] = true
	ccp.logConnectionMessage("MsgConnectionOpenAck", ci)
	return true
}

func (ccp *CosmosChainProcessor) handleMsgConnectionOpenConfirm(p msgHandlerParams) bool {
	ci := p.messageInfo.(*connectionInfo)
	k := ci.connectionKey().Counterparty()
	// Retaining a nil message here because this is for book-keeping in the PathProcessor cache only.
	// A message does not need to be constructed for the counterparty chain after the MsgConnectionOpenConfirm is observed,
	// but we want to tell the PathProcessor that the connection handshake is complete for this sequence.
	p.ibcMessagesCache.ConnectionHandshake.Retain(k, processor.MsgConnectionOpenConfirm, nil)
	ccp.connectionStateCache[k] = true
	ccp.logConnectionMessage("MsgConnectionOpenConfirm", ci)
	return true
}

func (ccp *CosmosChainProcessor) logConnectionMessage(message string, connectionInfo *connectionInfo) {
	ccp.logObservedIBCMessage(message, zap.String("client_id", connectionInfo.clientID),
		zap.String("connection_id", connectionInfo.connectionID),
		zap.String("counterparty_client_id", connectionInfo.counterpartyClientID),
		zap.String("counterparty_connection_id", connectionInfo.counterpartyConnectionID))
}
