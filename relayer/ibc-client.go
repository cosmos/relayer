package relayer

import (
	"context"
	"fmt"

	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	tmclient "github.com/cosmos/ibc-go/v3/modules/light-clients/07-tendermint/types"
	tmtypes "github.com/tendermint/tendermint/types"
)

func (c *Chain) GetLightSignedHeaderAtHeight(h int64) (*tmclient.Header, error) {
	lightBlock, err := c.Provider.LightBlock(context.Background(), h)
	if err != nil {
		return nil, err
	}
	protoVal, err := tmtypes.NewValidatorSet(lightBlock.ValidatorSet.Validators).ToProto()
	if err != nil {
		return nil, err
	}

	return &tmclient.Header{
		SignedHeader: lightBlock.SignedHeader.ToProto(),
		ValidatorSet: protoVal,
	}, nil
}

// GetIBCUpdateHeader updates the off chain tendermint light client and
// returns an IBC Update Header which can be used to update an on chain
// light client on the destination chain. The source is used to construct
// the header data.
func (c *Chain) GetIBCUpdateHeader(dst *Chain, srch int64) (*tmclient.Header, error) {
	// Construct header data from light client representing source.
	h, err := c.GetLightSignedHeaderAtHeight(srch)
	if err != nil {
		return nil, err
	}

	// Inject trusted fields based on previous header data from source
	return c.InjectTrustedFields(dst, h)
}

// InjectTrustedFields injects the necessary trusted fields for a header to update a light
// client stored on the destination chain, using the information provided by the source
// chain.
// TrustedHeight is the latest height of the IBC client on dst
// TrustedValidators is the validator set of srcChain at the TrustedHeight
// InjectTrustedFields returns a copy of the header with TrustedFields modified
func (c *Chain) InjectTrustedFields(dst *Chain, header *tmclient.Header) (*tmclient.Header, error) {
	// make copy of header stored in mop
	h := *(header)

	// retrieve counterparty client from src chain
	// this is the client that will updated
	cs, err := dst.QueryClientState(0)
	if err != nil {
		return nil, err
	}

	// inject TrustedHeight as latest height stored on counterparty client
	h.TrustedHeight = cs.GetLatestHeight().(clienttypes.Height)

	// NOTE: We need to get validators from the source chain at height: trustedHeight+1
	// since the last trusted validators for a header at height h is the NextValidators
	// at h+1 committed to in header h by NextValidatorsHash

	// TODO: this is likely a source of off by 1 errors but may be impossible to change? Maybe this is the
	// place where we need to fix the upstream query proof issue?
	trustedHeader, err := c.GetLightSignedHeaderAtHeight(int64(h.TrustedHeight.RevisionHeight) + 1)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get trusted header, please ensure header at the height %d has not been pruned by the connected node: %w",
			h.TrustedHeight.RevisionHeight, err,
		)
	}

	// inject TrustedValidators into header
	h.TrustedValidators = trustedHeader.ValidatorSet
	return &h, nil
}

// MustGetHeight takes the height inteface and returns the actual height
func MustGetHeight(h ibcexported.Height) clienttypes.Height {
	height, ok := h.(clienttypes.Height)
	if !ok {
		panic("height is not an instance of height!")
	}
	return height
}
