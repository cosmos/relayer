package relayer

import (
	clienttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
)

// ConstructIBCTMClientHeader returns a header to be used to update the on-chain light
// client (on destination) to the latest consensus state for the source chain.
func ConstructIBCTMHeader(sh *SyncHeaders, src, dst *Chain) (*tmclient.Header, error) {
	h := sh.GetHeader(src.ChainID)

	return InjectTrustedFields(src, dst, h)
}

// InjectTrustedFields injects the necessary trusted fields for a header to update a light
// client stored on the destination chain, using the information provided by the source
// chain.
// TrustedHeight is the latest height of the IBC client on dst
// TrustedValidators is the validator set of srcChain at the TrustedHeight
// InjectTrustedFields returns a copy of the header with TrustedFields modified
func InjectTrustedFields(src, dst *Chain, header *tmclient.Header) (*tmclient.Header, error) {
	// make copy of header stored in mop
	h := *(header)

	dstH, err := dst.GetLatestLightHeight()
	if err != nil {
		return nil, err
	}

	// retrieve counterparty client from dst chain
	counterpartyClientRes, err := dst.QueryClientState(dstH)
	if err != nil {
		return nil, err
	}
	cs, err := clienttypes.UnpackClientState(counterpartyClientRes.ClientState)
	if err != nil {
		panic(err)
	}

	// inject TrustedHeight as latest height stored on counterparty client
	h.TrustedHeight = cs.GetLatestHeight().(clienttypes.Height)

	// query TrustedValidators at Trusted Height from srcChain
	trustedHeader, err := src.GetLightSignedHeaderAtHeight(int64(h.TrustedHeight.RevisionHeight))
	if err != nil {
		return nil, err
	}

	// inject TrustedValidators into header
	h.TrustedValidators = trustedHeader.ValidatorSet
	return &h, nil
}
