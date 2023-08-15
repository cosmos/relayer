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
func (ap *WasmProvider) SetSDKContext() func() {

	sdkConfigMutex.Lock()
	cfg_update := sdk.GetConfig()
	cfg_update.SetBech32PrefixForAccount(ap.PCfg.AccountPrefix, app.Bech32PrefixAccPub)
	cfg_update.SetBech32PrefixForValidator(ap.PCfg.AccountPrefix, app.Bech32PrefixValPub)
	cfg_update.SetBech32PrefixForConsensusNode(app.Bech32PrefixConsAddr, app.Bech32PrefixConsPub)
	cfg_update.SetAddressVerifier(wasmtypes.VerifyAddressLen())
	return sdkConfigMutex.Unlock
}
