package cclient

import (
	"context"
	"strings"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	bytes "github.com/cometbft/cometbft/libs/bytes"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	tmtypes "github.com/cometbft/cometbft/types"
	types "github.com/cosmos/cosmos-sdk/types"
)

// TODO(reece): get off cometbft types into internal relayer.
type ConsensusClient interface {
	GetBlockTime(ctx context.Context, height uint64) (time.Time, error)
	GetStatus(ctx context.Context) (*Status, error)
	GetBlockResults(ctx context.Context, height uint64) (*BlockResults, error)
	GetABCIQuery(ctx context.Context, queryPath string, data bytes.HexBytes) (*ABCIQueryResponse, error)
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
	DoBroadcastTxSync(ctx context.Context, tx []byte) (*TxResultResponse, error) // TODO: is tx []byte fine or does it need to be tx tmtypes.Tx?
	DoBroadcastTxAsync(ctx context.Context, tx tmtypes.Tx) (*TxResultResponse, error)
	GetTx(ctx context.Context, hash []byte, psrove bool) (*coretypes.ResultTx, error)
	GetBlockSearch(
		ctx context.Context,
		query string,
		page, perPage *int,
		orderBy string,
	) (*coretypes.ResultBlockSearch, error)
	GetCommit(ctx context.Context, height uint64) (*ResultCommit, error)
	GetABCIQueryWithOptions(
		ctx context.Context,
		path string,
		data bytes.HexBytes,
		opts rpcclient.ABCIQueryOptions,
	) (*coretypes.ResultABCIQuery, error)

	SimulateTransaction(ctx context.Context, tx []byte, cfg *SimTxConfig) (types.GasInfo, error)
}

type ResultCommit struct {
	Time    time.Time `json:"time"`
	AppHash []byte    `json:"app_hash"`
}

type SimTxConfig struct {
	// CometBFT only function (QueryABCI).
	QueryABCIFunc func(ctx context.Context, req abci.RequestQuery) (abci.ResponseQuery, error)
}

type Status struct {
	CatchingUp        bool
	LatestBlockHeight uint64
}

type BlockResults struct {
	FinalizeBlockEvents []abci.Event         `json:"finalize_block_events"`
	TxsResults          []*abci.ExecTxResult `json:"txs_results"`
}

type ABCIQueryResponse struct {
	Code  uint32 `json:"code,omitempty"`
	Value []byte `json:"value,omitempty"`
}

// The response value contains the data link escape control character which must be removed before parsing.
func (q ABCIQueryResponse) ValueCleaned() string {
	return strings.ReplaceAll(strings.TrimSpace(string(q.Value)), "\u0010", "")
}

// coretypes.ResultTxSearch
type ResultTxSearch struct {
	Txs []*coretypes.ResultTx `json:"txs"`
}

type ResultValidators struct {
	Validators []*tmtypes.Validator `json:"validators"`
}

// type Validator struct {
// 	Address          crypto.Address `json:"address"`
// 	PubKey           crypto.PubKey  `json:"pub_key"`
// 	VotingPower      int64          `json:"voting_power"`
// 	ProposerPriority int64          `json:"proposer_priority"`
// }

type ResultBroadcastTx struct {
	Code      uint32         `json:"code"`
	Data      bytes.HexBytes `json:"data"`
	Log       string         `json:"log"`
	Codespace string         `json:"codespace"`
	Hash      bytes.HexBytes `json:"hash"`
}

type TxResultResponse struct {
	Events []*Event `protobuf:"bytes,1,rep,name=events,proto3" json:"events,omitempty"`
	// bytes resp = 2; //  []transaction.Msg
	Error     string `protobuf:"bytes,3,opt,name=error,proto3" json:"error,omitempty"`
	Code      uint32 `protobuf:"varint,4,opt,name=code,proto3" json:"code,omitempty"`
	Data      []byte `protobuf:"bytes,5,opt,name=data,proto3" json:"data,omitempty"`
	Log       string `protobuf:"bytes,6,opt,name=log,proto3" json:"log,omitempty"`
	Info      string `protobuf:"bytes,7,opt,name=info,proto3" json:"info,omitempty"`
	GasWanted uint64 `protobuf:"varint,8,opt,name=gas_wanted,proto3" json:"gas_wanted,omitempty"`
	GasUsed   uint64 `protobuf:"varint,9,opt,name=gas_used,proto3" json:"gas_used,omitempty"`
	Codespace string `protobuf:"bytes,10,opt,name=codespace,proto3" json:"codespace,omitempty"`
	TxHash    string `protobuf:"bytes,11,opt,name=tx_hash,proto3" json:"tx_hash,omitempty"`
}

type Event struct {
	Type       string            `protobuf:"bytes,1,opt,name=type,proto3" json:"type,omitempty"`
	Attributes []*EventAttribute `protobuf:"bytes,2,rep,name=attributes,proto3" json:"attributes,omitempty"`
}

func convertConsensusEvents(e []*Event) []abci.Event {
	events := make([]abci.Event, len(e))
	for _, ev := range e {
		attributes := make([]abci.EventAttribute, len(ev.Attributes))
		for idx, attr := range ev.Attributes {
			attributes[idx] = abci.EventAttribute{
				Key:   attr.Key,
				Value: attr.Value,
			}
		}

		events = append(events, abci.Event{
			Type:       ev.Type,
			Attributes: attributes,
		})
	}
	return events
}

type EventAttribute struct {
	Key   string `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	Value string `protobuf:"bytes,2,opt,name=value,proto3" json:"value,omitempty"`
}
