package cosmos

import (
	"context"
	"fmt"

	tmtypes "github.com/cosmos/ibc-go/v7/modules/light-clients/07-tendermint"

	"go.uber.org/zap"

	"github.com/cosmos/cosmos-sdk/codec"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	"github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	commitmenttypes "github.com/cosmos/ibc-go/v7/modules/core/23-commitment/types"
	"github.com/cosmos/ibc-go/v7/modules/core/exported"
	"github.com/cosmos/ibc-go/v7/modules/core/multihop"
)

var _ multihop.Endpoint = (*endpoint)(nil)

type endpoint struct {
	provider     *CosmosProvider
	clientID     string
	connectionID string
	counterparty *endpoint
}

func (e endpoint) ChainID() string {
	return e.provider.ChainId()
}

func (e endpoint) Codec() codec.BinaryCodec {
	return e.provider.Cdc.Marshaler
}

func (e endpoint) ClientID() string {
	return e.clientID
}

func (e endpoint) GetConsensusHeight() exported.Height {
	ctx := context.Background()
	height, err := e.provider.QueryLatestHeight(ctx)
	if err != nil {
		panic(err)
	}
	clientState, err := e.provider.QueryClientState(ctx, int64(height), e.clientID)
	if err != nil {
		panic(err)
	}
	return clientState.GetLatestHeight()
}

// TODO: this is redundant, should be removed
func (e endpoint) GetKeyValueProofHeight() exported.Height {
	height := e.GetConsensusHeight()
	e.provider.log.Info(
		"Getting key and value proof height for multihop proof",
		zap.String("chain_id", e.ChainID()),
		zap.String("client_id", e.ClientID()),
		zap.String("height", height.String()),
	)
	return height
}

func (e endpoint) GetConsensusState(height exported.Height) (exported.ConsensusState, error) {
	ctx := context.Background()
	chainHeight, err := e.provider.QueryLatestHeight(ctx)
	if err != nil {
		return nil, err
	}
	consensusStateResponse, err := e.provider.QueryClientConsensusState(ctx, chainHeight, e.clientID, height)
	if err != nil {
		return nil, err
	}
	return clienttypes.UnpackConsensusState(consensusStateResponse.ConsensusState)
}

func (e endpoint) ConnectionID() string {
	return e.connectionID
}

func (e endpoint) GetConnection() (*types.ConnectionEnd, error) {
	ctx := context.Background()
	height, err := e.provider.QueryLatestHeight(ctx)
	if err != nil {
		return nil, err
	}
	connectionResponse, err := e.provider.QueryConnection(ctx, height, e.connectionID)
	if err != nil {
		return nil, err
	}
	return connectionResponse.Connection, nil
}

func (e endpoint) QueryProofAtHeight(key []byte, height int64) ([]byte, clienttypes.Height, error) {
	ctx := context.Background()
	chainHeight, err := e.provider.QueryLatestHeight(ctx)
	if err != nil {
		return nil, clienttypes.Height{}, err
	}
	_, proof, proofHeight, err := e.provider.QueryTendermintProof(ctx, chainHeight, key)
	e.provider.log.Info(
		"Querying for multihop proof",
		zap.String("chain_id", e.ChainID()),
		zap.String("client_id", e.ClientID()),
		zap.String("key", string(key)),
		zap.Int64("height", chainHeight),
		zap.Int64("client_height", height),
		zap.String("proof_height", proofHeight.String()),
	)

	// TODO: disable this for production
	var merkleProof commitmenttypes.MerkleProof
	if err := e.Codec().Unmarshal(proof, &merkleProof); err != nil {
		return nil, clienttypes.Height{}, err
	}
	header, err := e.provider.QueryIBCHeader(ctx, chainHeight)
	if err != nil {
		return nil, clienttypes.Height{}, err
	}
	var consensusState *tmtypes.ConsensusState
	consensusState = header.ConsensusState().(*tmtypes.ConsensusState)

	path, err := e.GetMerklePath(string(key))
	if err != nil {
		return nil, clienttypes.Height{}, err
	}
	var value []byte
	if e.ChainID() == "wasm-1" {
		channelResponse, err := e.provider.QueryChannel(ctx, chainHeight, "channel-0", "transfer")
		if err != nil {
			return nil, clienttypes.Height{}, err
		}
		e.provider.log.Info(
			"Queried channel for verification",
			zap.String("chain_id", e.ChainID()),
			zap.String("client_id", e.ClientID()),
			zap.String("key", string(key)),
			zap.Int64("height", height),
			zap.String("proof_height", proofHeight.String()),
			zap.String("channel", fmt.Sprintf("%#v", channelResponse.Channel)),
		)
		value, err = channelResponse.Channel.Marshal()
		if err != nil {
			return nil, clienttypes.Height{}, err
		}
	} else { // osmosis-1
		e.provider.log.Info(
			"Querying consensus state for verification",
			zap.String("chain_id", e.ChainID()),
			zap.String("client_id", e.ClientID()),
			zap.String("key", string(key)),
			zap.Int64("height", height),
			zap.String("proof_height", proofHeight.String()),
		)
		consensusState, err := e.GetConsensusState(clienttypes.NewHeight(1, uint64(height)))
		if err != nil {
			return nil, clienttypes.Height{}, err
		}
		value, err = consensusState.(*tmtypes.ConsensusState).Marshal()
		if err != nil {
			return nil, clienttypes.Height{}, err
		}
	}
	err = merkleProof.VerifyMembership(
		commitmenttypes.GetSDKSpecs(),
		consensusState.GetRoot(),
		path,
		value,
	)
	if err != nil {
		e.provider.log.Error(
			"Querying for multihop proof failed",
			zap.String("chain_id", e.ChainID()),
			zap.String("client_id", e.ClientID()),
			zap.String("key", string(key)),
			zap.Int64("height", chainHeight),
			zap.Int64("client_height", height),
			zap.String("proof_height", proofHeight.String()),
			zap.Error(err),
		)
	}
	return proof, proofHeight, err
}

func (e endpoint) GetMerklePath(path string) (commitmenttypes.MerklePath, error) {
	ctx := context.Background()
	height, err := e.provider.QueryLatestHeight(ctx)
	if err != nil {
		return commitmenttypes.MerklePath{}, err
	}
	connection, err := e.provider.QueryConnection(ctx, height, e.connectionID)
	if err != nil {
		return commitmenttypes.MerklePath{}, err
	}
	prefix := connection.Connection.Counterparty.GetPrefix()
	return commitmenttypes.ApplyPrefix(prefix, commitmenttypes.NewMerklePath(path))
}

func (e endpoint) UpdateClient() error {
	ctx := context.Background()
	srcEndpoint := e.counterparty
	srcProvider := srcEndpoint.provider
	dstProvider := e.provider
	dstEndpoint := e
	srch, err := srcProvider.QueryLatestHeight(ctx)
	if err != nil {
		return err
	}
	srcHeader, err := srcProvider.QueryIBCHeader(ctx, srch)
	if err != nil {
		return err
	}
	dsth, err := dstProvider.QueryLatestHeight(ctx)
	if err != nil {
		return err
	}
	dstClientState, err := dstProvider.QueryClientState(ctx, dsth, dstEndpoint.clientID)
	if err != nil {
		return err
	}
	dstTrustedHeader, err := srcProvider.QueryIBCHeader(ctx,
		int64(dstClientState.GetLatestHeight().GetRevisionHeight())+1)
	if err != nil {
		return err
	}
	updateHeader, err := srcProvider.MsgUpdateClientHeader(srcHeader,
		dstClientState.GetLatestHeight().(clienttypes.Height), dstTrustedHeader)
	if err != nil {
		return err
	}
	msg, err := dstProvider.MsgUpdateClient(dstEndpoint.clientID, updateHeader)
	if err != nil {
		return err
	}
	response, success, err := dstProvider.SendMessage(ctx, msg, "")
	if err != nil {
		return err
	}
	if response.Code != 0 {
		return fmt.Errorf("client update failed with code %d", response.Code)
	}
	if !success {
		return fmt.Errorf("client update execution failed")
	}
	chainHeight, err := e.provider.QueryLatestHeight(ctx)
	if err != nil {
		return err
	}
	e.provider.log.Info(
		"Client updated for multihop proof",
		zap.String("chain_id", e.ChainID()),
		zap.String("client_id", e.ClientID()),
		zap.Int64("height", chainHeight),
		zap.String("client_height", e.GetConsensusHeight().String()),
	)
	return nil
}

func (e endpoint) Counterparty() multihop.Endpoint {
	return e.counterparty
}

func (cc *CosmosProvider) newEndpoint(clientID, connectionID string) multihop.Endpoint {
	return &endpoint{
		provider:     cc,
		clientID:     clientID,
		connectionID: connectionID,
	}
}
