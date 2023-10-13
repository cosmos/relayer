package relayer

import (
	"testing"
	"time"

	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"

	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	"github.com/stretchr/testify/require"
)

func TestSPrintClientExpiration_PrintChainId(t *testing.T) {
	previousTime := time.Now().Add(10 * time.Hour)
	mockHeight := clienttypes.NewHeight(1, 100)
	trustingPeriod := time.Duration(1 * time.Hour)

	chain := mockChain("expected-chain-id", "test-client-id")
	clientStateInfo := mockClientStateInfo("test-chain-id", trustingPeriod, mockHeight)
	expiration := SPrintClientExpiration(chain, previousTime, *clientStateInfo)

	require.Contains(t, expiration, "expected-chain-id")
}

func TestSPrintClientExpiration_PrintClientId(t *testing.T) {
	previousTime := time.Now().Add(10 * time.Hour)
	mockHeight := clienttypes.NewHeight(1, 100)
	trustingPeriod := time.Duration(1 * time.Hour)

	chain := mockChain("test-chain-id", "expected-client-id")
	clientStateInfo := mockClientStateInfo("test-chain-id", trustingPeriod, mockHeight)
	expiration := SPrintClientExpiration(chain, previousTime, *clientStateInfo)
	expirationJson := SPrintClientExpirationJson(chain, previousTime, *clientStateInfo)

	require.Contains(t, expiration, "expected-client-id")
	require.Contains(t, expirationJson, "expected-client-id")
}

func TestSPrintClientExpiration_PrintExpired_WhenTimeIsInPast(t *testing.T) {
	previousTime := time.Now().Add(-10 * time.Hour)
	mockHeight := clienttypes.NewHeight(1, 100)
	trustingPeriod := time.Duration(1 * time.Hour)

	chain := mockChain("test-chain-id", "test-client-id")
	clientStateInfo := mockClientStateInfo("test-chain-id", trustingPeriod, mockHeight)
	expiration := SPrintClientExpiration(chain, previousTime, *clientStateInfo)
	expirationJson := SPrintClientExpirationJson(chain, previousTime, *clientStateInfo)

	require.Contains(t, expiration, "EXPIRED")
	require.Contains(t, expirationJson, "EXPIRED")
}

func TestSPrintClientExpiration_PrintRFC822FormattedTime_WhenTimeIsInPast(t *testing.T) {
	pastTime := time.Now().Add(-10 * time.Hour)
	mockHeight := clienttypes.NewHeight(1, 100)
	trustingPeriod := time.Duration(1 * time.Hour)

	chain := mockChain("expected-chain-id", "test-client-id")
	clientStateInfo := mockClientStateInfo("test-chain-id", trustingPeriod, mockHeight)
	expiration := SPrintClientExpiration(chain, pastTime, *clientStateInfo)
	expirationJson := SPrintClientExpirationJson(chain, pastTime, *clientStateInfo)

	require.Contains(t, expiration, pastTime.Format(time.RFC822))
	require.Contains(t, expirationJson, pastTime.Format(time.RFC822))
}

func TestSPrintClientExpiration_PrintGood_WhenTimeIsInFuture(t *testing.T) {
	previousTime := time.Now().Add(10 * time.Hour)
	mockHeight := clienttypes.NewHeight(1, 100)
	trustingPeriod := time.Duration(1 * time.Hour)

	chain := mockChain("test-chain-id", "test-client-id")
	clientStateInfo := mockClientStateInfo("test-chain-id", trustingPeriod, mockHeight)
	expiration := SPrintClientExpiration(chain, previousTime, *clientStateInfo)
	expirationJson := SPrintClientExpirationJson(chain, previousTime, *clientStateInfo)

	require.Contains(t, expiration, "GOOD")
	require.Contains(t, expirationJson, "GOOD")
}

func TestSPrintClientExpiration_PrintRFC822FormattedTime_WhenTimeIsInFuture(t *testing.T) {
	futureTime := time.Now().Add(10 * time.Hour)
	mockHeight := clienttypes.NewHeight(1, 100)
	trustingPeriod := time.Duration(1 * time.Hour)

	chain := mockChain("test-chain-id", "test-client-id")
	clientStateInfo := mockClientStateInfo("test-chain-id", trustingPeriod, mockHeight)
	expiration := SPrintClientExpiration(chain, futureTime, *clientStateInfo)
	expirationJson := SPrintClientExpirationJson(chain, futureTime, *clientStateInfo)

	require.Contains(t, expiration, futureTime.Format(time.RFC822))
	require.Contains(t, expirationJson, futureTime.Format(time.RFC822))
}

func TestSPrintClientExpiration_PrintRemainingTime_WhenTimeIsInFuture(t *testing.T) {
	futureTime := time.Now().Add(10 * time.Hour)
	mockHeight := clienttypes.NewHeight(1, 100)
	trustingPeriod := time.Duration(1 * time.Hour)

	chain := mockChain("test-chain-id", "test-client-id")
	clientStateInfo := mockClientStateInfo("test-chain-id", trustingPeriod, mockHeight)
	expiration := SPrintClientExpiration(chain, futureTime, *clientStateInfo)
	expirationJson := SPrintClientExpirationJson(chain, futureTime, *clientStateInfo)

	require.Contains(t, expiration, "10h0m0s")
	require.Contains(t, expirationJson, "10h0m0s")
}

func TestSPrintClientExpiration_TrustingPeriod(t *testing.T) {
	previousTime := time.Now().Add(10 * time.Hour)
	mockHeight := clienttypes.NewHeight(1, 100)
	trustingPeriod := time.Duration(1 * time.Hour)

	chain := mockChain("expected-chain-id", "test-client-id")
	clientStateInfo := mockClientStateInfo("test-chain-id", trustingPeriod, mockHeight)
	expiration := SPrintClientExpiration(chain, previousTime, *clientStateInfo)
	expirationJson := SPrintClientExpirationJson(chain, previousTime, *clientStateInfo)

	require.Contains(t, expiration, "1h0m0s")
	require.Contains(t, expirationJson, "1h0m0s")
}

func TestSPrintClientExpiration_LastUpdateHeight(t *testing.T) {
	previousTime := time.Now().Add(10 * time.Hour)
	mockHeight := clienttypes.NewHeight(1, 100)
	trustingPeriod := time.Duration(1 * time.Hour)

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

func mockClientStateInfo(chainID string, trustingPeriod time.Duration, latestHeight ibcexported.Height) *ClientStateInfo {
	mockHeight := clienttypes.NewHeight(1, 100)
	return &ClientStateInfo{
		ChainID:        chainID,
		TrustingPeriod: time.Duration(1 * time.Hour),
		LatestHeight:   mockHeight,
	}
}
