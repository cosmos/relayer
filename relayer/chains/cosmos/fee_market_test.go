package cosmos

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	coinType = 118

	testCfg = CosmosProviderConfig{
		KeyDirectory:     "",
		Key:              "default",
		ChainName:        "osmosis",
		ChainID:          "osmosis-1",
		RPCAddr:          "https://osmosis-rpc.polkachu.com:443",
		AccountPrefix:    "osmo",
		KeyringBackend:   "test",
		DynamicGasPrice:  true,
		GasAdjustment:    1.2,
		GasPrices:        "0.0025uosmo",
		MinGasAmount:     1,
		MaxGasAmount:     0,
		Debug:            false,
		Timeout:          "3m",
		BlockTimeout:     "30s",
		OutputFormat:     "json",
		SignModeStr:      "direct",
		ExtraCodecs:      nil,
		Modules:          nil,
		Slip44:           &coinType,
		SigningAlgorithm: "",
		Broadcast:        "batch",
		MinLoopDuration:  0,
		ExtensionOptions: nil,
		FeeGrants:        nil,
	}
)

func TestQueryBaseFee(t *testing.T) {
	p, err := testCfg.NewProvider(nil, t.TempDir(), true, testCfg.ChainName)
	require.NoError(t, err)

	ctx := context.Background()
	err = p.Init(ctx)
	require.NoError(t, err)

	cp := p.(*CosmosProvider)

	baseFee, err := cp.QueryBaseFee(ctx)
	require.NoError(t, err)
	require.NotEqual(t, "", baseFee)
}

func TestParseDenom(t *testing.T) {
	denom, err := parseTokenDenom(testCfg.GasPrices)
	require.NoError(t, err)
	require.Equal(t, "uosmo", denom)
}
