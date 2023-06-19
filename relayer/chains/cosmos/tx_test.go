package cosmos

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

type mockAccountSequenceMismatchError struct {
	Expected uint64
	Actual   uint64
}

func (err mockAccountSequenceMismatchError) Error() string {
	return fmt.Sprintf("account sequence mismatch, expected %d, got %d: incorrect account sequence", err.Expected, err.Actual)
}

func TestHandleAccountSequenceMismatchError(t *testing.T) {
	p := &CosmosProvider{}
	ws := &WalletState{}
	p.handleAccountSequenceMismatchError(ws, mockAccountSequenceMismatchError{Actual: 9, Expected: 10})
	require.Equal(t, ws.NextAccountSequence, uint64(10))
}

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
