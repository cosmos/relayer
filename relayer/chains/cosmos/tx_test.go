package cosmos

import (
	"fmt"
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
