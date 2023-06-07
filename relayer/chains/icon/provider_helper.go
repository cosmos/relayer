package icon

import (
	"fmt"
	"strings"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	"github.com/icon-project/IBC-Integration/libraries/go/common/icon"
	itm "github.com/icon-project/IBC-Integration/libraries/go/common/tendermint"
)

// Implement when a new chain is added to ICON IBC Contract
func (icp *IconProvider) ClientToAny(clientId string, clientStateB []byte) (*codectypes.Any, error) {
	if strings.Contains(clientId, "icon") {
		var clientState icon.ClientState
		err := icp.codec.Marshaler.Unmarshal(clientStateB, &clientState)
		if err != nil {
			return nil, err
		}
		return clienttypes.PackClientState(&clientState)
	}
	if strings.Contains(clientId, "tendermint") {
		var clientState itm.ClientState
		err := icp.codec.Marshaler.Unmarshal(clientStateB, &clientState)
		if err != nil {
			return nil, err
		}

		return clienttypes.PackClientState(&clientState)
	}
	return nil, fmt.Errorf("unknown client type")
}

func (icp *IconProvider) ConsensusToAny(clientId string, cb []byte) (*codectypes.Any, error) {
	if strings.Contains(clientId, "icon") {
		var consensusState icon.ConsensusState
		err := icp.codec.Marshaler.Unmarshal(cb, &consensusState)
		if err != nil {
			return nil, err
		}
		return clienttypes.PackConsensusState(&consensusState)
	}
	if strings.Contains(clientId, "tendermint") {
		var consensusState itm.ConsensusState
		err := icp.codec.Marshaler.Unmarshal(cb, &consensusState)
		if err != nil {
			return nil, err
		}

		return clienttypes.PackConsensusState(&consensusState)
	}
	return nil, fmt.Errorf("unknown consensus type")
}

func (icp *IconProvider) MustReturnIconClientState(cs ibcexported.ClientState) (*icon.ClientState, error) {
	if !strings.Contains(cs.ClientType(), "icon") {
		return nil, fmt.Errorf("Is not icon client state")
	}

	iconClient, ok := cs.(*icon.ClientState)
	if !ok {
		return nil, fmt.Errorf("Unable to return client state")
	}
	return iconClient, nil
}
