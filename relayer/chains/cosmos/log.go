package cosmos

import (
	"errors"
	"reflect"

	"github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	typestx "github.com/cosmos/cosmos-sdk/types/tx"
	feetypes "github.com/cosmos/ibc-go/v7/modules/apps/29-fee/types"
	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"

	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// getChannelsIfPresent scans the events for channel tags
func getChannelsIfPresent(events []provider.RelayerEvent) []zapcore.Field {
	channelTags := []string{srcChanTag, dstChanTag}
	fields := []zap.Field{}

	// While a transaction may have multiple messages, we just need to first
	// pair of channels
	foundTag := map[string]struct{}{}

	for _, event := range events {
		for _, tag := range channelTags {
			for attributeKey, attributeValue := range event.Attributes {
				if attributeKey == tag {
					// Only append the tag once
					// TODO: what if they are different?
					if _, ok := foundTag[tag]; !ok {
						fields = append(fields, zap.String(tag, attributeValue))
						foundTag[tag] = struct{}{}
					}
				}
			}
		}
	}
	return fields
}

// LogFailedTx takes the transaction and the messages to create it and logs the appropriate data
func (cc *CosmosProvider) LogFailedTx(res *provider.RelayerTxResponse, err error, msgs []provider.RelayerMessage) {
	// Include the chain_id
	fields := []zapcore.Field{zap.String("chain_id", cc.ChainId())}

	// Extract the channels from the events, if present
	if res != nil {
		channels := getChannelsIfPresent(res.Events)
		fields = append(fields, channels...)
	}
	fields = append(fields, msgTypesField(msgs))

	if err != nil {

		if errors.Is(err, chantypes.ErrRedundantTx) {
			cc.log.Debug("Redundant message(s)", fields...)
			return
		}

		// Make a copy since we may continue to the warning
		errorFields := append(fields, zap.Error(err))
		cc.log.Error(
			"Failed sending cosmos transaction",
			errorFields...,
		)

		if res == nil {
			return
		}
	}

	if res.Code != 0 {
		if sdkErr := cc.sdkError(res.Codespace, res.Code); err != nil {
			fields = append(fields, zap.NamedError("sdk_error", sdkErr))
		}
		fields = append(fields, zap.Object("response", res))
		cc.log.Warn(
			"Sent transaction but received failure response",
			fields...,
		)
	}
}

// LogSuccessTx take the transaction and the messages to create it and logs the appropriate data
func (cc *CosmosProvider) LogSuccessTx(res *sdk.TxResponse, msgs []provider.RelayerMessage) {
	// Include the chain_id
	fields := []zapcore.Field{zap.String("chain_id", cc.ChainId())}

	// Extract the channels from the events, if present
	if res != nil {
		events := parseEventsFromTxResponse(res)
		fields = append(fields, getChannelsIfPresent(events)...)
	}

	// Include the gas used
	fields = append(fields, zap.Int64("gas_used", res.GasUsed))

	// Extract fees and fee_payer if present
	ir := types.NewInterfaceRegistry()
	var m sdk.Msg
	if err := ir.UnpackAny(res.Tx, &m); err == nil {
		if tx, ok := m.(*typestx.Tx); ok {
			fields = append(fields, zap.Stringer("fees", tx.GetFee()))
			if feePayer := getFeePayer(tx); feePayer != "" {
				fields = append(fields, zap.String("fee_payer", feePayer))
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

	// Include the height, msgType, and tx_hash
	fields = append(fields,
		zap.Int64("height", res.Height),
		msgTypesField(msgs),
		zap.String("tx_hash", res.TxHash),
	)

	// Log the successful transaction with fields
	cc.log.Info(
		"Successful transaction",
		fields...,
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
	case *clienttypes.MsgUpgradeClient:
		return firstMsg.Signer
	case *clienttypes.MsgSubmitMisbehaviour:
		// Same failure mode as MsgCreateClient.
		return firstMsg.Signer
	case *feetypes.MsgRegisterPayee:
		return firstMsg.Relayer
	case *feetypes.MsgRegisterCounterpartyPayee:
		return firstMsg.Relayer
	case *feetypes.MsgPayPacketFee:
		return firstMsg.Signer
	case *feetypes.MsgPayPacketFeeAsync:
		return firstMsg.PacketFee.RefundAddress
	default:
		return firstMsg.GetSigners()[0].String()
	}
}
