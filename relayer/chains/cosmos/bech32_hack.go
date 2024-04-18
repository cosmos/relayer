package cosmos

import (
	"sync"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// This file is cursed and this mutex is too
// you don't want none of this dewey cox.
var sdkConfigMutex sync.Mutex

// SetSDKContext sets the SDK config to the proper bech32 prefixes.
// Don't use this unless you know what you're doing.
// TODO: :dagger: :knife: :chainsaw: remove this function
func (cc *CosmosProvider) SetSDKContext() func() {
	return SetSDKConfigContext(cc.PCfg.AccountPrefix)
}

// SetSDKContext sets the SDK config to the given bech32 prefixes
func SetSDKConfigContext(prefix string) func() {
	sdkConfigMutex.Lock()
	sdkConf := sdk.GetConfig()
	sdkConf.SetBech32PrefixForAccount(prefix, prefix+"pub")
	sdkConf.SetBech32PrefixForValidator(prefix+"valoper", prefix+"valoperpub")
	sdkConf.SetBech32PrefixForConsensusNode(prefix+"valcons", prefix+"valconspub")
	return sdkConfigMutex.Unlock
}
