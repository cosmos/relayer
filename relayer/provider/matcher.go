package provider

import (
	"context"
	"fmt"
	"reflect"
	"time"

	sdkcodec "github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	tmclient "github.com/cosmos/ibc-go/v7/modules/light-clients/07-tendermint"
)

var tendermintClientCodec = tmClientCodec()

func tmClientCodec() *sdkcodec.ProtoCodec {
	interfaceRegistry := types.NewInterfaceRegistry()
	tmclient.RegisterInterfaces(interfaceRegistry)
	return sdkcodec.NewProtoCodec(interfaceRegistry)
}

// ClientsMatch will check the type of an existing light client on the src chain, tracking the dst chain, and run
// an appropriate matcher function to determine if the existing client's state matches a proposed new client
// state constructed from the dst chain.
func ClientsMatch(ctx context.Context, src, dst ChainProvider, existingClient clienttypes.IdentifiedClientState, newClient ibcexported.ClientState) (string, error) {
	existingClientState, err := clienttypes.UnpackClientState(existingClient.ClientState)
	if err != nil {
		return "", err
	}

	if existingClientState.ClientType() != newClient.ClientType() {
		return "", nil
	}

	switch ec := existingClientState.(type) {
	case *tmclient.ClientState:
		nc := newClient.(*tmclient.ClientState)
		return cometMatcher(ctx, src, dst, existingClient.ClientId, ec, nc)
	}

	return "", nil
}

// CheckForMisbehaviour checks that a proposed header, for updating a light client, contains a consensus state that matches
// the trusted consensus state from the counterparty for the same block height. If the consensus states for the proposed
// header and the trusted header match then both returned values will be nil.
func CheckForMisbehaviour(
	ctx context.Context,
	counterparty ChainProvider,
	clientID string,
	proposedHeader []byte,
	cachedHeader IBCHeader,
) (ibcexported.ClientMessage, error) {
	var (
		misbehavior ibcexported.ClientMessage
		err         error
	)

	clientMsg, err := clienttypes.UnmarshalClientMessage(tendermintClientCodec, proposedHeader)
	if err != nil {
		return nil, err
	}

	switch header := clientMsg.(type) {
	case *tmclient.Header:
		misbehavior, err = checkTendermintMisbehaviour(ctx, clientID, header, cachedHeader, counterparty)
		if err != nil {
			return nil, err
		}
		if misbehavior == nil && err == nil {
			return nil, nil
		}
	}

	return misbehavior, nil
}

// cometMatcher determines if there is an existing light client on the src chain, tracking the dst chain,
// with a state which matches a proposed new client state constructed from the dst chain.
func cometMatcher(ctx context.Context, src, dst ChainProvider, existingClientID string, existingClient, newClient ibcexported.ClientState) (string, error) {
	newClientState, ok := newClient.(*tmclient.ClientState)
	if !ok {
		return "", fmt.Errorf("got type(%T) expected type(*tmclient.ClientState)", newClient)
	}

	existingClientState, ok := existingClient.(*tmclient.ClientState)
	if !ok {
		return "", fmt.Errorf("got type(%T) expected type(*tmclient.ClientState)", existingClient)
	}

	// Check if the client states match.
	// NOTE: FrozenHeight.IsZero() is a sanity check, the client to be created should always
	// have a zero frozen height and therefore should never match with a frozen client.
	if isMatchingTendermintClient(*newClientState, *existingClientState) && existingClientState.FrozenHeight.IsZero() {
		srch, err := src.QueryLatestHeight(ctx)
		if err != nil {
			return "", err
		}

		// Query the src chain for the latest consensus state of the potential matching client.
		consensusStateResp, err := src.QueryClientConsensusState(ctx, srch, existingClientID, existingClientState.GetLatestHeight())
		if err != nil {
			return "", err
		}

		exportedConsState, err := clienttypes.UnpackConsensusState(consensusStateResp.ConsensusState)
		if err != nil {
			return "", err
		}

		existingConsensusState, ok := exportedConsState.(*tmclient.ConsensusState)
		if !ok {
			return "", fmt.Errorf("got type(%T) expected type(*tmclient.ConsensusState)", exportedConsState)
		}

		// If the existing client state has not been updated within the trusting period,
		// we do not want to use the existing client since it's in an expired state.
		if existingClientState.IsExpired(existingConsensusState.Timestamp, time.Now()) {
			return "", tmclient.ErrTrustingPeriodExpired
		}

		// Construct a header for the consensus state of the counterparty chain.
		ibcHeader, err := dst.QueryIBCHeader(ctx, int64(existingClientState.GetLatestHeight().GetRevisionHeight()))
		if err != nil {
			return "", err
		}

		consensusState, ok := ibcHeader.ConsensusState().(*tmclient.ConsensusState)
		if !ok {
			return "", fmt.Errorf("got type(%T) expected type(*tmclient.ConsensusState)", consensusState)
		}

		// Determine if the existing consensus state on src for the potential matching client is identical
		// to the consensus state of the counterparty chain.
		if isMatchingTendermintConsensusState(existingConsensusState, consensusState) {
			return existingClientID, nil // found matching client
		}
	}

	return "", nil
}

// isMatchingTendermintClient determines if the two provided clients match in all fields
// except latest height. They are assumed to be IBC tendermint light clients.
// NOTE: we don't pass in a pointer so upstream references don't have a modified
// latest height set to zero.
func isMatchingTendermintClient(a, b tmclient.ClientState) bool {
	// zero out latest client height since this is determined and incremented
	// by on-chain updates. Changing the latest height does not fundamentally
	// change the client. The associated consensus state at the latest height
	// determines this last check
	a.LatestHeight = clienttypes.ZeroHeight()
	b.LatestHeight = clienttypes.ZeroHeight()

	return reflect.DeepEqual(a, b)
}

// isMatchingTendermintConsensusState determines if the two provided consensus states are
// identical. They are assumed to be IBC tendermint light clients.
func isMatchingTendermintConsensusState(a, b *tmclient.ConsensusState) bool {
	return reflect.DeepEqual(*a, *b)
}

// checkTendermintMisbehaviour checks that a proposed consensus state, used to update a tendermint light client,
// matches the trusted consensus state from the counterparty chain. If there is no cached trusted header then
// it will be queried from the counterparty. If the consensus states for the proposed header and the trusted header
// match then both returned values will be nil.
func checkTendermintMisbehaviour(
	ctx context.Context,
	clientID string,
	proposedHeader *tmclient.Header,
	cachedHeader IBCHeader,
	counterparty ChainProvider,
) (ibcexported.ClientMessage, error) {
	var (
		trustedHeader *tmclient.Header
		err           error
	)

	if cachedHeader == nil {
		header, err := counterparty.QueryIBCHeader(ctx, proposedHeader.Header.Height)
		if err != nil {
			return nil, err
		}

		tmHeader, ok := header.(TendermintIBCHeader)
		if !ok {
			return nil, fmt.Errorf("failed to check for misbehaviour, expected %T, got %T", (*TendermintIBCHeader)(nil), header)
		}

		trustedHeader, err = tmHeader.TMHeader()
		if err != nil {
			return nil, err
		}
	} else {
		trustedHeader, err = cachedHeader.(TendermintIBCHeader).TMHeader()
		if err != nil {
			return nil, err
		}
	}

	if isMatchingTendermintConsensusState(proposedHeader.ConsensusState(), trustedHeader.ConsensusState()) {
		return nil, nil
	}

	// When we queried the light block in QueryIBCHeader we did not have the TrustedHeight or TrustedValidators,
	// it is the relayer's responsibility to inject these trusted fields i.e. we need a height < the proposed headers height.
	// The TrustedHeight is the height of a stored ConsensusState on the client that will be used to verify the new untrusted header.
	// The Trusted ConsensusState must be within the unbonding period of current time in order to correctly verify,
	// and the TrustedValidators must hash to TrustedConsensusState.NextValidatorsHash since that is the last trusted
	// validator set at the TrustedHeight.
	trustedHeader.TrustedValidators = proposedHeader.TrustedValidators
	trustedHeader.TrustedHeight = proposedHeader.TrustedHeight

	return tmclient.NewMisbehaviour(clientID, proposedHeader, trustedHeader), nil
}
