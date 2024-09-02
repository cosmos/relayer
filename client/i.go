package client

import (
	"context"
	"strings"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	tmtypes "github.com/cometbft/cometbft/types"

	bytes "github.com/cometbft/cometbft/libs/bytes"
	rbytes "github.com/cosmos/relayer/v2/client/bytes"
)

// make this so we can use the cometbft & gordian for this.
// TODO: first just get to work with cometbft so we can be happy
// Names use Get as to not collide with the cometbft types for namespaces
// TODO: simplify to JUST the data we need with our own relayer type objects
type ConsensusRelayerI interface {
	// DONE: (gordian /Block, just returns time.Now() for now. Need to impl proposer setting time)
	GetBlockTime(ctx context.Context, height uint64) (time.Time, error) // resultBlock.Block.Time (return the header annotation time.Time)
	GetStatus(ctx context.Context) (*Status, error)
	GetBlockResults(ctx context.Context, height uint64) (*BlockResults, error)                           // in (d *Driver) handleFinalization
	GetABCIQuery(ctx context.Context, queryPath string, data bytes.HexBytes) (*ABCIQueryResponse, error) // err != nil || resp.Response.Code != 0 // TODO: make this through baseapp `(app *BaseApp) Query` now? the store should handle
	GetValidators(
		ctx context.Context,
		height *int64,
		page, perPage *int,
	) (*ResultValidators, error)
	GetTxSearch(
		ctx context.Context,
		query string,
		prove bool,
		page, perPage *int,
		orderBy string,
	) (*ResultTxSearch, error)
	DoBroadcastTxSync(ctx context.Context, tx tmtypes.Tx) (*ResultBroadcastTx, error)
	DoBroadcastTxAsync(ctx context.Context, tx tmtypes.Tx) (*ResultBroadcastTx, error)

	// TODO: migrate with v2
	GetTx(ctx context.Context, hash []byte, prove bool) (*coretypes.ResultTx, error) // resp (Events), err != nil - does this need its own tm store? or does the manager have context to this

	GetBlockSearch(
		ctx context.Context,
		query string,
		page, perPage *int,
		orderBy string,
	) (*coretypes.ResultBlockSearch, error)

	GetCommit(ctx context.Context, height *int64) (*coretypes.ResultCommit, error)
	GetABCIQueryWithOptions(
		ctx context.Context,
		path string,
		data bytes.HexBytes,
		opts rpcclient.ABCIQueryOptions,
	) (*coretypes.ResultABCIQuery, error)
}

// Not sure if we can re-use the cometbft type. Just trying to get exactly what we need at minimum
// type Block struct {
// 	Height uint64
// 	Time   time.Time
// }

type Status struct {
	CatchingUp        bool
	LatestBlockHeight uint64
}

type BlockResults struct {
	// TODO: ideally we get off of this into our own internal type. Then the ConsensusRelayerI can have methods to convert
	FinalizeBlockEvents []abci.Event         `json:"finalize_block_events"`
	TxsResults          []*abci.ExecTxResult `json:"txs_results"`
}

type ABCIQueryResponse struct {
	Code  uint32 `json:"code,omitempty"`
	Value []byte `json:"value,omitempty"`
}

func (q ABCIQueryResponse) ValueCleaned() string {
	// The response value contains the data link escape control character which must be removed before parsing.
	return strings.ReplaceAll(strings.TrimSpace(string(q.Value)), "\u0010", "")
}

// TODO: can't do this yet as the cosmos-sdk side in v0.50 is tied to cometbft
// type Transaction struct {
// 	Height uint64
// 	TxHash []byte
// 	Code   uint32
// 	Data   string
// 	Events []abci.Event // TODO: []provider.RelayerEvent
// 	Tx     cmtypes.Tx   `json:"tx"`
// }

// coretypes.ResultTxSearch
type ResultTxSearch struct {
	Txs        []*coretypes.ResultTx `json:"txs"`
	TotalCount int                   `json:"total_count"`
}

type ResultValidators struct {
	Validators []*tmtypes.Validator `json:"validators"`
	// Validators []Validator // TODO: requires some helper methods on the gordian side for the query to update set stuff
}

type Validator struct {
	Address          crypto.Address `json:"address"`
	PubKey           crypto.PubKey  `json:"pub_key"`
	VotingPower      int64          `json:"voting_power"`
	ProposerPriority int64          `json:"proposer_priority"`
}

type ResultBroadcastTx struct {
	Code      uint32          `json:"code"`
	Data      rbytes.HexBytes `json:"data"`
	Log       string          `json:"log"`
	Codespace string          `json:"codespace"`
	Hash      rbytes.HexBytes `json:"hash"`
}
