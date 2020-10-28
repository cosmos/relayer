package relayer

import "sync"

func init() {
	SDKConfig = &sdkCfg{}
}

// SDKConfig is a global to gate access to the SDK config object
var SDKConfig *sdkCfg

// sdkCfg helps manage access to the SDK config object
type sdkCfg struct {
	sync.Mutex
}

// SetContext sets the chains sdk context and prevents concurrent operations
func (c *sdkCfg) Set(chain *Chain) {
	c.Lock()
	defer c.Unlock()
	chain.UseSDKContext()
}

// SetLock returns the unlock function from the mutex so that the context can be held
func (c *sdkCfg) SetLock(chain *Chain) func() {
	c.Lock()
	chain.UseSDKContext()
	return c.Unlock
}
