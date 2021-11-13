package cosmos

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/store/rootmulti"
	querytypes "github.com/cosmos/cosmos-sdk/types/query"
	clienttypes "github.com/cosmos/ibc-go/v2/modules/core/02-client/types"
	committypes "github.com/cosmos/ibc-go/v2/modules/core/23-commitment/types"
	"github.com/cosmos/relayer/relayer"
	abci "github.com/tendermint/tendermint/abci/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client"

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

// isQueryStoreWithProof expects a format like /<queryType>/<storeName>/<subpath>
// queryType must be "store" and subpath must be "key" to require a proof.
func isQueryStoreWithProof(path string) bool {
	if !strings.HasPrefix(path, "/") {
		return false
	}

	paths := strings.SplitN(path[1:], "/", 3)
	switch {
	case len(paths) != 3:
		return false
	case paths[0] != "store":
		return false
	case rootmulti.RequireProof("/" + paths[2]):
		return true
	}

	return false
}

// QueryABCI is an affordance for querying the ABCI server associated with a chain
// Similar to cliCtx.QueryABCI
func (cp *CosmosProvider) QueryABCI(req abci.RequestQuery) (res abci.ResponseQuery, err error) {
	opts := rpcclient.ABCIQueryOptions{
		Height: req.GetHeight(),
		Prove:  req.Prove,
	}

	result, err := cp.Client.ABCIQueryWithOptions(context.Background(), req.Path, req.Data, opts)
	if err != nil {
		// retry queries on EOF
		if strings.Contains(err.Error(), "EOF") {
			if cp.debug {
				cp.Error(err)
			}
			return cp.QueryABCI(req)
		}
		return res, err
	}

	if !result.Response.IsOK() {
		return res, errors.New(result.Response.Log)
	}

	// data from trusted node or subspace query doesn't need verification
	if !isQueryStoreWithProof(req.Path) {
		return result.Response, nil
	}

	// TODO: figure out how to verify queries?

	return result.Response, nil
}

// QueryUpgradeProof performs an abci query with the given key and returns the proto encoded merkle proof
// for the query and the height at which the proof will succeed on a tendermint verifier.
func (cp *CosmosProvider) QueryUpgradeProof(key []byte, height uint64) ([]byte, clienttypes.Height, error) {
	res, err := cp.QueryABCI(abci.RequestQuery{
		Path:   "store/upgrade/key",
		Height: int64(height - 1),
		Data:   key,
		Prove:  true,
	})
	if err != nil {
		return nil, clienttypes.Height{}, err
	}

	merkleProof, err := committypes.ConvertProofs(res.ProofOps)
	if err != nil {
		return nil, clienttypes.Height{}, err
	}

	proof, err := cp.Encoding.Marshaler.Marshal(&merkleProof)
	if err != nil {
		return nil, clienttypes.Height{}, err
	}

	revision := clienttypes.ParseChainID(cp.Config.ChainID)

	// proof height + 1 is returned as the proof created corresponds to the height the proof
	// was created in the IAVL tree. Tendermint and subsequently the clients that rely on it
	// have heights 1 above the IAVL tree. Thus we return proof height + 1
	return proof, clienttypes.NewHeight(revision, uint64(res.Height+1)), nil
}

func DefaultPageRequest() *querytypes.PageRequest {
	return &querytypes.PageRequest{
		Key:        []byte(""),
		Offset:     0,
		Limit:      1000,
		CountTotal: true,
	}
}

// KeyOutput contains mnemonic and address of key
type KeyOutput struct {
	Mnemonic string `json:"mnemonic" yaml:"mnemonic"`
	Address  string `json:"address" yaml:"address"`
}

// KeyAddOrRestore is a helper function for add key and restores key when mnemonic is passed
func (cp *CosmosProvider) KeyAddOrRestore(keyName string, coinType uint32, mnemonic ...string) (KeyOutput, error) {
	var mnemonicStr string
	var err error

	if len(mnemonic) > 0 {
		mnemonicStr = mnemonic[0]
	} else {
		mnemonicStr, err = relayer.CreateMnemonic()
		if err != nil {
			return KeyOutput{}, err
		}
	}

	info, err := cp.Keybase.NewAccount(keyName, mnemonicStr, "", hd.CreateHDPath(coinType, 0, 0).String(), hd.Secp256k1)
	if err != nil {
		return KeyOutput{}, err
	}

	done := cp.UseSDKContext()
	ko := KeyOutput{Mnemonic: mnemonicStr, Address: info.GetAddress().String()}
	done()

	return ko, nil
}
