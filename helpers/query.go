package helpers

import (
	"context"
	"strconv"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer"
)

// QueryBalance is a helper function for query balance
func QueryBalance(ctx context.Context, chain *relayer.Chain, address string, showDenoms bool) (sdk.Coins, error) {
	coins, err := chain.ChainProvider.QueryBalanceWithAddress(ctx, address)
	if err != nil {
		return nil, err
	}

	if showDenoms {
		return coins, nil
	}

	h, err := chain.ChainProvider.QueryLatestHeight(ctx)
	if err != nil {
		return nil, err
	}

	dts, err := chain.ChainProvider.QueryDenomTraces(ctx, 0, 1000, h)
	if err != nil {
		return nil, err
	}

	if len(dts) == 0 {
		return coins, nil
	}

	var out sdk.Coins
	for _, c := range coins {
		if c.Amount.Equal(sdk.NewInt(0)) {
			continue
		}

		for i, d := range dts {
			if strings.EqualFold(c.Denom, d.IBCDenom()) {
				out = append(out, sdk.Coin{Denom: d.GetFullDenomPath(), Amount: c.Amount})
				break
			}

			if i == len(dts)-1 {
				out = append(out, c)
			}
		}
	}
	return out, nil
}

// QueryHeader is a helper function for query header
func QueryHeader(ctx context.Context, chain *relayer.Chain, opts ...string) (ibcexported.ClientMessage, error) {
	if len(opts) > 0 {
		height, err := strconv.ParseInt(opts[0], 10, 64) //convert to int64
		if err != nil {
			return nil, err
		}

		return chain.ChainProvider.QueryHeaderAtHeight(ctx, height)
	}

	return chain.ChainProvider.GetLightSignedHeaderAtHeight(ctx, 0)
}
