package cosmos

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

// LogFailedTx takes the transaction and the messages to create it and logs the appropriate data
func (cc *CosmosProvider) LogFailedTx(res *provider.RelayerTxResponse, err error, msgs []provider.RelayerMessage) {
	if err != nil {
		cc.log.Error(
			"Failed sending transaction",
			zap.String("chain_id", cc.ChainId()),
			msgTypesField(msgs),
			zap.Error(err),
		)

		if res == nil {
			return
		}
	}

	if res.Code != 0 && res.Data != "" {
		cc.log.Warn(
			"Sent transaction but received failure response",
			zap.String("chain_id", cc.ChainId()),
			msgTypesField(msgs),
			zap.Object("response", res),
		)
	}
}

// LogSuccessTx take the transaction and the messages to create it and logs the appropriate data
func (cc *CosmosProvider) LogSuccessTx(res *sdk.TxResponse, msgs []provider.RelayerMessage) {
	cc.log.Info(
		"Successful transaction",
		zap.String("chain_id", cc.ChainId()),
		zap.Int64("height", res.Height),
		msgTypesField(msgs),
		zap.String("tx_hash", res.TxHash),
	)
}

func msgTypesField(msgs []provider.RelayerMessage) zap.Field {
	msgTypes := make([]string, len(msgs))
	for i, m := range msgs {
		msgTypes[i] = m.Type()
	}
	return zap.Strings("msg_types", msgTypes)
}
