package cclient

import (
	"context"
	"fmt"
	"time"

	"github.com/cometbft/cometbft/libs/bytes"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	tmtypes "github.com/cometbft/cometbft/types"
)

var _ ConsensusClient = (*CometRPCClient)(nil)

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
func (r CometRPCClient) GetCommit(ctx context.Context, height uint64) (*coretypes.ResultCommit, error) {
	h := int64(height)
	c, err := r.Commit(ctx, &h)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit: %w", err)
	}
	return c, nil
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
