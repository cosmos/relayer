package relayer

import (
	"sync"

	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint/types"
)

// SyncHeaders is an instance of map[string]*tmclient.Header
// that is "reasonably" up to date
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
	return uh.hds[chainID].GetHeight()
}

// UpdatingHeaders calls UpdateLiteWithHeader on the passed chains concurrently
func UpdatingHeaders(src, dst *Chain) (*SyncHeaders, error) {
	mp, err := UpdatesWithHeaders(src, dst)
	if err != nil {
		return nil, err
	}
	return &SyncHeaders{hds: mp}, nil
}
