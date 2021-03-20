package helpers

import (
	"fmt"
	"math"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/relayer"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

// QueryBalance is a helper function for query balance
func QueryBalance(chain *relayer.Chain, address string, showDenoms bool) (sdk.Coins, error) {
	coins, err := chain.QueryBalanceWithAddress(address)
	if err != nil {
		return nil, err
	}

	if showDenoms {
		return coins, nil
	}

	h, err := chain.QueryLatestHeight()
	if err != nil {
		return nil, err
	}

	dts, err := chain.QueryDenomTraces(0, 1000, h)
	if err != nil {
		return nil, err
	}

	if len(dts.DenomTraces) == 0 {
		return coins, nil
	}

	var out sdk.Coins
	for _, c := range coins {
		if c.Amount.Equal(sdk.NewInt(0)) {
			continue
		}

		for i, d := range dts.DenomTraces {
			if c.Denom == d.IBCDenom() {
				out = append(out, sdk.Coin{Denom: d.GetFullDenomPath(), Amount: c.Amount})
				break
			}

			if i == len(dts.DenomTraces)-1 {
				out = append(out, c)
			}
		}
	}
	return out, nil
}

// QueryHeader is a helper function for query header
func QueryHeader(chain *relayer.Chain, opts ...string) (*tmclient.Header, error) {
	if len(opts) > 0 {
		height, err := strconv.ParseInt(opts[0], 10, 64) //convert to int64
		if err != nil {
			return nil, err
		}

		return chain.QueryHeaderAtHeight(height)
	}

	return chain.GetLightSignedHeaderAtHeight(0)
}

// QueryTxs is a helper function for query txs
func QueryTxs(chain *relayer.Chain, eventsStr string, offset uint64, limit uint64) ([]*ctypes.ResultTx, error) {
	events, err := relayer.ParseEvents(eventsStr)
	if err != nil {
		return nil, err
	}

	_, err = chain.UpdateLightClient()
	if err != nil {
		return nil, err
	}

	if offset > math.MaxInt64 {
		return nil, fmt.Errorf("offset (%d) value is greater than max int value", offset)
	}

	if limit > math.MaxInt64 {
		return nil, fmt.Errorf("limit (%d) value is greater than max int value", limit)
	}

	return chain.QueryTxs(chain.MustGetLatestLightHeight(), int(offset), int(limit), events)
}
