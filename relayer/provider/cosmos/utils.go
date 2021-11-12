package cosmos

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	sdkCtx "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authTypes "github.com/cosmos/cosmos-sdk/x/auth/types"
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

func getMsgAction(msgs []sdk.Msg) string {
	var out string
	for i, msg := range msgs {
		out += fmt.Sprintf("%d:%s,", i, sdk.MsgTypeURL(msg))
	}
	return strings.TrimSuffix(out, ",")
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

// CLIContext returns an instance of client.Context derived from Chain
func (cp *CosmosProvider) CLIContext(height int64) sdkCtx.Context {
	addr, err := cp.GetAddress()
	if err != nil {
		panic(err)
	}
	return sdkCtx.Context{}.
		WithChainID(cp.Config.ChainID).
		WithCodec(cp.Encoding.Marshaler).
		WithInterfaceRegistry(cp.Encoding.InterfaceRegistry).
		WithTxConfig(cp.Encoding.TxConfig).
		WithLegacyAmino(cp.Encoding.Amino).
		WithInput(os.Stdin).
		WithNodeURI(cp.Config.RPCAddr).
		WithClient(cp.Client).
		WithAccountRetriever(authTypes.AccountRetriever{}).
		WithBroadcastMode(flags.BroadcastBlock).
		WithKeyring(cp.Keybase).
		WithOutputFormat("json").
		WithFrom(cp.Config.Key).
		WithFromName(cp.Config.Key).
		WithFromAddress(addr).
		WithSkipConfirmation(true).
		WithNodeURI(cp.Config.RPCAddr).
		WithHeight(height)
}
