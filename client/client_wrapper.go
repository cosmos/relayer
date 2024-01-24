package client

import (
	"context"

	"github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto/merkle"
	"github.com/cometbft/cometbft/libs/bytes"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/proto/tendermint/crypto"
	cometprotoversion "github.com/cometbft/cometbft/proto/tendermint/version"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	tmtypes "github.com/cometbft/cometbft/types"
	sltypes "github.com/strangelove-ventures/cometbft-client/abci/types"
	"github.com/strangelove-ventures/cometbft-client/client"
	slbytes "github.com/strangelove-ventures/cometbft-client/libs/bytes"
	slclient "github.com/strangelove-ventures/cometbft-client/rpc/client"
	types2 "github.com/strangelove-ventures/cometbft-client/types"
)

// RPCClient wraps our slimmed down CometBFT client and converts the returned types to the upstream CometBFT types.
// This is useful so that it can be used in any function calls that expect the upstream types.
type RPCClient struct {
	Client *client.Client
}

func (r RPCClient) ABCIInfo(ctx context.Context) (*coretypes.ResultABCIInfo, error) {
	res, err := r.Client.ABCIInfo(ctx)
	if err != nil {
		return nil, err
	}

	return &coretypes.ResultABCIInfo{
		Response: types.ResponseInfo{
			Data:             res.Response.Data,
			Version:          res.Response.Version,
			AppVersion:       res.Response.AppVersion,
			LastBlockHeight:  res.Response.LastBlockHeight,
			LastBlockAppHash: res.Response.LastBlockAppHash,
		},
	}, nil
}

func (r RPCClient) ABCIQuery(
	ctx context.Context,
	path string,
	data bytes.HexBytes,
) (*coretypes.ResultABCIQuery, error) {
	res, err := r.Client.ABCIQuery(ctx, path, slbytes.HexBytes(data))
	if err != nil {
		return nil, err
	}

	return &coretypes.ResultABCIQuery{
		Response: types.ResponseQuery{
			Code:      res.Response.Code,
			Log:       res.Response.Log,
			Info:      res.Response.Info,
			Index:     res.Response.Index,
			Key:       res.Response.Key,
			Value:     res.Response.Value,
			ProofOps:  convertProofOps(res.Response.ProofOps),
			Height:    res.Response.Height,
			Codespace: res.Response.Codespace,
		},
	}, nil
}

func (r RPCClient) ABCIQueryWithOptions(
	ctx context.Context,
	path string,
	data bytes.HexBytes,
	opts rpcclient.ABCIQueryOptions,
) (*coretypes.ResultABCIQuery, error) {
	o := slclient.ABCIQueryOptions{
		Height: opts.Height,
		Prove:  opts.Prove,
	}

	res, err := r.Client.ABCIQueryWithOptions(ctx, path, slbytes.HexBytes(data), o)
	if err != nil {
		return nil, err
	}

	return &coretypes.ResultABCIQuery{
		Response: types.ResponseQuery{
			Code:      res.Response.Code,
			Log:       res.Response.Log,
			Info:      res.Response.Info,
			Index:     res.Response.Index,
			Key:       res.Response.Key,
			Value:     res.Response.Value,
			ProofOps:  convertProofOps(res.Response.ProofOps),
			Height:    res.Response.Height,
			Codespace: res.Response.Codespace,
		},
	}, nil
}

func (r RPCClient) BroadcastTxCommit(ctx context.Context, tx tmtypes.Tx) (*coretypes.ResultBroadcastTxCommit, error) {
	res, err := r.Client.BroadcastTxCommit(ctx, types2.Tx(tx))
	if err != nil {
		return nil, err
	}

	return &coretypes.ResultBroadcastTxCommit{
		CheckTx: types.ResponseCheckTx{
			Code:      res.CheckTx.Code,
			Data:      res.CheckTx.Data,
			Log:       res.CheckTx.Log,
			Info:      res.CheckTx.Info,
			GasWanted: res.CheckTx.GasWanted,
			GasUsed:   res.CheckTx.GasUsed,
			Events:    convertEvents(res.CheckTx.Events),
			Codespace: res.CheckTx.Codespace,
		},
		TxResult: types.ExecTxResult{
			Code:      res.TxResult.Code,
			Data:      res.TxResult.Data,
			Log:       res.TxResult.Log,
			Info:      res.TxResult.Info,
			GasWanted: res.TxResult.GasWanted,
			GasUsed:   res.TxResult.GasUsed,
			Events:    convertEvents(res.TxResult.Events),
			Codespace: res.TxResult.Codespace,
		},
		Hash:   bytes.HexBytes(res.Hash),
		Height: res.Height,
	}, nil
}

func (r RPCClient) BroadcastTxAsync(ctx context.Context, tx tmtypes.Tx) (*coretypes.ResultBroadcastTx, error) {
	res, err := r.Client.BroadcastTxAsync(ctx, types2.Tx(tx))
	if err != nil {
		return nil, err
	}

	return &coretypes.ResultBroadcastTx{
		Code:      res.Code,
		Data:      bytes.HexBytes(res.Data),
		Log:       res.Log,
		Codespace: res.Codespace,
		Hash:      bytes.HexBytes(res.Hash),
	}, nil
}

func (r RPCClient) BroadcastTxSync(ctx context.Context, tx tmtypes.Tx) (*coretypes.ResultBroadcastTx, error) {
	res, err := r.Client.BroadcastTxSync(ctx, types2.Tx(tx))
	if err != nil {
		return nil, err
	}

	return &coretypes.ResultBroadcastTx{
		Code:      res.Code,
		Data:      bytes.HexBytes(res.Data),
		Log:       res.Log,
		Codespace: res.Codespace,
		Hash:      bytes.HexBytes(res.Hash),
	}, nil
}

func (r RPCClient) Validators(
	ctx context.Context,
	height *int64,
	page, perPage *int,
) (*coretypes.ResultValidators, error) {
	res, err := r.Client.Validators(ctx, height, page, perPage)
	if err != nil {
		return nil, err
	}

	vals := make([]*tmtypes.Validator, len(res.Validators))
	for i, val := range res.Validators {
		vals[i] = &tmtypes.Validator{
			Address:          tmtypes.Address(val.Address),
			PubKey:           nil, // TODO: PubKey in our response type is an interface, need to figure out how to handle
			VotingPower:      val.VotingPower,
			ProposerPriority: val.ProposerPriority,
		}
	}

	return &coretypes.ResultValidators{
		BlockHeight: res.BlockHeight,
		Validators:  vals,
		Count:       res.Count,
		Total:       res.Total,
	}, nil
}

func (r RPCClient) Status(ctx context.Context) (*coretypes.ResultStatus, error) {
	res, err := r.Client.Status(ctx)
	if err != nil {
		return nil, err
	}

	return &coretypes.ResultStatus{
		NodeInfo: p2p.DefaultNodeInfo{
			ProtocolVersion: p2p.ProtocolVersion{
				P2P:   res.NodeInfo.ProtocolVersion.P2P,
				Block: res.NodeInfo.ProtocolVersion.Block,
				App:   res.NodeInfo.ProtocolVersion.App,
			},
			DefaultNodeID: p2p.ID(res.NodeInfo.DefaultNodeID),
			ListenAddr:    res.NodeInfo.ListenAddr,
			Network:       res.NodeInfo.Network,
			Version:       res.NodeInfo.Version,
			Channels:      bytes.HexBytes(res.NodeInfo.Channels),
			Moniker:       res.NodeInfo.Moniker,
			Other: p2p.DefaultNodeInfoOther{
				TxIndex:    res.NodeInfo.Other.TxIndex,
				RPCAddress: res.NodeInfo.Other.RPCAddress,
			},
		},
		SyncInfo: coretypes.SyncInfo{
			LatestBlockHash:     bytes.HexBytes(res.SyncInfo.LatestBlockHash),
			LatestAppHash:       bytes.HexBytes(res.SyncInfo.LatestAppHash),
			LatestBlockHeight:   res.SyncInfo.LatestBlockHeight,
			LatestBlockTime:     res.SyncInfo.LatestBlockTime,
			EarliestBlockHash:   bytes.HexBytes(res.SyncInfo.EarliestBlockHash),
			EarliestAppHash:     bytes.HexBytes(res.SyncInfo.EarliestAppHash),
			EarliestBlockHeight: res.SyncInfo.EarliestBlockHeight,
			EarliestBlockTime:   res.SyncInfo.EarliestBlockTime,
			CatchingUp:          res.SyncInfo.CatchingUp,
		},
		ValidatorInfo: coretypes.ValidatorInfo{
			Address:     bytes.HexBytes(res.ValidatorInfo.Address),
			PubKey:      nil, // TODO: PubKey in our response type is an interface, need to figure out how to handle
			VotingPower: res.ValidatorInfo.VotingPower,
		},
	}, nil
}

func (r RPCClient) Block(ctx context.Context, height *int64) (*coretypes.ResultBlock, error) {
	res, err := r.Client.Block(ctx, height)
	if err != nil {
		return nil, err
	}

	return &coretypes.ResultBlock{
		BlockID: convertBlockID(res.BlockID),
		Block:   convertBlock(res.Block),
	}, nil
}

func (r RPCClient) BlockByHash(ctx context.Context, hash []byte) (*coretypes.ResultBlock, error) {
	res, err := r.Client.BlockByHash(ctx, hash)
	if err != nil {
		return nil, err
	}

	return &coretypes.ResultBlock{
		BlockID: convertBlockID(res.BlockID),
		Block:   convertBlock(res.Block),
	}, nil
}

func (r RPCClient) BlockResults(ctx context.Context, height *int64) (*coretypes.ResultBlockResults, error) {
	res, err := r.Client.BlockResults(ctx, height)
	if err != nil {
		return nil, err
	}

	txs := make([]*types.ExecTxResult, len(res.TxResponses))
	for i, tx := range res.TxResponses {
		txs[i] = &types.ExecTxResult{
			Code:      tx.Code,
			Data:      tx.Data,
			Log:       tx.Log,
			Info:      tx.Info,
			GasWanted: tx.GasWanted,
			GasUsed:   tx.GasUsed,
			Events:    nil,
			Codespace: tx.Codespace,
		}
	}

	return &coretypes.ResultBlockResults{
		Height:                res.Height,
		TxsResults:            nil,
		FinalizeBlockEvents:   nil,
		ValidatorUpdates:      nil,
		ConsensusParamUpdates: nil,
		AppHash:               res.AppHash,
	}, nil
}

func (r RPCClient) BlockchainInfo(
	ctx context.Context,
	minHeight, maxHeight int64,
) (*coretypes.ResultBlockchainInfo, error) {
	res, err := r.Client.BlockchainInfo(ctx, minHeight, maxHeight)
	if err != nil {
		return nil, err
	}

	meta := make([]*tmtypes.BlockMeta, len(res.BlockMetas))
	for i, m := range res.BlockMetas {
		meta[i] = &tmtypes.BlockMeta{
			BlockID: tmtypes.BlockID{
				Hash: bytes.HexBytes(m.BlockID.Hash),
				PartSetHeader: tmtypes.PartSetHeader{
					Total: m.BlockID.PartSetHeader.Total,
					Hash:  bytes.HexBytes(m.BlockID.PartSetHeader.Hash),
				},
			},
			BlockSize: m.BlockSize,
			Header:    convertHeader(m.Header),
			NumTxs:    m.NumTxs,
		}
	}

	return &coretypes.ResultBlockchainInfo{
		LastHeight: res.LastHeight,
		BlockMetas: meta,
	}, nil
}

func (r RPCClient) Commit(ctx context.Context, height *int64) (*coretypes.ResultCommit, error) {
	res, err := r.Client.Commit(ctx, height)
	if err != nil {
		return nil, err
	}

	signatures := make([]tmtypes.CommitSig, len(res.Commit.Signatures))
	for i, sig := range res.Commit.Signatures {
		signatures[i] = tmtypes.CommitSig{
			BlockIDFlag:      tmtypes.BlockIDFlag(sig.BlockIDFlag),
			ValidatorAddress: tmtypes.Address(sig.ValidatorAddress),
			Timestamp:        sig.Timestamp,
			Signature:        sig.Signature,
		}
	}

	header := convertHeader(*res.SignedHeader.Header)
	return &coretypes.ResultCommit{
		SignedHeader: tmtypes.SignedHeader{
			Header: &header,
			Commit: &tmtypes.Commit{
				Height:     res.Commit.Height,
				Round:      res.Commit.Round,
				BlockID:    convertBlockID(res.Commit.BlockID),
				Signatures: signatures,
			},
		},
		CanonicalCommit: res.CanonicalCommit,
	}, nil
}

func (r RPCClient) Tx(ctx context.Context, hash []byte, prove bool) (*coretypes.ResultTx, error) {
	res, err := r.Client.Tx(ctx, hash, prove)
	if err != nil {
		return nil, err
	}

	return &coretypes.ResultTx{
		Hash:     bytes.HexBytes(res.Hash),
		Height:   res.Height,
		Index:    res.Index,
		TxResult: types.ExecTxResult{}, // TODO:
		Tx:       tmtypes.Tx(res.Tx),
		Proof: tmtypes.TxProof{
			RootHash: bytes.HexBytes(res.Proof.RootHash),
			Data:     tmtypes.Tx(res.Proof.Data),
			Proof: merkle.Proof{
				Total:    res.Proof.Proof.Total,
				Index:    res.Proof.Proof.Index,
				LeafHash: res.Proof.Proof.LeafHash,
				Aunts:    res.Proof.Proof.Aunts,
			},
		},
	}, nil
}

func (r RPCClient) TxSearch(
	ctx context.Context,
	query string,
	prove bool,
	page, perPage *int,
	orderBy string,
) (*coretypes.ResultTxSearch, error) {
	res, err := r.Client.TxSearch(ctx, query, prove, page, perPage, orderBy)
	if err != nil {
		return nil, err
	}

	_ = res

	return &coretypes.ResultTxSearch{
		Txs:        nil,
		TotalCount: 0,
	}, nil
}

func (r RPCClient) BlockSearch(
	ctx context.Context,
	query string,
	page, perPage *int,
	orderBy string,
) (*coretypes.ResultBlockSearch, error) {
	res, err := r.Client.BlockSearch(ctx, query, page, perPage, orderBy)
	if err != nil {
		return nil, err
	}

	_ = res

	return &coretypes.ResultBlockSearch{
		Blocks:     nil,
		TotalCount: 0,
	}, nil
}

func convertProofOps(proofOps *sltypes.ProofOps) *crypto.ProofOps {
	ops := make([]crypto.ProofOp, len(proofOps.Ops))
	for i, op := range proofOps.Ops {
		ops[i] = crypto.ProofOp{
			Type: op.Type,
			Key:  op.Key,
			Data: op.Data,
		}
	}

	return &crypto.ProofOps{Ops: ops}
}

func convertEvents(events []sltypes.Event) []types.Event {
	evts := make([]types.Event, len(events))

	for i, evt := range events {
		attributes := make([]types.EventAttribute, len(evt.Attributes))

		for j, attr := range evt.Attributes {
			attributes[j] = types.EventAttribute{
				Key:   attr.Key,
				Value: attr.Value,
				Index: attr.Index,
			}
		}

		evts[i] = types.Event{
			Type:       evt.Type,
			Attributes: attributes,
		}
	}

	return evts
}

func convertHeader(header types2.Header) tmtypes.Header {
	return tmtypes.Header{
		// TODO: Version does not appear to be present in our response type
		Version: cometprotoversion.Consensus{
			Block: 0,
			App:   0,
		},
		ChainID: header.ChainID,
		Height:  header.Height,
		Time:    header.Time,
		LastBlockID: tmtypes.BlockID{
			Hash: bytes.HexBytes(header.LastBlockID.Hash),
			PartSetHeader: tmtypes.PartSetHeader{
				Total: header.LastBlockID.PartSetHeader.Total,
				Hash:  bytes.HexBytes(header.LastBlockID.PartSetHeader.Hash),
			},
		},
		LastCommitHash:     bytes.HexBytes(header.LastCommitHash),
		DataHash:           bytes.HexBytes(header.DataHash),
		ValidatorsHash:     bytes.HexBytes(header.ValidatorsHash),
		NextValidatorsHash: bytes.HexBytes(header.NextValidatorsHash),
		ConsensusHash:      bytes.HexBytes(header.ConsensusHash),
		AppHash:            bytes.HexBytes(header.AppHash),
		LastResultsHash:    bytes.HexBytes(header.LastResultsHash),
		EvidenceHash:       bytes.HexBytes(header.EvidenceHash),
		ProposerAddress:    tmtypes.Address(header.ProposerAddress),
	}
}

func convertBlockID(id types2.BlockID) tmtypes.BlockID {
	return tmtypes.BlockID{
		Hash: bytes.HexBytes(id.Hash),
		PartSetHeader: tmtypes.PartSetHeader{
			Total: id.PartSetHeader.Total,
			Hash:  bytes.HexBytes(id.PartSetHeader.Hash),
		},
	}
}

func convertBlock(block *types2.Block) *tmtypes.Block {
	signatures := make([]tmtypes.CommitSig, len(block.LastCommit.Signatures))
	for i, sig := range block.LastCommit.Signatures {
		signatures[i] = tmtypes.CommitSig{
			BlockIDFlag:      tmtypes.BlockIDFlag(sig.BlockIDFlag),
			ValidatorAddress: tmtypes.Address(sig.ValidatorAddress),
			Timestamp:        sig.Timestamp,
			Signature:        sig.Signature,
		}
	}

	txs := make([]tmtypes.Tx, len(block.Data.Txs))
	for i, tx := range block.Data.Txs {
		txs[i] = tmtypes.Tx(tx)
	}

	return &tmtypes.Block{
		Header: convertHeader(block.Header),
		Data: tmtypes.Data{
			Txs: txs,
		},
		Evidence: tmtypes.EvidenceData{}, // TODO: EvidenceData
		LastCommit: &tmtypes.Commit{
			Height:     block.LastCommit.Height,
			Round:      block.LastCommit.Round,
			BlockID:    convertBlockID(block.LastCommit.BlockID),
			Signatures: signatures,
		},
	}
}
