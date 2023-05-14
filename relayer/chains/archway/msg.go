package archway

import (
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

type WasmContractMessage wasmtypes.MsgExecuteContract

func (w *WasmContractMessage) Type() string {
	return "wasm"
}

func (w *WasmContractMessage) MsgBytes() ([]byte, error) {
	return []byte("ibc"), nil
}

func NewWasmContractMessage(sender, contract string, msg wasmtypes.RawContractMessage) provider.RelayerMessage {
	return &WasmContractMessage{
		Sender:   sender,
		Contract: contract,
		Msg:      msg,
	}
}
