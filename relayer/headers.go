package relayer

import (
	"sync"

	clienttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/types"
	ibcexported "github.com/cosmos/cosmos-sdk/x/ibc/core/exported"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
	"golang.org/x/sync/errgroup"
)

// NewSyncHeaders returns a new instance of map[string]*tmclient.Header that can be easily
// kept "reasonably up to date"
func NewSyncHeaders(src, dst *Chain) (*SyncHeaders, error) {
	srch, dsth, err := UpdatesWithHeaders(src, dst)
	if err != nil {
		return nil, err
	}
	return &SyncHeaders{hds: map[string]*tmclient.Header{src.ChainID: srch, dst.ChainID: dsth}}, nil
}

// SyncHeaders is an instance of map[string]*tmclient.Header
// that can be kept "reasonably up to date" using it's Update method
type SyncHeaders struct {
	sync.Mutex

	hds map[string]*tmclient.Header
}

// Update the header for a given chain
func (uh *SyncHeaders) Update(c *Chain) error {
	hd, err := c.UpdateLightWithHeader()
	if err != nil {
		return err
	}
	uh.Lock()
	defer uh.Unlock()
	uh.hds[c.ChainID] = hd
	return nil
}

// Updates updates the headers for a given set of chains
func (uh *SyncHeaders) Updates(c ...*Chain) error {
	eg := new(errgroup.Group)
	for _, chain := range c {
		chain := chain
		eg.Go(func() error {
			return uh.Update(chain)
		})
	}
	return eg.Wait()
}

// GetHeader returns the latest header for a given chainID
func (uh *SyncHeaders) GetHeader(chainID string) *tmclient.Header {
	uh.Lock()
	defer uh.Unlock()
	return uh.hds[chainID]
}

// GetUpdateHeader returns a header to be used to UpdateClient of dstChain stored on srcChain
func (uh *SyncHeaders) GetUpdateHeader(srcChain, dstChain *Chain) (*tmclient.Header, error) {
	h := uh.GetHeader(srcChain.ChainID)

	return InjectTrustedFields(srcChain, dstChain, h)
}

// GetHeight returns the latest height for a given chainID
func (uh *SyncHeaders) GetHeight(chainID string) uint64 {
	uh.Lock()
	defer uh.Unlock()
	return MustGetHeight(uh.hds[chainID].GetHeight())
}

// UpdateWithTrustedHeaders updates the latest headers in SyncHeaders
func (uh *SyncHeaders) UpdateWithTrustedHeaders(src, dst *Chain) (srcTh, dstTh *tmclient.Header, err error) {
	eg := new(errgroup.Group)
	eg.Go(func() error {
		return uh.Update(src)
	})
	eg.Go(func() error {
		return uh.Update(dst)
	})
	err = eg.Wait()
	return
}

// GetTrustedHeaders returns the trusted headers for the current headers stored in SyncHeaders
func (uh *SyncHeaders) GetTrustedHeaders(src, dst *Chain) (srcTh, dstTh *tmclient.Header, err error) {
	eg := new(errgroup.Group)
	eg.Go(func() error {
		srcTh, err = InjectTrustedFields(src, dst, uh.GetHeader(src.ChainID))
		return err
	})
	eg.Go(func() error {
		dstTh, err = InjectTrustedFields(dst, src, uh.GetHeader(dst.ChainID))
		return err
	})
	err = eg.Wait()
	return
}

// InjectTrustedFields injects the necessary trusted fields for a srcHeader coming from a srcChain
// destined for an IBC client stored on the dstChain
// TrustedHeight is the latest height of the IBC client on dstChain
// TrustedValidators is the validator set of srcChain at the TrustedHeight
// InjectTrustedFields returns a copy of the header with TrustedFields modified
func InjectTrustedFields(srcChain, dstChain *Chain, srcHeader *tmclient.Header) (*tmclient.Header, error) {
	// make copy of header stored in mop
	h := *(srcHeader)

	dsth, err := dstChain.GetLatestLightHeight()
	if err != nil {
		return nil, err
	}

	// retrieve counterparty client from dst chain
	counterpartyClientRes, err := dstChain.QueryClientState(dsth)
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
	valSet, err := srcChain.QueryValsetAtHeight(h.TrustedHeight)
	if err != nil {
		return nil, err
	}

	// inject TrustedValidators into header
	h.TrustedValidators = valSet
	return &h, nil
}

// MustGetHeight takes the height inteface and returns the actual height
func MustGetHeight(h ibcexported.Height) uint64 {
	height, ok := h.(clienttypes.Height)
	if !ok {
		panic("height is not an instance of height! wtf")
	}
	return height.GetRevisionHeight()
}
