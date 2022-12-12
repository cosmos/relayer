package relayer

import (
	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSPrintClientExpiration_PrintChainId(t *testing.T) {
	previousTime := time.Now().Add(10 * time.Hour)

	chain := mockChain("expected-chain-id", "test-client-id")
	expiration := SPrintClientExpiration(chain, previousTime)

	require.Contains(t, expiration, "expected-chain-id")
}

func TestSPrintClientExpiration_PrintClientId(t *testing.T) {
	previousTime := time.Now().Add(10 * time.Hour)

	chain := mockChain("test-chain-id", "expected-client-id")
	expiration := SPrintClientExpiration(chain, previousTime)

	require.Contains(t, expiration, "expected-client-id")
}

func TestSPrintClientExpiration_PrintIsAlreadyExpired_WhenTimeIsInPast(t *testing.T) {
	previousTime := time.Now().Add(-10 * time.Hour)

	chain := mockChain("test-chain-id", "test-client-id")
	expiration := SPrintClientExpiration(chain, previousTime)

	require.Contains(t, expiration, "is already expired")
}

func TestSPrintClientExpiration_PrintRFC822FormattedTime_WhenTimeIsInPast(t *testing.T) {
	pastTime := time.Now().Add(-10 * time.Hour)

	chain := mockChain("test-chain-id", "test-client-id")
	expiration := SPrintClientExpiration(chain, pastTime)

	require.Contains(t, expiration, pastTime.Format(time.RFC822))
}

func TestSPrintClientExpiration_PrintExpiresIn_WhenTimeIsInFuture(t *testing.T) {
	previousTime := time.Now().Add(10 * time.Hour)

	chain := mockChain("test-chain-id", "test-client-id")
	expiration := SPrintClientExpiration(chain, previousTime)

	require.Contains(t, expiration, "expires in")
}

func TestSPrintClientExpiration_PrintRFC822FormattedTime_WhenTimeIsInFuture(t *testing.T) {
	futureTime := time.Now().Add(10 * time.Hour)

	chain := mockChain("test-chain-id", "test-client-id")
	expiration := SPrintClientExpiration(chain, futureTime)

	require.Contains(t, expiration, futureTime.Format(time.RFC822))
}

func TestSPrintClientExpiration_PrintRemainingTime_WhenTimeIsInFuture(t *testing.T) {
	futureTime := time.Now().Add(10 * time.Hour)

	chain := mockChain("test-chain-id", "test-client-id")
	expiration := SPrintClientExpiration(chain, futureTime)

	require.Contains(t, expiration, "10h0m0s")
}

func mockChain(chainId string, clientId string) *Chain {
	return &Chain{
		Chainid: chainId,
		ChainProvider: &cosmos.CosmosProvider{
			PCfg: cosmos.CosmosProviderConfig{
				ChainID: chainId,
			},
		},
		PathEnd: &PathEnd{
			ChainID:  chainId,
			ClientID: clientId,
		},
	}
}
