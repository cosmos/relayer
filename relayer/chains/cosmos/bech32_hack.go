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
	sdkConfigMutex.Lock()
	sdkConf := sdk.GetConfig()
	sdkConf.SetBech32PrefixForAccount(cc.PCfg.AccountPrefix, cc.PCfg.AccountPrefix+"pub")
	sdkConf.SetBech32PrefixForValidator(cc.PCfg.AccountPrefix+"valoper", cc.PCfg.AccountPrefix+"valoperpub")
	sdkConf.SetBech32PrefixForConsensusNode(cc.PCfg.AccountPrefix+"valcons", cc.PCfg.AccountPrefix+"valconspub")
	return sdkConfigMutex.Unlock
}
