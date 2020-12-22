package helpers

import (
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
		for _, d := range dts.DenomTraces {
			switch {
			case c.Amount.Equal(sdk.NewInt(0)):
			case c.Denom == d.IBCDenom():
				out = append(out, sdk.NewCoin(d.GetFullDenomPath(), c.Amount))
			default:
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

		if height <= 0 {
			height, err = chain.QueryLatestHeight()
			if err != nil {
				return nil, err
			}

			if height < 0 {
				return nil, relayer.ErrLightNotInitialized
			}
		}

		return chain.QueryHeaderAtHeight(height)
	}

	return chain.QueryLatestHeader()
}

// QueryTxs is a helper function for query txs
func QueryTxs(chain *relayer.Chain, eventsStr string, offset uint64, limit uint64) ([]*ctypes.ResultTx, error) {
	events, err := relayer.ParseEvents(eventsStr)
	if err != nil {
		return nil, err
	}

	h, err := chain.UpdateLightWithHeader()
	if err != nil {
		return nil, err
	}

	return chain.QueryTxs(relayer.MustGetHeight(h.GetHeight()), int(offset), int(limit), events)
}
