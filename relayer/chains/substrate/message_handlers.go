package substrate

import (
	"github.com/cosmos/relayer/v2/relayer/processor"
	"go.uber.org/zap"
)

func (scp *SubstrateChainProcessor) logObservedIBCMessage(m string, fields ...zap.Field) {
	scp.log.With(zap.String("event_type", m)).Debug("Observed IBC message", fields...)
}

func (scp *SubstrateChainProcessor) handleClientMessage(eventType string, ci clientInfo) {
	scp.latestClientState.update(ci)
	scp.logObservedIBCMessage(eventType, zap.String("client_id", ci.ClientID))
}

func (scp *SubstrateChainProcessor) handleMessage(m ibcMessage, c processor.IBCMessagesCache) {
	switch t := m.info.(type) {
	case *clientInfo:
		scp.handleClientMessage(m.eventType, *t)
	}
}
