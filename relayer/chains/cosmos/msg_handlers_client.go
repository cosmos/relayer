package cosmos

import (
	"go.uber.org/zap"
)

func (ccp *CosmosChainProcessor) handleMsgCreateClient(p msgHandlerParams) bool {
	clientInfo := p.messageInfo.(*clientInfo)
	// save the latest consensus height and header for this client
	ccp.latestClientState.UpdateLatestClientState(*clientInfo)
	ccp.logObservedIBCMessage("MsgCreateClient", zap.String("client_id", clientInfo.clientID))
	return false
}

func (ccp *CosmosChainProcessor) handleMsgUpdateClient(p msgHandlerParams) bool {
	clientInfo := p.messageInfo.(*clientInfo)
	// save the latest consensus height and header for this client
	ccp.latestClientState.UpdateLatestClientState(*clientInfo)
	ccp.logObservedIBCMessage("MsgUpdateClient", zap.String("client_id", clientInfo.clientID))
	return false
}

func (ccp *CosmosChainProcessor) handleMsgUpgradeClient(p msgHandlerParams) bool {
	clientInfo := p.messageInfo.(*clientInfo)
	// save the latest consensus height and header for this client
	ccp.latestClientState.UpdateLatestClientState(*clientInfo)
	ccp.logObservedIBCMessage("MsgUpgradeClient", zap.String("client_id", clientInfo.clientID))
	return false
}

func (ccp *CosmosChainProcessor) handleMsgSubmitMisbehaviour(p msgHandlerParams) bool {
	clientInfo := p.messageInfo.(*clientInfo)
	// save the latest consensus height and header for this client
	ccp.latestClientState.UpdateLatestClientState(*clientInfo)
	ccp.logObservedIBCMessage("MsgSubmitMisbehaviour", zap.String("client_id", clientInfo.clientID))
	return false
}
