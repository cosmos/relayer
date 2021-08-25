package helpers

import (
	"strconv"

	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/relayer"
)

// GetLightHeader returns header with chain and optional height as inputs
func GetLightHeader(chain *relayer.Chain, opts ...string) (*tmclient.Header, error) {
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
		}

		return chain.GetLightSignedHeaderAtHeight(height)
	}

	return chain.GetIBCCreateClientHeader()
}
