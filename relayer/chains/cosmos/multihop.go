package cosmos

import (
	"context"
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

func (e endpoint) GetKeyValueProofHeight() exported.Height {
	//TODO implement me
	panic("implement me")
}

func (e endpoint) GetConsensusHeight() exported.Height {
	//TODO implement me
	panic("implement me")
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
	//TODO implement me
	panic("implement me")
}

func (e endpoint) GetMerklePath(path string) (commitmenttypes.MerklePath, error) {
	//TODO implement me
	panic("implement me")
}

func (e endpoint) UpdateClient() error {
	//TODO implement me
	panic("implement me")
}

func (e endpoint) Counterparty() multihop.Endpoint {
	//TODO implement me
	panic("implement me")
}

func newEndpoint(provider *CosmosProvider, clientID, connectionID string) multihop.Endpoint {
	return &endpoint{
		provider,
		clientID,
		connectionID,
	}
}
