package helpers

import (
	"net/http"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/relayer"
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

	if len(dts.DenomTraces) > 0 {
		out := sdk.Coins{}
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

	return coins, nil
}

// QueryHeader is a helper function for query header
func QueryHeader(chain *relayer.Chain, opts ...string) (*tmclient.Header, error) {
	if len(opts) > 0 {
		height, err := strconv.ParseInt(opts[0], 10, 64) //convert to int64
		if err != nil {
			return nil, err
		}

		if height == 0 {
			height, err = chain.QueryLatestHeight()
			if err != nil {
				return nil, err
			}

			if height == -1 {
				return nil, relayer.ErrLightNotInitialized
			}
		}

		header, err := chain.QueryHeaderAtHeight(height)
		if err != nil {
			return nil, err
		}
		return header, nil
	}

	header, err := chain.QueryLatestHeader()
	if err != nil {
		return nil, err
	}

	return header, nil
}

// ParseHeightFromRequest parse height from query params and if not found, returns latest height
func ParseHeightFromRequest(r *http.Request, chain *relayer.Chain) (int64, error) {
	heightStr := r.URL.Query().Get("height")

	if len(heightStr) == 0 {
		height, err := chain.QueryLatestHeight()
		if err != nil {
			return 0, err
		}
		return height, nil
	}

	height, err := strconv.ParseInt(heightStr, 10, 64) //convert to int64
	if err != nil {
		return 0, err
	}
	return height, nil
}

// ParsePaginationParams parse limit and offset query params in request
func ParsePaginationParams(r *http.Request) (uint64, uint64, error) {
	offsetStr := r.URL.Query().Get("offset")
	limitStr := r.URL.Query().Get("limit")

	var offset, limit uint64
	var err error

	if len(offsetStr) != 0 {
		offset, err = strconv.ParseUint(offsetStr, 10, 64) //convert to int64
		if err != nil {
			return offset, limit, err
		}
	}

	if len(limitStr) != 0 {
		limit, err = strconv.ParseUint(limitStr, 10, 64) //convert to int64
		if err != nil {
			return offset, limit, err
		}
	}

	return offset, limit, nil
}
