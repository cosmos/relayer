package cclient

import (
	"context"
	"fmt"
	"time"

	"github.com/avast/retry-go/v4"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/libs/bytes"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	tmtypes "github.com/cometbft/cometbft/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
)

var _ ConsensusClient = (*CometRPCClient)(nil)

var (
	// originally from relayer/chains/cosmos/tx.go
	rtyAttNum = uint(5)
	rtyAtt    = retry.Attempts(rtyAttNum)
	rtyDel    = retry.Delay(time.Millisecond * 400)
	rtyErr    = retry.LastErrorOnly(true)
)

// GetBlock implements ConsensusClient.
func (r CometRPCClient) GetBlockTime(ctx context.Context, height uint64) (time.Time, error) {
	h := int64(height)

	b, err := r.Block(ctx, &h)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get block: %w", err)
	}

	return b.Block.Header.Time, nil
}

// GetBlockResults implements ConsensusClient.
func (r CometRPCClient) GetBlockResults(ctx context.Context, height uint64) (*BlockResults, error) {
	h := int64(height)
	br, err := r.BlockResults(ctx, &h)
	if err != nil {
		return nil, fmt.Errorf("failed to get block results: %w", err)
	}
	return &BlockResults{
		TxsResults:          br.TxsResults,
		FinalizeBlockEvents: br.FinalizeBlockEvents,
	}, nil
}

// GetABCIQuery implements ConsensusClient.
func (r CometRPCClient) GetABCIQuery(ctx context.Context, queryPath string, data bytes.HexBytes) (*ABCIQueryResponse, error) {
	resp, err := r.ABCIQuery(ctx, queryPath, data)
	if err != nil {
		return nil, fmt.Errorf("failed to get ABCI query: %w", err)
	}
	return &ABCIQueryResponse{
		Code:  resp.Response.Code,
		Value: resp.Response.Value,
	}, nil
}

// GetTx implements ConsensusClient.
func (r CometRPCClient) GetTx(ctx context.Context, hash []byte, prove bool) (*coretypes.ResultTx, error) {
	resp, err := r.Tx(ctx, hash, prove)
	if err != nil {
		return nil, fmt.Errorf("failed to get tx: %w", err)
	}
	return resp, nil
}

// GetTxSearch implements ConsensusClient.
func (r CometRPCClient) GetTxSearch(ctx context.Context, query string, prove bool, page *int, perPage *int, orderBy string) (*ResultTxSearch, error) {
	resp, err := r.TxSearch(ctx, query, prove, page, perPage, orderBy)
	if err != nil {
		return nil, fmt.Errorf("failed to get tx search: %w", err)
	}
	return &ResultTxSearch{
		Txs: resp.Txs,
	}, nil
}

// GetBlockSearch implements ConsensusClient.
func (r CometRPCClient) GetBlockSearch(ctx context.Context, query string, page *int, perPage *int, orderBy string) (*coretypes.ResultBlockSearch, error) {
	resp, err := r.BlockSearch(ctx, query, page, perPage, orderBy)
	if err != nil {
		return nil, fmt.Errorf("failed to get block search: %w", err)
	}
	return resp, nil
}

// GetCommit implements ConsensusClient.
func (r CometRPCClient) GetCommit(ctx context.Context, height uint64) (*ResultCommit, error) {
	h := int64(height)
	c, err := r.Commit(ctx, &h)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit: %w", err)
	}
	return &ResultCommit{
		AppHash: c.AppHash,
		Time:    c.Time,
	}, nil
}

// GetValidators implements ConsensusClient.
func (r CometRPCClient) GetValidators(ctx context.Context, height *int64, page *int, perPage *int) (*ResultValidators, error) {
	v, err := r.Validators(ctx, height, page, perPage)
	if err != nil {
		return nil, fmt.Errorf("failed to get validators: %w", err)
	}

	return &ResultValidators{
		Validators: v.Validators,
	}, nil
}

// DoBroadcastTxAsync implements ConsensusClient.
func (r CometRPCClient) DoBroadcastTxAsync(ctx context.Context, tx tmtypes.Tx) (*TxResultResponse, error) {
	b, err := r.BroadcastTxAsync(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to broadcast tx async: %w", err)
	}
	return &TxResultResponse{
		Code:      b.Code,
		Data:      b.Data,
		Log:       b.Log,
		Codespace: b.Codespace,
		TxHash:    string(b.Hash),
	}, nil
}

// DoBroadcastTxSync implements ConsensusClient.
func (r CometRPCClient) DoBroadcastTxSync(ctx context.Context, tx []byte) (*TxResultResponse, error) {
	b, err := r.BroadcastTxSync(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to broadcast tx sync: %w", err)
	}
	return &TxResultResponse{
		Code:      b.Code,
		Data:      b.Data,
		Log:       b.Log,
		Codespace: b.Codespace,
		TxHash:    string(b.Hash),
	}, nil
}

// SimulateTransaction implements ConsensusClient.
func (r CometRPCClient) SimulateTransaction(ctx context.Context, tx []byte, cfg *SimTxConfig) (sdk.GasInfo, error) {
	simQuery := abci.RequestQuery{
		Path: "/cosmos.tx.v1beta1.Service/Simulate",
		Data: tx,
	}

	if cfg == nil {
		return sdk.GasInfo{}, fmt.Errorf("BUG: SimulateTransaction cfg is nil, cfg.QueryABCIFunc is required for CometRPCClient")
	}

	var res abci.ResponseQuery
	if err := retry.Do(func() error {
		var err error
		res, err = cfg.QueryABCIFunc(ctx, simQuery)
		if err != nil {
			return err
		}
		return nil
	}, retry.Context(ctx), rtyAtt, rtyDel, rtyErr); err != nil {
		return sdk.GasInfo{}, err
	}

	var simRes txtypes.SimulateResponse
	if err := simRes.Unmarshal(res.Value); err != nil {
		return sdk.GasInfo{}, err
	}

	return sdk.GasInfo{
		GasWanted: simRes.GasInfo.GasWanted,
		GasUsed:   simRes.GasInfo.GasUsed,
	}, nil
}

// GetABCIQueryWithOptions implements ConsensusClient.
func (r CometRPCClient) GetABCIQueryWithOptions(ctx context.Context, path string, data bytes.HexBytes, opts rpcclient.ABCIQueryOptions) (*coretypes.ResultABCIQuery, error) {
	q, err := r.ABCIQueryWithOptions(ctx, path, data, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get ABCI query with options: %w", err)
	}
	return q, nil
}

// GetStatus implements ConsensusClient.
func (r CometRPCClient) GetStatus(ctx context.Context) (*Status, error) {
	s, err := r.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}
	return &Status{
		CatchingUp:        s.SyncInfo.CatchingUp,
		LatestBlockHeight: uint64(s.SyncInfo.LatestBlockHeight),
	}, nil
}
