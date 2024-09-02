package client

import (
	"context"
	"time"

	"github.com/cometbft/cometbft/libs/bytes"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	tmtypes "github.com/cometbft/cometbft/types"
)

// make this so we can use the cometbft & gordian for this.
// TODO: first just get to work with cometbft so we can be happy
// Names use Get as to not collide with the cometbft types for namespaces
// TODO: simplify to JUST the data we need with our own relayer type objects
type ConsensusRelayerI interface {
	// DONE: (gordian /Block, just returns time.Now() for now. Need to impl proposer setting time)
	GetBlockTime(ctx context.Context, height uint64) (time.Time, error) // resultBlock.Block.Time (return the header annotation time.Time)

	GetStatus(ctx context.Context) (*coretypes.ResultStatus, error) // TODO:

	GetBlockResults(ctx context.Context, height uint64) (*coretypes.ResultBlockResults, error)                   // in (d *Driver) handleFinalization
	GetABCIQuery(ctx context.Context, queryPath string, data bytes.HexBytes) (*coretypes.ResultABCIQuery, error) // err != nil || resp.Response.Code != 0 // TODO: make this through baseapp `(app *BaseApp) Query` now? the store should handle

	GetTx(ctx context.Context, hash []byte, prove bool) (*coretypes.ResultTx, error) // resp (Events), err != nil - does this need its own tm store? or does the manager have context to this

	GetTxSearch(
		ctx context.Context,
		query string,
		prove bool,
		page, perPage *int,
		orderBy string,
	) (*coretypes.ResultTxSearch, error)

	GetBlockSearch(
		ctx context.Context,
		query string,
		page, perPage *int,
		orderBy string,
	) (*coretypes.ResultBlockSearch, error)

	GetCommit(ctx context.Context, height *int64) (*coretypes.ResultCommit, error)
	GetValidators(
		ctx context.Context,
		height *int64,
		page, perPage *int,
	) (*coretypes.ResultValidators, error)

	GetABCIQueryWithOptions(
		ctx context.Context,
		path string,
		data bytes.HexBytes,
		opts rpcclient.ABCIQueryOptions,
	) (*coretypes.ResultABCIQuery, error)

	DoBroadcastTxSync(ctx context.Context, tx tmtypes.Tx) (*coretypes.ResultBroadcastTx, error)
	DoBroadcastTxAsync(ctx context.Context, tx tmtypes.Tx) (*coretypes.ResultBroadcastTx, error)
}

// Not sure if we can re-use the cometbft type. Just trying to get exactly what we need at minimum
// type Block struct {
// 	Height uint64
// 	Time   time.Time
// }

// type Status struct {
// 	CatchingUp        bool
// 	LatestBlockHeight uint64
// }
