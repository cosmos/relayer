package interchaintest

import (
	"context"
	"testing"

	"github.com/cosmos/cosmos-sdk/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ibctestv7 "github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// See https://github.com/cosmos/cosmos-sdk/issues/15317 for description of bug.
// Basically, in cosmos SDK, there is an account address cache that will ignore the bech32 prefix setting.
// This will cause the AccAddress.String() to print out unexpected prefixes.
// If this function fails you are on an unsafe SDK version that should NOT be used with the relayer.
func TestAccCacheBugfix(t *testing.T) {
	types.SetAddrCacheEnabled(false)

	testMnemonic := "clip hire initial neck maid actor venue client foam budget lock catalog sweet steak waste crater broccoli pipe steak sister coyote moment obvious choose"
	nv := 1
	nf := 0

	// Chain Factory
	cf := ibctestv7.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctestv7.ChainSpec{
		{Name: "osmosis", ChainName: "osmosis", Version: "v14.0.1", NumValidators: &nv, NumFullNodes: &nf},
	})

	ctx := context.Background()
	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	osmosis := chains[0]

	// Prep Interchain
	ic := ibctestv7.NewInterchain().AddChain(osmosis)

	// Reporter/logs
	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	client, network := ibctestv7.DockerSetup(t)

	// Build interchain
	require.NoError(t, ic.Build(ctx, eRep, ibctestv7.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,

		SkipPathCreation: false,
	}))

	user, err := osmosis.BuildWallet(ctx, "default", testMnemonic)
	require.NoError(t, err)

	//Set to 'osmo'
	prefix := "osmo"
	sdkConf := sdk.GetConfig()
	sdkConf.SetBech32PrefixForAccount(prefix, prefix+"pub")
	sdkConf.SetBech32PrefixForValidator(prefix+"valoper", prefix+"valoperpub")
	sdkConf.SetBech32PrefixForConsensusNode(prefix+"valcons", prefix+"valconspub")

	addrOsmo := sdk.AccAddress(user.Address())
	osmoAddrBech32 := addrOsmo.String()

	//Set to 'cosmos'
	prefix = "cosmos"
	sdkConf.SetBech32PrefixForAccount(prefix, prefix+"pub")
	sdkConf.SetBech32PrefixForValidator(prefix+"valoper", prefix+"valoperpub")
	sdkConf.SetBech32PrefixForConsensusNode(prefix+"valcons", prefix+"valconspub")

	addrCosmos := sdk.AccAddress(user.Address())
	cosmosAddrBech32 := addrCosmos.String()

	//If the addresses are equal, the AccAddress caching caused a bug
	require.NotEqual(t, osmoAddrBech32, cosmosAddrBech32)
}
