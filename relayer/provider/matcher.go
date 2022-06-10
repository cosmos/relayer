package provider

import (
	"context"
	"fmt"
	"reflect"
	"time"

	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	tmclient "github.com/cosmos/ibc-go/v3/modules/light-clients/07-tendermint/types"
)

// ClientsMatch
func ClientsMatch(ctx context.Context, src, dst ChainProvider, existingClient clienttypes.IdentifiedClientState, newClient ibcexported.ClientState) (string, bool, error) {
	existingClientState, err := clienttypes.UnpackClientState(existingClient.ClientState)
	if err != nil {
		return "", false, err
	}

	if existingClientState.ClientType() != newClient.ClientType() {
		return "", false, nil
	}

	switch ec := existingClientState.(type) {
	case *tmclient.ClientState:
		nc := newClient.(*tmclient.ClientState)
		return TendermintMatcher(ctx, src, dst, existingClient.ClientId, ec, nc)
	}

	return "", false, nil
}

// TendermintMatcher
func TendermintMatcher(ctx context.Context, src, dst ChainProvider, existingClientID string, existingClient, newClient ibcexported.ClientState) (string, bool, error) {
	newClientState, ok := newClient.(*tmclient.ClientState)
	if !ok {
		return "", false, fmt.Errorf("failed type assertion got type(%s) expected type(*tmclient.ClientState)", reflect.TypeOf(newClient))
	}

	existingClientState, ok := existingClient.(*tmclient.ClientState)
	if !ok {
		return "", false, fmt.Errorf("failed type assertion got type(%s) expected type(*tmclient.ClientState)", reflect.TypeOf(existingClient))
	}

	// check if the client states match
	// NOTE: FrozenHeight.IsZero() is a sanity check, the client to be created should always
	// have a zero frozen height and therefore should never match with a frozen client
	if isMatchingTendermintClient(*newClientState, *existingClientState) && existingClientState.FrozenHeight.IsZero() {

		// query the latest consensus state of the potential matching client
		srch, err := src.QueryLatestHeight(ctx)
		if err != nil {
			return "", false, err
		}

		consensusStateResp, err := src.QueryClientConsensusState(ctx, srch, existingClientID, existingClientState.GetLatestHeight())
		if err != nil {
			return "", false, err
		}

		exportedConsState, err := clienttypes.UnpackConsensusState(consensusStateResp.ConsensusState)
		if err != nil {
			return "", false, err
		}

		existingConsensusState, ok := exportedConsState.(*tmclient.ConsensusState)
		if !ok {
			return "", false, fmt.Errorf("failed type assertion got type(%s) expected type(*tmclient.ConsensusState)", reflect.TypeOf(exportedConsState))
		}

		if existingClientState.IsExpired(existingConsensusState.Timestamp, time.Now()) {
			return "", false, tmclient.ErrTrustingPeriodExpired
		}

		header, err := dst.GetLightSignedHeaderAtHeight(ctx, int64(existingClientState.GetLatestHeight().GetRevisionHeight()))
		if err != nil {
			return "", false, err
		}

		tmHeader, ok := header.(*tmclient.Header)
		if !ok {
			return "", false, fmt.Errorf("failed type assertion got type(%s) expected type(*tmclient.Header)", reflect.TypeOf(header))
		}

		if isMatchingTendermintConsensusState(existingConsensusState, tmHeader.ConsensusState()) {
			// found matching client
			return existingClientID, true, nil
		}
	}

	return "", false, nil
}

// isMatchingTendermintClient determines if the two provided clients match in all fields
// except latest height. They are assumed to be IBC tendermint light clients.
// NOTE: we don't pass in a pointer so upstream references don't have a modified
// latest height set to zero.
func isMatchingTendermintClient(clientStateA, clientStateB tmclient.ClientState) bool {
	// zero out latest client height since this is determined and incremented
	// by on-chain updates. Changing the latest height does not fundamentally
	// change the client. The associated consensus state at the latest height
	// determines this last check
	clientStateA.LatestHeight = clienttypes.ZeroHeight()
	clientStateB.LatestHeight = clienttypes.ZeroHeight()

	return reflect.DeepEqual(clientStateA, clientStateB)
}

// isMatchingTendermintConsensusState determines if the two provided consensus states are
// identical. They are assumed to be IBC tendermint light clients.
func isMatchingTendermintConsensusState(consensusStateA, consensusStateB *tmclient.ConsensusState) bool {
	return reflect.DeepEqual(*consensusStateA, *consensusStateB)
}
