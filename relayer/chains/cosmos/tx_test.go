package cosmos

import (
	"fmt"
	"math"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/testutil"
	"github.com/cosmos/cosmos-sdk/codec/types"
	authTx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/cosmos/relayer/v2/relayer/ethermint"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/stretchr/testify/require"
)

func TestCosmosProvider_AdjustEstimatedGas(t *testing.T) {
	testCases := []struct {
		name          string
		gasUsed       uint64
		gasAdjustment float64
		maxGasAmount  uint64
		expectedGas   uint64
		expectedErr   error
	}{
		{
			name:          "gas used is zero",
			gasUsed:       0,
			gasAdjustment: 1.0,
			maxGasAmount:  0,
			expectedGas:   0,
			expectedErr:   nil,
		},
		{
			name:          "gas used is non-zero",
			gasUsed:       50000,
			gasAdjustment: 1.5,
			maxGasAmount:  100000,
			expectedGas:   75000,
			expectedErr:   nil,
		},
		{
			name:          "gas used is infinite",
			gasUsed:       10000,
			gasAdjustment: math.Inf(1),
			maxGasAmount:  0,
			expectedGas:   0,
			expectedErr:   fmt.Errorf("infinite gas used"),
		},
		{
			name:          "gas used is non-zero with zero max gas amount as default",
			gasUsed:       50000,
			gasAdjustment: 1.5,
			maxGasAmount:  0,
			expectedGas:   75000,
			expectedErr:   nil,
		},
		{
			name:          "estimated gas is higher than max gas",
			gasUsed:       50000,
			gasAdjustment: 1.5,
			maxGasAmount:  70000,
			expectedGas:   75000,
			expectedErr:   fmt.Errorf("estimated gas 75000 is higher than max gas 70000"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cc := &CosmosProvider{PCfg: CosmosProviderConfig{
				GasAdjustment: tc.gasAdjustment,
				MaxGasAmount:  tc.maxGasAmount,
			}}
			adjustedGas, err := cc.AdjustEstimatedGas(tc.gasUsed)
			if err != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.Equal(t, adjustedGas, tc.expectedGas)
			}
		})
	}
}

func TestSetWithExtensionOptions(t *testing.T) {
	cc := &CosmosProvider{PCfg: CosmosProviderConfig{
		ExtensionOptions: []provider.ExtensionOption{
			{Value: "1000000000"},
			{Value: "2000000000"},
		},
	}}

	cfg := makeMockTxConfig()
	txf := tx.Factory{}.
		WithChainID("chainID").
		WithTxConfig(cfg)
	updatedTxf, err := cc.SetWithExtensionOptions(txf)
	require.NoError(t, err)
	txb, err := updatedTxf.BuildUnsignedTx()
	require.NoError(t, err)
	extOptions := txb.(*mockTxBuilder).extOptions
	actualNumExtOptions := len(extOptions)
	expectedNumExtOptions := len(cc.PCfg.ExtensionOptions)
	require.Equal(t, expectedNumExtOptions, actualNumExtOptions)
	// Check that each extension option was added with the correct type URL and value
	for i, opt := range cc.PCfg.ExtensionOptions {
		expectedTypeURL := "/ethermint.types.v1.ExtensionOptionDynamicFeeTx"
		max, ok := sdkmath.NewIntFromString(opt.Value)
		require.True(t, ok)
		expectedValue, err := (&ethermint.ExtensionOptionDynamicFeeTx{
			MaxPriorityPrice: max,
		}).Marshal()
		require.NoError(t, err)
		actualTypeURL := extOptions[i].TypeUrl
		actualValue := extOptions[i].Value
		require.Equal(t, expectedTypeURL, actualTypeURL)
		require.Equal(t, expectedValue, actualValue)
	}
}

type mockTxConfig struct {
	client.TxConfig
	txBuilder *mockTxBuilder
}

func (cfg mockTxConfig) NewTxBuilder() client.TxBuilder {
	if cfg.txBuilder == nil {
		cfg.txBuilder = &mockTxBuilder{
			TxBuilder: cfg.TxConfig.NewTxBuilder(),
		}
	}
	return cfg.txBuilder
}

type mockTxBuilder struct {
	client.TxBuilder
	extOptions []*types.Any
}

func (b *mockTxBuilder) SetExtensionOptions(extOpts ...*types.Any) {
	b.extOptions = extOpts
}

func makeTxConfig() client.TxConfig {
	interfaceRegistry := testutil.CodecOptions{}.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(interfaceRegistry)
	return authTx.NewTxConfig(cdc, authTx.DefaultSignModes)
}

func makeMockTxConfig() mockTxConfig {
	return mockTxConfig{
		TxConfig: makeTxConfig(),
	}
}
