package cclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/cometbft/cometbft/libs/bytes"
	"github.com/cometbft/cometbft/rpc/client"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cometbft/cometbft/types"
)

var _ ConsensusClient = (*GordianConsensus)(nil)

type GordianConsensus struct {
	// temp until IBC-Go is updated so we can import & use gRPC
	addr string
}

func NewGordianConsensus(addr string) *GordianConsensus {
	return &GordianConsensus{
		addr: addr,
	}
}

// DoBroadcastTxAsync implements ConsensusClient.
func (g *GordianConsensus) DoBroadcastTxAsync(ctx context.Context, tx types.Tx) (*ResultBroadcastTx, error) {
	panic("unimplemented")
}

// DoBroadcastTxSync implements ConsensusClient.
func (g *GordianConsensus) DoBroadcastTxSync(ctx context.Context, tx types.Tx) (*ResultBroadcastTx, error) {
	panic("unimplemented")
}

// GetABCIQuery implements ConsensusClient.
func (g *GordianConsensus) GetABCIQuery(ctx context.Context, queryPath string, data bytes.HexBytes) (*ABCIQueryResponse, error) {
	panic("unimplemented")
}

// GetABCIQueryWithOptions implements ConsensusClient.
func (g *GordianConsensus) GetABCIQueryWithOptions(ctx context.Context, path string, data bytes.HexBytes, opts client.ABCIQueryOptions) (*coretypes.ResultABCIQuery, error) {
	panic("unimplemented")
}

// GetBlockResults implements ConsensusClient.
func (g *GordianConsensus) GetBlockResults(ctx context.Context, height uint64) (*BlockResults, error) {
	panic("unimplemented")
}

// GetBlockSearch implements ConsensusClient.
func (g *GordianConsensus) GetBlockSearch(ctx context.Context, query string, page *int, perPage *int, orderBy string) (*coretypes.ResultBlockSearch, error) {
	panic("unimplemented")
}

// GetBlockTime implements ConsensusClient.
func (g *GordianConsensus) GetBlockTime(ctx context.Context, height uint64) (time.Time, error) {
	res, err := http.Get(fmt.Sprintf("%s/block/%d", g.addr, height))
	if err != nil {
		fmt.Printf("error making http request: %s\n", err)
		os.Exit(1)
	}

	// decode into type (copy pasted from gserver/internal/ggrpc
	type GetBlockResponse struct {
		Time uint64 `protobuf:"varint,1,opt,name=time,proto3" json:"time,omitempty"` // nanoseconds
	}

	var resp GetBlockResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		fmt.Printf("error decoding response: %s\n", err)
		os.Exit(1)
	}

	return time.Unix(int64(resp.Time), 0), nil
}

// GetCommit implements ConsensusClient.
func (g *GordianConsensus) GetCommit(ctx context.Context, height uint64) (*coretypes.ResultCommit, error) {
	panic("unimplemented")
}

// GetStatus implements ConsensusClient.
func (g *GordianConsensus) GetStatus(ctx context.Context) (*Status, error) {
	panic("unimplemented")
}

// GetTx implements ConsensusClient.
func (g *GordianConsensus) GetTx(ctx context.Context, hash []byte, prove bool) (*coretypes.ResultTx, error) {
	panic("unimplemented")
}

// GetTxSearch implements ConsensusClient.
func (g *GordianConsensus) GetTxSearch(ctx context.Context, query string, prove bool, page *int, perPage *int, orderBy string) (*ResultTxSearch, error) {
	panic("unimplemented")
}

// GetValidators implements ConsensusClient.
func (g *GordianConsensus) GetValidators(ctx context.Context, height *int64, page *int, perPage *int) (*ResultValidators, error) {
	panic("unimplemented")
}
