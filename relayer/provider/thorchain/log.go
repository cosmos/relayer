package thorchain

import (
	"fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/relayer/relayer/provider"
)

// LogFailedTx takes the transaction and the messages to create it and logs the appropriate data
func (cc *CosmosProvider) LogFailedTx(res *provider.RelayerTxResponse, err error, msgs []provider.RelayerMessage) {
	if cc.PCfg.Debug {
		cc.Log(fmt.Sprintf("- [%s] -> failed sending transaction:", cc.ChainId()))
		for _, msg := range msgs {
			_ = cc.PrintObject(msg)
		}
	}

	if err != nil {
		cc.Logger.Error(fmt.Errorf("- [%s] -> err(%v)", cc.ChainId(), err).Error())
		if res == nil {
			return
		}
	}

	if res.Code != 0 && res.Data != "" {
		cc.Log(fmt.Sprintf("✘ [%s]@{%d} - msg(%s) err(%d:%s)", cc.ChainId(), res.Height, getMsgTypes(msgs), res.Code, res.Data))
	}

	if cc.PCfg.Debug && res != nil {
		cc.Log("- transaction response:")
		_ = cc.PrintObject(res)
	}
}

// LogSuccessTx take the transaction and the messages to create it and logs the appropriate data
func (cc *CosmosProvider) LogSuccessTx(res *sdk.TxResponse, msgs []provider.RelayerMessage) {
	cc.Logger.Info(fmt.Sprintf("✔ [%s]@{%d} - msg(%s) hash(%s)", cc.ChainId(), res.Height, getMsgTypes(msgs), res.TxHash))
}

func getMsgTypes(msgs []provider.RelayerMessage) string {
	var out string
	for i, msg := range msgs {
		out += fmt.Sprintf("%d:%s,", i, msg.Type())
	}
	return strings.TrimSuffix(out, ",")
}
