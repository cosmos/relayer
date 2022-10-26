package substrate

import (
	"github.com/cosmos/relayer/v2/relayer/provider"
)

// LogFailedTx takes the transaction and the messages to create it and logs the appropriate data
func (cc *SubstrateProvider) LogFailedTx(res *provider.RelayerTxResponse, err error, msgs []provider.RelayerMessage) {
	// implement
}

// LogSuccessTx take the transaction and the messages to create it and logs the appropriate data
func (cc *SubstrateProvider) LogSuccessTx(res *provider.RelayerTxResponse, msgs []provider.RelayerMessage) {
	// implement
}
