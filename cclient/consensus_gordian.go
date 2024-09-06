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
	tmtypes "github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/types"
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

//  -----

// DoBroadcastTxAsync implements ConsensusClient.
func (g *GordianConsensus) DoBroadcastTxAsync(ctx context.Context, tx tmtypes.Tx) (*TxResultResponse, error) {
	// TODO: fix me to be async
	// panic("unimplemented")
	return g.DoBroadcastTxSync(ctx, tx)
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

// SimulateTransaction implements ConsensusClient.
func (g *GordianConsensus) SimulateTransaction(ctx context.Context, tx []byte, cfg *SimTxConfig) (types.GasInfo, error) {
	var body io.Reader
	if tx != nil {
		body = bytes.NewReader(tx)
	} else {
		return types.GasInfo{}, fmt.Errorf("SimulateTransaction tx is nil")
	}

	res, err := http.Post(fmt.Sprintf("%s/debug/simulate_tx", g.addr), "application/json", body)
	if err != nil {
		fmt.Printf("error making http request: %s\n", err)
		os.Exit(1)
	}

	var resp TxResultResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		fmt.Printf("error decoding response: %s\n", err)
		return types.GasInfo{}, err
	}

	return types.GasInfo{
		GasWanted: resp.GasWanted,
		GasUsed:   resp.GasUsed,
	}, nil
}

// GetABCIQuery implements ConsensusClient.
func (g *GordianConsensus) GetABCIQuery(ctx context.Context, queryPath string, data cmtbytes.HexBytes) (*ABCIQueryResponse, error) {
	// res, err := cc.QueryABCI(ctx, abci.RequestQuery{
	// Path:   "store/upgrade/key",
	// Height: int64(height - 1),
	// Data:   key,
	// Prove:  true,
	// })
	// if err != nil {
	// return nil, clienttypes.Height{}, err
	// }
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
func (g *GordianConsensus) GetCommit(ctx context.Context, height uint64) (*ResultCommit, error) {
	// looks like we just need the apphash. returning just this for gordian to see how it goes.
	// get latest header from the network

	res, err := http.Get(fmt.Sprintf("%s/commit", g.addr))
	if err != nil {
		fmt.Printf("error making http request: %s\n", err)
		os.Exit(1)
	}

	// tmconsensus.CommittedBlock
	type GetCommitResponse struct {
		BlockHash         []byte `protobuf:"bytes,1,opt,name=block_hash,json=blockHash,proto3" json:"block_hash,omitempty"`
		BlockHashPrevious []byte `protobuf:"bytes,2,opt,name=block_hash_previous,json=blockHashPrevious,proto3" json:"block_hash_previous,omitempty"`
		Height            uint64 `protobuf:"varint,3,opt,name=height,proto3" json:"height,omitempty"`
		// PreviousCommitProof *CommitProof  `protobuf:"bytes,4,opt,name=previous_commit_proof,json=previousCommitProof,proto3" json:"previous_commit_proof,omitempty"`
		// ValidatorSet        *ValidatorSet `protobuf:"bytes,5,opt,name=validator_set,json=validatorSet,proto3" json:"validator_set,omitempty"`
		// ValidatorSetNext    *ValidatorSet `protobuf:"bytes,6,opt,name=validator_set_next,json=validatorSetNext,proto3" json:"validator_set_next,omitempty"`
		DataId           []byte `protobuf:"bytes,7,opt,name=data_id,json=dataId,proto3" json:"data_id,omitempty"`
		AppStatePrevHash []byte `protobuf:"bytes,8,opt,name=app_state_prev_hash,json=appStatePrevHash,proto3" json:"app_state_prev_hash,omitempty"` // annotations
	}

	var resp GetCommitResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		fmt.Printf("error decoding response: %s\n", err)
		os.Exit(1)
	}

	// Get this from the header annotation directly?
	bt, err := g.GetBlockTime(ctx, resp.Height)
	if err != nil {
		return nil, fmt.Errorf("failed to get block time: %w", err)
	}

	// TODO: do we need the full coretypes.NewResultCommit ? Does not seem like it
	return &ResultCommit{
		AppHash: resp.AppStatePrevHash,
		Time:    bt,
	}, nil
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
	// TODO:
	return nil, nil
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
