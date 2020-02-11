package relayer

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/store/rootmulti"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authTypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/ibc/02-client/exported"
	clientExported "github.com/cosmos/cosmos-sdk/x/ibc/02-client/exported"
	clientTypes "github.com/cosmos/cosmos-sdk/x/ibc/02-client/types"
	connTypes "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/types"
	"github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
	tendermint "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint"
	commitment "github.com/cosmos/cosmos-sdk/x/ibc/23-commitment"
	ibctypes "github.com/cosmos/cosmos-sdk/x/ibc/types"
	abci "github.com/tendermint/tendermint/abci/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
)

// NOTE: This file contains logic for querying the Tendermint RPC port of a configured chain
// All the operations here hit the network and data coming back may be untrusted.
// These functions by convention are named Query*

// TODO: Validate all info coming back from these queries using the verifier

// QueryConsensusState returns a consensus state for a given chain to be used as a
// client in another chain, fetches latest height when passed 0 as arg
func (c *Chain) QueryConsensusState(height int64) (*tmclient.ConsensusState, error) {
	var (
		commit *ctypes.ResultCommit
		err    error
	)
	if height == 0 {
		commit, err = c.Client.Commit(nil)
	} else {
		commit, err = c.Client.Commit(&height)
	}
	if err != nil {
		return nil, err
	}

	state := &tendermint.ConsensusState{
		Root:             commitment.NewRoot(commit.AppHash),
		ValidatorSetHash: commit.ValidatorsHash,
	}

	return state, nil
}

// QueryClientConsensusState retrevies the latest consensus state for a client in state at a given height
func (c *Chain) QueryClientConsensusState(clientID string, height uint64) (clientTypes.ConsensusStateResponse, error) {
	var conStateRes clientTypes.ConsensusStateResponse

	req := abci.RequestQuery{
		Path:   "store/ibc/key",
		Height: int64(height),
		Data:   ibctypes.KeyConsensusState(clientID, height),
		Prove:  true,
	}

	res, err := c.QueryABCI(req)
	if err != nil {
		return conStateRes, err
	}

	var cs exported.ConsensusState
	if err := c.Cdc.UnmarshalBinaryLengthPrefixed(res.Value, &cs); err != nil {
		return conStateRes, err
	}

	return clientTypes.NewConsensusStateResponse(clientID, cs, res.Proof, res.Height), nil
}

// QueryClientState retrevies the latest consensus state for a client in state at a given height
func (c *Chain) QueryClientState(clientID string) (clientTypes.StateResponse, error) {
	var conStateRes clientTypes.StateResponse

	req := abci.RequestQuery{
		Path:  "store/ibc/key",
		Data:  ibctypes.KeyClientState(clientID),
		Prove: true,
	}

	res, err := c.QueryABCI(req)
	if err != nil {
		return conStateRes, err
	}

	var cs exported.ClientState
	if err := c.Cdc.UnmarshalBinaryLengthPrefixed(res.Value, &cs); err != nil {
		return conStateRes, err
	}

	return clientTypes.NewClientStateResponse(clientID, cs, res.Proof, res.Height), nil
}

// QueryClients queries all the clients!
func (c *Chain) QueryClients(page, limit int) ([]clientExported.ClientState, error) {
	params := clientTypes.NewQueryAllClientsParams(page, limit)
	bz, err := c.Cdc.MarshalJSON(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query params: %w", err)
	}

	route := fmt.Sprintf("custom/%s/%s/%s", ibctypes.QuerierRoute, clientTypes.QuerierRoute, clientTypes.QueryAllClients)
	res, _, err := c.QueryWithData(route, bz)
	if err != nil {
		return nil, err
	}

	var clients []clientExported.ClientState
	err = c.Cdc.UnmarshalJSON(res, &clients)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal light clients: %w", err)
	}
	return clients, nil
}

// QueryConnectionsUsingClient gets any connections that exist between chain and counterparty
func (c *Chain) QueryConnectionsUsingClient(clientID string, height int64) (connTypes.ClientConnectionsResponse, error) {
	req := abci.RequestQuery{
		Path:   "store/ibc/key",
		Height: height,
		Data:   ibctypes.KeyClientConnections(clientID),
		Prove:  true,
	}

	res, err := c.QueryABCI(req)
	if err != nil {
		return connTypes.ClientConnectionsResponse{}, err
	}

	var paths []string
	if err := c.Cdc.UnmarshalBinaryLengthPrefixed(res.Value, &paths); err != nil {
		return connTypes.ClientConnectionsResponse{}, err
	}

	return connTypes.NewClientConnectionsResponse(clientID, paths, res.Proof, res.Height), nil
}

// QueryConnection returns the remote end of a given connection
func (c *Chain) QueryConnection(connectionID string, height int64) (connTypes.ConnectionResponse, error) {
	req := abci.RequestQuery{
		Path:   "store/ibc/key",
		Data:   ibctypes.KeyConnection(connectionID),
		Height: height,
		Prove:  true,
	}

	res, err := c.QueryABCI(req)
	if err != nil {
		return connTypes.ConnectionResponse{}, err
	}

	var connection connTypes.ConnectionEnd
	if err := c.Cdc.UnmarshalBinaryLengthPrefixed(res.Value, &connection); err != nil {
		return connTypes.ConnectionResponse{}, err
	}

	return connTypes.NewConnectionResponse(connectionID, connection, res.Proof, res.Height), nil
}

// QueryChannelsUsingConnections returns all channels associated with a given set of connections
func (c *Chain) QueryChannelsUsingConnections(connections []string) ([]chanTypes.ChannelResponse, error) {
	return []chanTypes.ChannelResponse{}, nil
}

// QueryChannel returns the channel associated with a channelID
func (c *Chain) QueryChannel(channelID, portID string, height int64) (chanTypes.ChannelResponse, error) {
	req := abci.RequestQuery{
		Path:   "store/ibc/key",
		Data:   ibctypes.KeyChannel(portID, channelID),
		Height: height,
		Prove:  true,
	}

	res, err := c.QueryABCI(req)
	if res.Value == nil || err != nil {
		return types.ChannelResponse{}, err
	}

	var channel types.Channel
	if err := c.Cdc.UnmarshalBinaryLengthPrefixed(res.Value, &channel); err != nil {
		return types.ChannelResponse{}, err
	}

	return types.NewChannelResponse(portID, channelID, channel, res.Proof, res.Height), nil
}

// QueryTxs returns an array of transactions given a tag
func (c *Chain) QueryTxs(height uint64, events []string) (*sdk.SearchTxsResult, error) {
	if len(events) == 0 {
		return nil, errors.New("must declare at least one event to search")
	}

	resTxs, err := c.Client.TxSearch(strings.Join(events, " AND "), true, 0, 10000, "asc")
	if err != nil {
		return nil, err
	}

	for _, tx := range resTxs.Txs {
		err := c.ValidateTxResult(tx)
		if err != nil {
			return nil, err
		}
	}

	resBlocks, err := c.queryBlocksForTxResults(resTxs.Txs)
	if err != nil {
		return nil, err
	}

	txs, err := c.formatTxResults(resTxs.Txs, resBlocks)
	if err != nil {
		return nil, err
	}

	result := sdk.NewSearchTxsResult(resTxs.TotalCount, len(txs), 0, 10000, txs)

	return &result, nil
}

// QueryABCI is an affordance for querying the ABCI server associated with a chain
// Similar to cliCtx.QueryABCI
func (c *Chain) QueryABCI(req abci.RequestQuery) (abci.ResponseQuery, error) {
	opts := rpcclient.ABCIQueryOptions{
		Height: req.GetHeight(),
		Prove:  req.Prove,
	}

	result, err := c.Client.ABCIQueryWithOptions(req.Path, req.Data, opts)
	if err != nil {
		return abci.ResponseQuery{}, err
	}

	if !result.Response.IsOK() {
		return abci.ResponseQuery{}, errors.New(result.Response.Log)
	}

	// data from trusted node or subspace query doesn't need verification
	if !isQueryStoreWithProof(req.Path) {
		return result.Response, nil
	}

	if err = c.VerifyProof(req.Path, result.Response); err != nil {
		return abci.ResponseQuery{}, err
	}

	return result.Response, nil
}

// QueryWithData satisfies auth.NodeQuerier interface and used for fetching account details
func (c *Chain) QueryWithData(path string, data []byte) ([]byte, int64, error) {
	req := abci.RequestQuery{
		Path:   path,
		Height: 0,
		Data:   data,
	}

	resp, err := c.QueryABCI(req)
	if err != nil {
		return []byte{}, 0, err
	}

	return resp.Value, resp.Height, nil
}

// QueryLatestHeight queries the chain for the latest height and returns it
func (c *Chain) QueryLatestHeight() (int64, error) {
	res, err := c.Client.Status()
	if err != nil {
		return -1, err
	}

	if res.SyncInfo.CatchingUp {
		return -1, fmt.Errorf("node at %s running chain %s not caught up", c.RPCAddr, c.ChainID)
	}

	return res.SyncInfo.LatestBlockHeight, nil
}

// QueryLatestHeader returns the latest header from the chain
func (c *Chain) QueryLatestHeader() (*tmclient.Header, error) {
	h, err := c.QueryLatestHeight()
	if err != nil {
		return nil, err
	}

	out, err := c.QueryHeaderAtHeight(h)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// QueryHeaderAtHeight returns the header at a given height
func (c *Chain) QueryHeaderAtHeight(height int64) (*tmclient.Header, error) {
	if height <= 0 {
		return nil, fmt.Errorf("must pass in valid height, %d not valid", height)
	}

	res, err := c.Client.Commit(&height)
	if err != nil {
		return nil, err
	}

	val, err := c.Client.Validators(&height, 0, 10000)
	if err != nil {
		return nil, err
	}

	return &tmclient.Header{
		// NOTE: This is not a SignedHeader
		// We are missing a lite.Commit type here
		SignedHeader: res.SignedHeader,
		ValidatorSet: tmtypes.NewValidatorSet(val.Validators),
	}, nil
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

// queryBlocksForTxResults returns a map[blockHeight]txResult
func (c *Chain) queryBlocksForTxResults(resTxs []*ctypes.ResultTx) (map[int64]*ctypes.ResultBlock, error) {
	resBlocks := make(map[int64]*ctypes.ResultBlock)

	for _, resTx := range resTxs {
		if _, ok := resBlocks[resTx.Height]; !ok {
			resBlock, err := c.Client.Block(&resTx.Height)
			if err != nil {
				return nil, err
			}

			resBlocks[resTx.Height] = resBlock
		}
	}

	return resBlocks, nil
}

// formatTxResults parses the indexed txs into a slice of TxResponse objects.
func (c *Chain) formatTxResults(resTxs []*ctypes.ResultTx, resBlocks map[int64]*ctypes.ResultBlock) ([]sdk.TxResponse, error) {
	var err error
	out := make([]sdk.TxResponse, len(resTxs))
	for i := range resTxs {
		out[i], err = c.formatTxResult(resTxs[i], resBlocks[resTxs[i].Height])
		if err != nil {
			return nil, err
		}
	}

	return out, nil
}

// formatTxResult parses a tx into a TxResponse object
func (c *Chain) formatTxResult(resTx *ctypes.ResultTx, resBlock *ctypes.ResultBlock) (sdk.TxResponse, error) {
	tx, err := parseTx(c.Cdc, resTx.Tx)
	if err != nil {
		return sdk.TxResponse{}, err
	}

	return sdk.NewResponseResultTx(resTx, tx, resBlock.Block.Time.Format(time.RFC3339)), nil
}

// Takes some bytes and a codec and returns an sdk.Tx
func parseTx(cdc *codec.Codec, txBytes []byte) (sdk.Tx, error) {
	var tx authTypes.StdTx

	err := cdc.UnmarshalBinaryLengthPrefixed(txBytes, &tx)
	if err != nil {
		return nil, err
	}

	return tx, nil
}
