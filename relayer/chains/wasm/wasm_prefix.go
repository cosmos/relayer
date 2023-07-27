package wasm

import (
	"sync"

	"github.com/CosmWasm/wasmd/app"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// This file is cursed and this mutex is too
// you don't want none of this dewey cox.
var sdkConfigMutex sync.Mutex

// Based on cosmos bech32_hack.go
// SetSDKContext sets the SDK config to the proper bech32 prefixes for wasm.
// Don't use this unless you know what you're doing.
// TODO: :dagger: :knife: :chainsaw: remove this function
func (ap *WasmProvider) SetSDKContext() {
	sdkConfigMutex.Lock()
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount(ap.PCfg.AccountPrefix, app.Bech32PrefixAccPub)
	cfg.SetBech32PrefixForValidator(ap.PCfg.AccountPrefix, app.Bech32PrefixValPub)
	cfg.SetBech32PrefixForConsensusNode(app.Bech32PrefixConsAddr, app.Bech32PrefixConsPub)
	cfg.SetAddressVerifier(wasmtypes.VerifyAddressLen())
	sdkConfigMutex.Unlock()
}
