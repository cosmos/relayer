package relayer

import (
	"testing"
	"time"

	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
)

func TestSPrintClientExpiration_PrintChainId(t *testing.T) {
	previousTime := time.Now().Add(10 * time.Hour)
	mockHeight := clienttypes.NewHeight(1, 100)
	trustingPeriod := 1 * time.Hour

	chain := mockChain("expected-chain-id", "test-client-id")
	clientStateInfo := mockClientStateInfo("test-chain-id", trustingPeriod, mockHeight)
	expiration := SPrintClientExpiration(chain, previousTime, *clientStateInfo)

	require.Contains(t, expiration, "expected-chain-id")
}

func TestSPrintClientExpiration_PrintClientId(t *testing.T) {
	previousTime := time.Now().Add(10 * time.Hour)
	mockHeight := clienttypes.NewHeight(1, 100)
	trustingPeriod := 1 * time.Hour

	chain := mockChain("test-chain-id", "expected-client-id")
	clientStateInfo := mockClientStateInfo("test-chain-id", trustingPeriod, mockHeight)
	expiration := SPrintClientExpiration(chain, previousTime, *clientStateInfo)

	require.Contains(t, expiration, "expected-client-id")
}

func TestSPrintClientExpiration_PrintExpired_WhenTimeIsInPast(t *testing.T) {
	previousTime := time.Now().Add(-10 * time.Hour)
	mockHeight := clienttypes.NewHeight(1, 100)
	trustingPeriod := 1 * time.Hour

	chain := mockChain("test-chain-id", "test-client-id")
	clientStateInfo := mockClientStateInfo("test-chain-id", trustingPeriod, mockHeight)
	expiration := SPrintClientExpiration(chain, previousTime, *clientStateInfo)

	require.Contains(t, expiration, "EXPIRED")
}

func TestSPrintClientExpiration_PrintRFC822FormattedTime_WhenTimeIsInPast(t *testing.T) {
	pastTime := time.Now().Add(-10 * time.Hour)
	mockHeight := clienttypes.NewHeight(1, 100)
	trustingPeriod := 1 * time.Hour

	chain := mockChain("expected-chain-id", "test-client-id")
	clientStateInfo := mockClientStateInfo("test-chain-id", trustingPeriod, mockHeight)
	expiration := SPrintClientExpiration(chain, pastTime, *clientStateInfo)

	require.Contains(t, expiration, pastTime.Format(time.RFC822))
}

func TestSPrintClientExpiration_PrintGood_WhenTimeIsInFuture(t *testing.T) {
	previousTime := time.Now().Add(10 * time.Hour)
	mockHeight := clienttypes.NewHeight(1, 100)
	trustingPeriod := 1 * time.Hour

	chain := mockChain("test-chain-id", "test-client-id")
	clientStateInfo := mockClientStateInfo("test-chain-id", trustingPeriod, mockHeight)
	expiration := SPrintClientExpiration(chain, previousTime, *clientStateInfo)

	require.Contains(t, expiration, "GOOD")
}

func TestSPrintClientExpiration_PrintRFC822FormattedTime_WhenTimeIsInFuture(t *testing.T) {
	futureTime := time.Now().Add(10 * time.Hour)
	mockHeight := clienttypes.NewHeight(1, 100)
	trustingPeriod := 1 * time.Hour

	chain := mockChain("test-chain-id", "test-client-id")
	clientStateInfo := mockClientStateInfo("test-chain-id", trustingPeriod, mockHeight)
	expiration := SPrintClientExpiration(chain, futureTime, *clientStateInfo)

	require.Contains(t, expiration, futureTime.Format(time.RFC822))
}

func TestSPrintClientExpiration_PrintRemainingTime_WhenTimeIsInFuture(t *testing.T) {
	futureTime := time.Now().Add(10 * time.Hour)
	mockHeight := clienttypes.NewHeight(1, 100)
	trustingPeriod := 1 * time.Hour

	chain := mockChain("test-chain-id", "test-client-id")
	clientStateInfo := mockClientStateInfo("test-chain-id", trustingPeriod, mockHeight)
	expiration := SPrintClientExpiration(chain, futureTime, *clientStateInfo)

	require.Contains(t, expiration, "10h0m0s")
}

func TestSPrintClientExpiration_TrustingPeriod(t *testing.T) {
	previousTime := time.Now().Add(10 * time.Hour)
	mockHeight := clienttypes.NewHeight(1, 100)
	trustingPeriod := 1 * time.Hour

	chain := mockChain("expected-chain-id", "test-client-id")
	clientStateInfo := mockClientStateInfo("test-chain-id", trustingPeriod, mockHeight)
	expiration := SPrintClientExpiration(chain, previousTime, *clientStateInfo)

	require.Contains(t, expiration, "1h0m0s")
}

func TestSPrintClientExpiration_LastUpdateHeight(t *testing.T) {
	previousTime := time.Now().Add(10 * time.Hour)
	mockHeight := clienttypes.NewHeight(1, 100)
	trustingPeriod := 1 * time.Hour

	chain := mockChain("expected-chain-id", "test-client-id")
	clientStateInfo := mockClientStateInfo("test-chain-id", trustingPeriod, mockHeight)
	expiration := SPrintClientExpiration(chain, previousTime, *clientStateInfo)

	require.Contains(t, expiration, "100")
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

func mockClientStateInfo(chainID string, trustingPeriod time.Duration, latestHeight ibcexported.Height) *ClientStateInfo { //nolint:unparam // chainID always receives the same value
	mockHeight := clienttypes.NewHeight(1, 100)
	return &ClientStateInfo{
		ChainID:        chainID,
		TrustingPeriod: 1 * time.Hour,
		LatestHeight:   mockHeight,
	}
}
