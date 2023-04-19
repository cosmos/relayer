package cosmos

import (
	"context"
	"fmt"
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

func (e endpoint) getClientState() exported.ClientState {
	ctx := context.Background()
	height, err := e.provider.QueryLatestHeight(ctx)
	if err != nil {
		panic(err)
	}
	clientState, err := e.provider.QueryClientState(ctx, height, e.clientID)
	if err != nil {
		panic(err)
	}
	return clientState
}

func (e endpoint) GetConsensusHeight() exported.Height {
	return e.getClientState().GetLatestHeight()
}

// TODO: this is redundant, should be removed
func (e endpoint) GetKeyValueProofHeight() exported.Height {
	return e.GetConsensusHeight()
}

func (e endpoint) GetConsensusState(height exported.Height) (exported.ConsensusState, error) {
	clientState := e.getClientState()
	clientHeight := clientState.GetLatestHeight()
	ctx := context.Background()
	chainHeight, err := e.provider.QueryLatestHeight(ctx)
	if err != nil {
		return nil, err
	}
	consensusStateResponse, err := e.provider.QueryClientConsensusState(ctx, chainHeight, e.clientID, clientHeight)
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
	_, proof, proofHeight, err := e.provider.QueryTendermintProof(ctx, height, key)
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
	srch, err := e.provider.QueryLatestHeight(ctx)
	if err != nil {
		return err
	}
	dsth, err := e.counterparty.provider.QueryLatestHeight(ctx)
	if err != nil {
		return err
	}
	srcHeader, err := e.provider.QueryIBCHeader(ctx, srch)
	if err != nil {
		return err
	}
	dstClientState, err := e.counterparty.provider.QueryClientState(ctx, dsth, e.clientID)
	if err != nil {
		return err
	}
	dstTrustedHeader, err := e.provider.QueryIBCHeader(ctx, int64(dstClientState.GetLatestHeight().GetRevisionHeight())+1)
	updateHeader, err := e.provider.MsgUpdateClientHeader(srcHeader, dstClientState.GetLatestHeight().(clienttypes.Height), dstTrustedHeader)
	msg, err := e.provider.MsgUpdateClient(e.counterparty.clientID, updateHeader)
	if err != nil {
		return err
	}
	response, success, err := e.provider.SendMessage(ctx, msg, "")
	if err != nil {
		return err
	}
	if response.Code != 0 {
		return fmt.Errorf("client update failed with code %d", response.Code)
	}
	if !success {
		return fmt.Errorf("client update execution failed")
	}
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
