package cosmos

import (
	"context"
	"fmt"

	"github.com/avast/retry-go/v4"

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

func (e *endpoint) ChainID() string {
	return e.provider.ChainId()
}

func (e *endpoint) Codec() codec.BinaryCodec {
	return e.provider.Cdc.Marshaler
}

func (e *endpoint) ClientID() string {
	return e.clientID
}

func (e *endpoint) GetConsensusHeight() exported.Height {
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
func (e *endpoint) GetKeyValueProofHeight() exported.Height {
	return e.GetConsensusHeight()
}

func (e *endpoint) GetConsensusState(height exported.Height) (exported.ConsensusState, error) {
	ctx := context.Background()
	chainHeight, err := e.provider.QueryLatestHeight(ctx)
	if err != nil {
		return nil, err
	}
	consensusStateResponse, err := e.provider.QueryClientConsensusState(ctx, chainHeight, e.clientID, height)
	if err != nil {
		e.provider.log.Error("Failed to query consensus state",
			zap.String("chain_id", e.ChainID()),
			zap.Int64("chain_height", chainHeight),
			zap.String("client_id", e.ClientID()),
			zap.String("client_height", height.String()),
			zap.Error(err))
		return nil, err
	}
	return clienttypes.UnpackConsensusState(consensusStateResponse.ConsensusState)
}

func (e *endpoint) ConnectionID() string {
	return e.connectionID
}

func (e *endpoint) GetConnection() (*types.ConnectionEnd, error) {
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

func (e *endpoint) QueryProofAtHeight(key []byte, chainHeight int64) ([]byte, clienttypes.Height, error) {
	ctx := context.Background()
	_, proof, proofHeight, err := e.provider.QueryTendermintProof(ctx, chainHeight, key)
	return proof, proofHeight, err
}

func (e *endpoint) GetMerklePath(path string) (commitmenttypes.MerklePath, error) {
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

func (e *endpoint) UpdateClient() error {
	ctx := context.Background()
	srcEndpoint := e.counterparty
	srcProvider := srcEndpoint.provider
	dstEndpoint := e
	dstProvider := e.provider
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
	newConsensusHeight := clienttypes.NewHeight(dstClientState.GetLatestHeight().GetRevisionNumber(), uint64(srch))
	if err := retry.Do(func() error {
		currentHeight := e.GetConsensusHeight()
		if !newConsensusHeight.EQ(e.GetConsensusHeight()) {
			return fmt.Errorf("client %s on %s still at %s", e.clientID, e.ChainID(), currentHeight)
		}
		return nil
	}, retry.Context(ctx), rtyAtt, rtyDel, rtyErr); err != nil {
		return fmt.Errorf("client %s on %s not updated to %s", e.clientID, e.ChainID(), newConsensusHeight)
	}
	return nil
}

func (e *endpoint) Counterparty() multihop.Endpoint {
	return e.counterparty
}

func (cc *CosmosProvider) newEndpoint(clientID, connectionID string) multihop.Endpoint {
	return &endpoint{
		provider:     cc,
		clientID:     clientID,
		connectionID: connectionID,
	}
}
