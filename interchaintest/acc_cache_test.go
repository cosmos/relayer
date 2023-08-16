package interchaintest

import (
	"testing"

	"github.com/cometbft/cometbft/crypto/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

// See https://github.com/cosmos/cosmos-sdk/issues/15317 for description of bug.
// Basically, in cosmos SDK, there is an account address cache that will ignore the bech32 prefix setting.
// This will cause the AccAddress.String() to print out unexpected prefixes.
// If this function fails you are on an unsafe SDK version that should NOT be used with the relayer.
func TestAccCacheBugfix(t *testing.T) {
	sdk.SetAddrCacheEnabled(false)

	// Use a random key
	priv := ed25519.GenPrivKey()
	pub := priv.PubKey()

	//Set to 'osmo'
	prefix := "osmo"
	sdkConf := sdk.GetConfig()
	sdkConf.SetBech32PrefixForAccount(prefix, prefix+"pub")
	sdkConf.SetBech32PrefixForValidator(prefix+"valoper", prefix+"valoperpub")
	sdkConf.SetBech32PrefixForConsensusNode(prefix+"valcons", prefix+"valconspub")

	addrOsmo := sdk.AccAddress(pub.Address())
	osmoAddrBech32 := addrOsmo.String()

	//Set to 'cosmos'
	prefix = "cosmos"
	sdkConf.SetBech32PrefixForAccount(prefix, prefix+"pub")
	sdkConf.SetBech32PrefixForValidator(prefix+"valoper", prefix+"valoperpub")
	sdkConf.SetBech32PrefixForConsensusNode(prefix+"valcons", prefix+"valconspub")

	addrCosmos := sdk.AccAddress(pub.Address())
	cosmosAddrBech32 := addrCosmos.String()

	//If the addresses are equal, the AccAddress caching caused a bug
	require.NotEqual(t, osmoAddrBech32, cosmosAddrBech32)
}
