package client

import (
	"context"
	"fmt"
	"time"

	"github.com/cometbft/cometbft/libs/bytes"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	tmtypes "github.com/cometbft/cometbft/types"
)

// ConsensusRelayerI is the itnerface we will use across the relayer so we can swap out the underlying consensus engine client.
var _ ConsensusRelayerI = (*CometRPCClient)(nil)

// GetBlock implements ConsensusRelayerI.
func (r CometRPCClient) GetBlockTime(ctx context.Context, height uint64) (time.Time, error) {
	h := int64(height)

	b, err := r.Block(ctx, &h)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get block: %w", err)
	}

	return b.Block.Header.Time, nil
}

// GetBlockResults implements ConsensusRelayerI.
func (r CometRPCClient) GetBlockResults(ctx context.Context, height uint64) (*coretypes.ResultBlockResults, error) {
	h := int64(height)
	br, err := r.BlockResults(ctx, &h)
	if err != nil {
		return nil, fmt.Errorf("failed to get block results: %w", err)
	}
	return br, nil
}

// GetABCIQuery implements ConsensusRelayerI.
func (r CometRPCClient) GetABCIQuery(ctx context.Context, queryPath string, data bytes.HexBytes) (*coretypes.ResultABCIQuery, error) {
	resp, err := r.ABCIQuery(ctx, queryPath, data)
	if err != nil {
		return nil, fmt.Errorf("failed to get ABCI query: %w", err)
	}
	return resp, nil
}

// GetTx implements ConsensusRelayerI.
func (r CometRPCClient) GetTx(ctx context.Context, hash []byte, prove bool) (*coretypes.ResultTx, error) {
	resp, err := r.Tx(ctx, hash, prove)
	if err != nil {
		return nil, fmt.Errorf("failed to get tx: %w", err)
	}
	return resp, nil
}

// GetTxSearch implements ConsensusRelayerI.
func (r CometRPCClient) GetTxSearch(ctx context.Context, query string, prove bool, page *int, perPage *int, orderBy string) (*coretypes.ResultTxSearch, error) {
	resp, err := r.TxSearch(ctx, query, prove, page, perPage, orderBy)
	if err != nil {
		return nil, fmt.Errorf("failed to get tx search: %w", err)
	}
	return resp, nil
}

// GetBlockSearch implements ConsensusRelayerI.
func (r CometRPCClient) GetBlockSearch(ctx context.Context, query string, page *int, perPage *int, orderBy string) (*coretypes.ResultBlockSearch, error) {
	resp, err := r.BlockSearch(ctx, query, page, perPage, orderBy)
	if err != nil {
		return nil, fmt.Errorf("failed to get block search: %w", err)
	}
	return resp, nil
}

// GetCommit implements ConsensusRelayerI.
func (r CometRPCClient) GetCommit(ctx context.Context, height *int64) (*coretypes.ResultCommit, error) {
	panic("unimplemented")
}

// GetValidators implements ConsensusRelayerI.
func (r CometRPCClient) GetValidators(ctx context.Context, height *int64, page *int, perPage *int) (*coretypes.ResultValidators, error) {
	panic("unimplemented")
}

// DoBroadcastTxAsync implements ConsensusRelayerI.
func (r CometRPCClient) DoBroadcastTxAsync(ctx context.Context, tx tmtypes.Tx) (*coretypes.ResultBroadcastTx, error) {
	panic("unimplemented")
}

// DoBroadcastTxSync implements ConsensusRelayerI.
func (r CometRPCClient) DoBroadcastTxSync(ctx context.Context, tx tmtypes.Tx) (*coretypes.ResultBroadcastTx, error) {
	panic("unimplemented")
}

// GetABCIQueryWithOptions implements ConsensusRelayerI.
func (r CometRPCClient) GetABCIQueryWithOptions(ctx context.Context, path string, data bytes.HexBytes, opts rpcclient.ABCIQueryOptions) (*coretypes.ResultABCIQuery, error) {
	panic("unimplemented")
}

// GetStatus implements ConsensusRelayerI.
func (r CometRPCClient) GetStatus(ctx context.Context) (*coretypes.ResultStatus, error) {
	panic("unimplemented")
}
