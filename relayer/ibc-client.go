package relayer

import (
	"fmt"

	clienttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/types"
	ibcexported "github.com/cosmos/cosmos-sdk/x/ibc/core/exported"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
	tmtypes "github.com/tendermint/tendermint/types"
	"golang.org/x/sync/errgroup"
)

// GetIBCCreationHeader updates the off chain tendermint light client and
// returns an IBC Update Header which can be used to create an on-chain
// light client.
func (c *Chain) GetIBCCreationHeader(dst *Chain) (*tmclient.Header, error) {
	lightBlock, err := c.UpdateLightClient()
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

// GetIBCCreationHeaders returns the IBC TM header which will create an on-chain
// light client. The headers do not have trusted headers (ie trusted validators
// and trusted height)
func GetIBCCreationHeaders(src, dst *Chain) (srcHeader, dstHeader *tmclient.Header, err error) {
	var eg = new(errgroup.Group)
	eg.Go(func() error {
		srcHeader, err = src.GetIBCCreationHeader(dst)
		return err
	})
	eg.Go(func() error {
		dstHeader, err = dst.GetIBCCreationHeader(src)
		return err
	})
	if err = eg.Wait(); err != nil {
		return
	}
	return

}

// GetIBCUpdateHeader updates the off chain tendermint light client and
// returns an IBC Update Header which can be used to update an on chain
// light client.
func (c *Chain) GetIBCUpdateHeader(dst *Chain) (*tmclient.Header, error) {
	h, err := c.GetIBCCreationHeader(dst)
	if err != nil {
		return nil, err
	}

	return c.InjectTrustedFields(dst, h)
}

// GetIBCUpdateHeaders return the IBC TM Header which will update an on-chain
// light client. A header for the source and destination chain is returned.
func GetIBCUpdateHeaders(src, dst *Chain) (srcHeader, dstHeader *tmclient.Header, err error) {
	var eg = new(errgroup.Group)
	eg.Go(func() error {
		srcHeader, err = src.GetIBCUpdateHeader(dst)
		return err
	})
	eg.Go(func() error {
		dstHeader, err = dst.GetIBCUpdateHeader(src)
		return err
	})
	if err = eg.Wait(); err != nil {
		return
	}
	return

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
	trustedHeader, err := c.GetLightSignedHeaderAtHeight(int64(h.TrustedHeight.RevisionHeight))
	if err != nil {
		return nil, fmt.Errorf("failed to get trusted header, please ensure the connected node is pruning nothing: %w", err)
	}

	// inject TrustedValidators into header
	h.TrustedValidators = trustedHeader.ValidatorSet
	return &h, nil
}

// InjectTrustedFieldsHeaders takes the headers and enriches them
func InjectTrustedFieldsHeaders(
	src, dst *Chain,
	srch, dsth *tmclient.Header) (srcho *tmclient.Header, dstho *tmclient.Header, err error) {
	var eg = new(errgroup.Group)
	eg.Go(func() error {
		srcho, err = src.InjectTrustedFields(dst, srch)
		return err
	})
	eg.Go(func() error {
		dstho, err = dst.InjectTrustedFields(src, dsth)
		return err
	})
	if err = eg.Wait(); err != nil {
		return
	}
	return
}

// MustGetHeight takes the height inteface and returns the actual height
func MustGetHeight(h ibcexported.Height) uint64 {
	height, ok := h.(clienttypes.Height)
	if !ok {
		panic("height is not an instance of height! wtf")
	}
	return height.GetRevisionHeight()
}
