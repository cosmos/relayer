package helpers

import (
	"fmt"
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
			height, err = chain.GetLatestLightHeight()
			if err != nil {
				return nil, err
			}

			if height < 0 {
				return nil, relayer.ErrLightNotInitialized
			}
		}

		return chain.GetLightSignedHeaderAtHeight(height)
	}

	return chain.GetLatestLightHeader()
}

// InitLight is a helper function for init light
func InitLight(chain *relayer.Chain, force bool, height int64, hash []byte) (string, error) {
	db, df, err := chain.NewLightDB()
	if err != nil {
		return "", err
	}
	defer df()

	switch {
	case force: // force initialization from trusted node
		_, err := chain.LightClientWithoutTrust(db)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("successfully created light client for %s by trusting endpoint %s...",
			chain.ChainID, chain.RPCAddr), nil
	case height > 0 && len(hash) > 0: // height and hash are given
		_, err = chain.LightClientWithTrust(db, chain.TrustOptions(height, hash))
		if err != nil {
			return "", wrapInitFailed(err)
		}
		return "", nil
	default: // return error
		return "", errInitWrongFlags
	}
}

// UpdateLight is a helper function for update light
func UpdateLight(chain *relayer.Chain) (string, error) {
	bh, err := chain.GetLatestLightHeader()
	if err != nil {
		return "", err
	}

	lightBlock, err := chain.UpdateLightClient()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Updated light client for %s from height %d -> height %d",
		chain.ChainID, bh.Header.Height, lightBlock.Header.Height), nil
}
