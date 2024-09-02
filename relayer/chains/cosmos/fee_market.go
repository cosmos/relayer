package cosmos

import (
	"context"
	"fmt"
	"regexp"

	sdkmath "cosmossdk.io/math"
	"go.uber.org/zap"
)

const queryPath = "/osmosis.txfees.v1beta1.Query/GetEipBaseFee"

// DynamicFee queries the dynamic gas price base fee and returns a string with the base fee and token denom concatenated.
// If the chain does not have dynamic fees enabled in the config, nothing happens and an empty string is always returned.
func (cc *CosmosProvider) DynamicFee(ctx context.Context) string {
	if !cc.PCfg.DynamicGasPrice {
		return ""
	}

	dynamicFee, err := cc.QueryBaseFee(ctx)
	if err != nil {
		// If there was an error querying the dynamic base fee, do nothing and fall back to configured gas price.
		cc.log.Warn("Failed to query the dynamic gas price base fee", zap.Error(err))
		return ""
	}

	return dynamicFee
}

// QueryBaseFee attempts to make an ABCI query to retrieve the base fee on chains using the Osmosis EIP-1559 implementation.
// This is currently hardcoded to only work on Osmosis.
func (cc *CosmosProvider) QueryBaseFee(ctx context.Context) (string, error) {
	resp, err := cc.ConsensusClient.GetABCIQuery(ctx, queryPath, nil)
	if err != nil || resp.Code != 0 {
		return "", err
	}

	decFee, err := sdkmath.LegacyNewDecFromStr(resp.ValueCleaned())
	if err != nil {
		return "", err
	}

	baseFee, err := decFee.Float64()
	if err != nil {
		return "", err
	}

	// The current EIP-1559 implementation returns an integer and does not return any value that tells us how many
	// decimal places we need to account for.
	//
	// This may be problematic because we are assuming that we always need to move the decimal 18 places.
	fee := baseFee / 1e18

	denom, err := parseTokenDenom(cc.PCfg.GasPrices)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%f%s", fee, denom), nil
}

// parseTokenDenom takes a string in the format numericGasPrice + tokenDenom (e.g. 0.0025uosmo),
// and parses the tokenDenom portion (e.g. uosmo) before returning just the token denom.
func parseTokenDenom(gasPrice string) (string, error) {
	regex := regexp.MustCompile(`^0\.\d+([a-zA-Z]+)$`)

	matches := regex.FindStringSubmatch(gasPrice)

	if len(matches) != 2 {
		return "", fmt.Errorf("failed to parse token denom from string %s", gasPrice)
	}

	return matches[1], nil
}
