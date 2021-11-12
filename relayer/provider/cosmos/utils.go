package cosmos

import (
	"path"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"

	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	libclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
)

func KeysDir(home, chainID string) string {
	return path.Join(home, "keys", chainID)
}

func newRPCClient(addr string, timeout time.Duration) (*rpchttp.HTTP, error) {
	httpClient, err := libclient.DefaultHTTPClient(addr)
	if err != nil {
		return nil, err
	}

	httpClient.Timeout = timeout
	rpcClient, err := rpchttp.NewWithClient(addr, "/websocket", httpClient)
	if err != nil {
		return nil, err
	}

	return rpcClient, nil
}

// GetAddress returns the sdk.AccAddress associated with the configured key
func (cp *CosmosProvider) GetAddress() (sdk.AccAddress, error) {
	done := cp.UseSDKContext()
	defer done()
	if cp.address != nil {
		return cp.address, nil
	}

	// Signing key for c chain
	srcAddr, err := cp.Keybase.Key(cp.Config.Key)
	if err != nil {
		return nil, err
	}

	return srcAddr.GetAddress(), nil
}

// MustGetAddress used for brevity
func (cp *CosmosProvider) MustGetAddress() string {
	srcAddr, err := cp.GetAddress()
	if err != nil {
		panic(err)
	}
	done := cp.UseSDKContext()
	defer done()
	return srcAddr.String()
}
