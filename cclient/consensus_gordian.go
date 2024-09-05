package cclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto"
	cmtbytes "github.com/cometbft/cometbft/libs/bytes"
	"github.com/cometbft/cometbft/rpc/client"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cometbft/cometbft/types"
	tmtypes "github.com/cometbft/cometbft/types"
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

//  -----

// DoBroadcastTxAsync implements ConsensusClient.
func (g *GordianConsensus) DoBroadcastTxAsync(ctx context.Context, tx types.Tx) (*ResultBroadcastTx, error) {
	panic("unimplemented")
}

// DoBroadcastTxSync implements ConsensusClient.
func (g *GordianConsensus) DoBroadcastTxSync(ctx context.Context, tx []byte) (*TxResultResponse, error) {
	var body io.Reader
	if tx != nil {
		body = bytes.NewReader(tx)
	} else {
		return nil, fmt.Errorf("DoBroadcastTxSync tx is nil")
	}

	res, err := http.Post(fmt.Sprintf("%s/debug/submit_tx", g.addr), "application/json", body)
	if err != nil {
		fmt.Printf("error making http request: %s\n", err)
		os.Exit(1)
	}

	var resp TxResultResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		fmt.Printf("error decoding response: %s\n", err)
		return nil, err
	}

	return &resp, nil

}

// GetABCIQuery implements ConsensusClient.
func (g *GordianConsensus) GetABCIQuery(ctx context.Context, queryPath string, data cmtbytes.HexBytes) (*ABCIQueryResponse, error) {
	panic("unimplemented")
}

// GetABCIQueryWithOptions implements ConsensusClient.
func (g *GordianConsensus) GetABCIQueryWithOptions(ctx context.Context, path string, data cmtbytes.HexBytes, opts client.ABCIQueryOptions) (*coretypes.ResultABCIQuery, error) {
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
	res, err := http.Get(fmt.Sprintf("%s/status", g.addr))
	if err != nil {
		fmt.Printf("error making http request: %s\n", err)
		os.Exit(1)
	}

	// decode into type (copy pasted from gserver/internal/ggrpc
	type GetStatusResponse struct {
		CatchingUp        bool   `protobuf:"varint,1,opt,name=catching_up,json=catchingUp,proto3" json:"catching_up,omitempty"`
		LatestBlockHeight uint64 `protobuf:"varint,2,opt,name=latest_block_height,json=latestBlockHeight,proto3" json:"latest_block_height,omitempty"`
	}

	var resp GetStatusResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		fmt.Printf("error decoding response: %s\n", err)
		os.Exit(1)
	}

	return &Status{
		CatchingUp:        resp.CatchingUp,
		LatestBlockHeight: resp.LatestBlockHeight,
	}, nil
}

// GetTx implements ConsensusClient.
func (g *GordianConsensus) GetTx(ctx context.Context, hash []byte, prove bool) (*coretypes.ResultTx, error) {
	res, err := http.Get(fmt.Sprintf("%s/tx/%s", g.addr, hash))
	if err != nil {
		fmt.Printf("error making http request: %s\n", err)
		os.Exit(1)
	}

	var resp TxResultResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		fmt.Printf("error decoding response: %s\n", err)
		os.Exit(1)
	}

	return &coretypes.ResultTx{
		Hash:   cmtbytes.HexBytes(resp.TxHash),
		Height: 999, // TODO: debugging
		TxResult: abci.ExecTxResult{
			Code:      resp.Code,
			Data:      resp.Data,
			Log:       resp.Log,
			Info:      resp.Info,
			GasWanted: int64(resp.GasWanted),
			GasUsed:   int64(resp.GasUsed),
			Events:    convertConsensusEvents(resp.Events),
			Codespace: resp.Codespace,
		},
	}, nil
}

// GetTxSearch implements ConsensusClient.
func (g *GordianConsensus) GetTxSearch(ctx context.Context, query string, prove bool, page *int, perPage *int, orderBy string) (*ResultTxSearch, error) {
	panic("unimplemented")
}

// TODO: GetValidators needs pubkey -> address conversions
// GetValidators implements ConsensusClient.
func (g *GordianConsensus) GetValidators(ctx context.Context, height *int64, page *int, perPage *int) (*ResultValidators, error) {
	// coppied & modified namespace to GordianValidator
	type GordianValidator struct {
		EncodedPubKey []byte `protobuf:"bytes,1,opt,name=encoded_pub_key,json=encodedPubKey,proto3" json:"encoded_pub_key,omitempty"`
		Power         uint64 `protobuf:"varint,2,opt,name=power,proto3" json:"power,omitempty"`
	}
	type GetValidatorsResponse struct {
		FinalizationHeight *uint64             `protobuf:"varint,1,opt,name=finalization_height,json=finalizationHeight,proto3,oneof" json:"finalization_height,omitempty"`
		Validators         []*GordianValidator `protobuf:"bytes,2,rep,name=validators,proto3" json:"validators,omitempty"`
	}

	res, err := http.Get(fmt.Sprintf("%s/validators", g.addr))
	if err != nil {
		fmt.Printf("error making http request: %s\n", err)
		os.Exit(1)
	}

	var resp GetValidatorsResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		fmt.Printf("error decoding response: %s\n", err)
		os.Exit(1)
	}

	converted := make([]*tmtypes.Validator, len(resp.Validators))

	for i, v := range resp.Validators {
		tmk := &tmPubKeyHack{pubKey: v.EncodedPubKey}

		converted[i] = &tmtypes.Validator{
			Address:          tmk.Address(),
			PubKey:           tmk,
			VotingPower:      int64(v.Power),
			ProposerPriority: -1, // TODO: do we need this for any reason?
		}
	}

	return &ResultValidators{
		Validators: converted,
	}, nil
}

var _ crypto.PubKey = (*tmPubKeyHack)(nil)

// tmPubKeyHack is a temp workaround to make pubkeys happy. In the future we can build a better interface wrapper
// or struct using the gordian crypto lib.
type tmPubKeyHack struct {
	pubKey []byte
}

// Address implements crypto.PubKey.
func (t *tmPubKeyHack) Address() cmtbytes.HexBytes {
	return t.pubKey
}

// Bytes implements crypto.PubKey.
func (t *tmPubKeyHack) Bytes() []byte {
	return t.pubKey
}

// Equals implements crypto.PubKey.
func (t *tmPubKeyHack) Equals(pk crypto.PubKey) bool {
	return bytes.Equal(t.pubKey, pk.Bytes())
}

// Type implements crypto.PubKey.
func (t *tmPubKeyHack) Type() string {
	return "ed25519"
}

// VerifySignature implements crypto.PubKey.
func (t *tmPubKeyHack) VerifySignature(msg []byte, sig []byte) bool {
	panic("unimplemented")
}
