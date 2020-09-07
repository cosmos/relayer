package relayer

import (
	"fmt"
	"sync"

	clientTypes "github.com/cosmos/cosmos-sdk/x/ibc/02-client/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint/types"
	clientExported "github.com/cosmos/cosmos-sdk/x/ibc/exported"
)

// NewSyncHeaders returns a new instance of map[string]*tmclient.Header that can be easily
// kept "reasonably up to date"
func NewSyncHeaders(chains ...*Chain) (*SyncHeaders, error) {
	mp, err := UpdatesWithHeaders(chains...)
	if err != nil {
		return nil, err
	}
	return &SyncHeaders{hds: mp}, nil
}

// SyncHeaders is an instance of map[string]*tmclient.Header
// that can be kept "reasonably up to date" using it's Update method
type SyncHeaders struct {
	sync.Mutex

	hds map[string]*tmclient.Header
}

// Update the header for a given chain
func (uh *SyncHeaders) Update(c *Chain) error {
	hd, err := c.UpdateLiteWithHeader()
	if err != nil {
		return err
	}
	uh.Lock()
	defer uh.Unlock()
	uh.hds[c.ChainID] = hd
	return nil
}

// GetHeader returns the latest header for a given chainID
func (uh *SyncHeaders) GetHeader(chainID string) *tmclient.Header {
	uh.Lock()
	defer uh.Unlock()
	return uh.hds[chainID]
}

// GetHeight returns the latest height for a given chainID
func (uh *SyncHeaders) GetHeight(chainID string) uint64 {
	uh.Lock()
	defer uh.Unlock()
	return MustGetHeight(uh.hds[chainID].GetHeight())
}

// InjectTrustedFields injects the necessary trusted fields for a srcHeader coming from a srcChain
// destined for an IBC client stored on the dstChain
// TrustedHeight is the latest height of the IBC client on dstChain
// TrustedValidators is the validator set of srcChain at the TrustedHeight
// InjectTrustedFields returns a copy of the header with TrustedFields modified
func InjectTrustedFields(srcChain, dstChain *Chain, srcHeader *tmclient.Header) (*tmclient.Header, error) {
	// make copy of header stored in mop
	h := *(srcHeader)
	// check that dstChain PathEnd set correctly
	if dstChain.PathEnd.ChainID != h.Header.ChainID {
		return nil, fmt.Errorf("counterparty chain has incorrect PathEnd. expected chainID: %s, got: %s", dstChain.PathEnd.ChainID, h.Header.ChainID)
	}
	// retrieve counterparty client from dst chain
	counterpartyClientRes, err := dstChain.QueryClientState()
	if err != nil {
		return nil, err
	}
	cs, err := clientTypes.UnpackClientState(counterpartyClientRes.ClientState)
	if err != nil {
		panic(err)
	}
	// inject TrustedHeight as latest height stored on counterparty client
	h.TrustedHeight = cs.GetLatestHeight().(clientTypes.Height)
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
func MustGetHeight(h clientExported.Height) uint64 {
	height, ok := h.(clientTypes.Height)
	if !ok {
		panic("height is not an instance of height! wtf")
	}
	return height.EpochHeight
}
