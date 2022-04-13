package cosmos

import (
	"reflect"

	"github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	typestx "github.com/cosmos/cosmos-sdk/types/tx"
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

// LogFailedTx takes the transaction and the messages to create it and logs the appropriate data
func (cc *CosmosProvider) LogFailedTx(res *provider.RelayerTxResponse, err error, msgs []provider.RelayerMessage) {
	if err != nil {
		cc.log.Error(
			"Failed sending cosmos transaction",
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
	feesField := zap.Skip()
	feePayerField := zap.Skip()

	ir := types.NewInterfaceRegistry()
	var m sdk.Msg
	if err := ir.UnpackAny(res.Tx, &m); err == nil {
		if tx, ok := m.(*typestx.Tx); ok {
			feesField = zap.Stringer("fees", tx.GetFee())

			if feePayer := getFeePayer(tx); feePayer != "" {
				feePayerField = zap.String("fee_payer", feePayer)
			}
		} else {
			cc.log.Debug(
				"Failed to convert message to Tx type",
				zap.Stringer("type", reflect.TypeOf(m)),
			)
		}
	} else {
		cc.log.Debug("Failed to unpack response Tx into sdk.Msg", zap.Error(err))
	}

	cc.log.Info(
		"Successful transaction",
		zap.String("chain_id", cc.ChainId()),
		zap.Int64("gas_used", res.GasUsed),
		feesField,
		feePayerField,
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

// getFeePayer returns the bech32 address of the fee payer of a transaction.
// This uses the fee payer field if set,
// otherwise falls back to the address of whoever signed the first message.
func getFeePayer(tx *typestx.Tx) string {
	payer := tx.AuthInfo.Fee.Payer
	if payer != "" {
		return payer
	}

	switch firstMsg := tx.GetMsgs()[0].(type) {
	case *transfertypes.MsgTransfer:
		// There is a possible data race around concurrent map access
		// in the cosmos sdk when it converts the address from bech32.
		// We don't need the address conversion; just the sender is all that
		// GetSigners is doing under the hood anyway.
		return firstMsg.Sender
	case *clienttypes.MsgCreateClient:
		// Without this particular special case, there is a panic in ibc-go
		// due to the sdk config singleton expecting one bech32 prefix but seeing another.
		return firstMsg.Signer
	case *clienttypes.MsgUpdateClient:
		// Same failure mode as MsgCreateClient.
		return firstMsg.Signer
	default:
		return firstMsg.GetSigners()[0].String()
	}
}
