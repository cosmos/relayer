package archway

import (
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
	"github.com/cosmos/relayer/v2/relayer/chains/archway/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

type WasmContractMessage struct {
	Msg *wasmtypes.MsgExecuteContract
}

func (w *WasmContractMessage) Type() string {
	return "wasm"
}

func (w *WasmContractMessage) MsgBytes() ([]byte, error) {
	return []byte("ibc"), nil
}

func (ap *ArchwayProvider) NewWasmContractMessage(method string, m codec.ProtoMarshaler) (provider.RelayerMessage, error) {
	signer, _ := ap.Address()
	contract := ap.PCfg.IbcHandlerAddress

	protoMsg, err := ap.Cdc.Marshaler.Marshal(m)
	if err != nil {
		return nil, err
	}

	msgParam, err := types.GenerateTxnParams(method, types.NewHexBytes(protoMsg))

	if err != nil {
		return nil, err
	}

	return &WasmContractMessage{
		Msg: &wasmtypes.MsgExecuteContract{
			Sender:   signer,
			Contract: contract,
			Msg:      msgParam,
		},
	}, nil
}

type ArchwayMessage struct {
	Msg sdk.Msg
}

func (am ArchwayMessage) Type() string {
	return sdk.MsgTypeURL(am.Msg)
}

func (am ArchwayMessage) MsgBytes() ([]byte, error) {
	return proto.Marshal(am.Msg)
}

func ArchwayMsgs(rm ...provider.RelayerMessage) []sdk.Msg {
	sdkMsgs := make([]sdk.Msg, 0)
	for _, rMsg := range rm {
		if val, ok := rMsg.(ArchwayMessage); !ok {
			return nil
		} else {
			sdkMsgs = append(sdkMsgs, val.Msg)
		}
	}
	return sdkMsgs
}
