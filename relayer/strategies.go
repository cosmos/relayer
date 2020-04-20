package relayer

import (
	"context"
	"fmt"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

var (
	txEvents = "tm.event = 'Tx'"
	blEvents = "tm.event = 'NewBlock'"
)

// MustGetStrategy returns the strategy and panics on error
func (r *Path) MustGetStrategy() Strategy {
	strat, err := r.GetStrategy()
	if err != nil {
		panic(err)
	}
	return strat
}

// GetStrategy the strategy defined in the relay messages
func (r *Path) GetStrategy() (Strategy, error) {
	switch r.Strategy.Type {
	case NaiveStrategy{}.GetType():
		return NaiveStrategy{}.Init(r.Strategy)
	default:
		return nil, fmt.Errorf("invalid strategy: %s", r.Strategy.Type)
	}
}

// StrategyCfg defines which relaying strategy to take for a given path
type StrategyCfg struct {
	Type        string            `json:"type" yaml:"type"`
	Constraints map[string]string `json:"constraints,omitempty" yaml:"constraints,omitempty"`
}

// Strategy defines the interface that strategies must
type Strategy interface {
	// Used to initialize the strategy implemenation
	// and validate the data from the configuration
	Init(*StrategyCfg) (Strategy, error)

	// Used to return the configuration
	Cfg() *StrategyCfg

	// Used in constructing StrategyCfg
	GetType() string

	// Used in constructing StrategyCfg
	GetConstraints() map[string]string

	// Run starts the relayer
	// it returns a recieve only channel that is used to wait for
	// the strategy to be ready to relay, and a done channel
	// that shuts down the relayer when it is time to exit
	Run(*Chain, *Chain) (func(), error)
}

// NewNaiveStrategy Returns a new NaiveStrategy config
func NewNaiveStrategy() *StrategyCfg {
	return &StrategyCfg{
		Type: NaiveStrategy{}.GetType(),
	}
}

// NaiveStrategy is a relaying strategy where everything in a Path is relayed
type NaiveStrategy struct{}

// Init implements Strategy
func (nrs NaiveStrategy) Init(sc *StrategyCfg) (Strategy, error) {
	if sc.Type != nrs.GetType() {
		return nil, fmt.Errorf("wrong type")
	}
	if len(sc.Constraints) != len(nrs.GetConstraints()) {
		return nil, fmt.Errorf("invalid constraint")
	}
	return nrs, nil
}

// Cfg implements Strategy
func (nrs NaiveStrategy) Cfg() *StrategyCfg {
	return &StrategyCfg{
		Type:        nrs.GetType(),
		Constraints: nrs.GetConstraints(),
	}
}

// GetType implements Strategy
func (nrs NaiveStrategy) GetType() string {
	return "naive"
}

// GetConstraints implements Strategy
func (nrs NaiveStrategy) GetConstraints() map[string]string {
	return map[string]string{}
}

// Run implements Strategy and defines what actions are taken when the relayer runs
func (nrs NaiveStrategy) Run(src, dst *Chain) (func(), error) {
	doneChan := make(chan struct{})

	sh, err := NewSyncHeaders(src, dst)
	if err != nil {
		return nil, err
	}

	go nrsLoop(src, dst, doneChan, sh)

	sp, err := UnrelayedSequences(src, dst, sh)
	if err != nil {
		return nil, err
	}

	// first, we want to ensure that there are no packets remaining to be relayed
	if err := RelayPacketsOrderedChan(src, dst, sh, sp); err != nil {
		// TODO: some errors may leak here when there are no packets to be relayed
		// be on the lookout for that
		return nil, err
	}

	return func() { doneChan <- struct{}{} }, nil
}

func nrsLoop(src, dst *Chain, doneChan chan struct{}, sh *SyncHeaders) {
	var (
		srcTxEvents, srcBlockEvents, dstTxEvents, dstBlockEvents <-chan ctypes.ResultEvent
		srcTxCancel, srcBlockCancel, dstTxCancel, dstBlockCancel context.CancelFunc
		err                                                      error
	)

	// Start client for source chain
	if err = src.Start(); err != nil {
		src.Error(err)
		return
	}

	// Subscibe to txEvents from the source chain
	if srcTxEvents, srcTxCancel, err = src.Subscribe(txEvents); err != nil {
		src.Error(err)
		return
	}
	defer srcTxCancel()
	src.Log(fmt.Sprintf("- listening to tx events from %s...", src.ChainID))

	// Subscibe to blockEvents from the source chain
	if srcBlockEvents, srcBlockCancel, err = src.Subscribe(blEvents); err != nil {
		src.Error(err)
		return
	}
	defer srcBlockCancel()
	src.Log(fmt.Sprintf("- listening to block events from %s...", src.ChainID))

	// Subscribe to destination chain
	if err = dst.Start(); err != nil {
		dst.Error(err)
		return
	}

	// Subscibe to txEvents from the destination chain
	if dstTxEvents, dstTxCancel, err = dst.Subscribe(txEvents); err != nil {
		dst.Error(err)
		return
	}
	defer dstTxCancel()
	dst.Log(fmt.Sprintf("- listening to tx events from %s...", dst.ChainID))

	// Subscibe to blockEvents from the destination chain
	if dstBlockEvents, dstBlockCancel, err = dst.Subscribe(blEvents); err != nil {
		src.Error(err)
		return
	}
	defer dstBlockCancel()
	dst.Log(fmt.Sprintf("- listening to block events from %s...", dst.ChainID))

	// Listen to channels and take appropriate action
	for {
		select {
		case srcMsg := <-srcTxEvents:
			src.logTx(srcMsg.Events)
			go dst.handleEvents(src, srcMsg.Events, sh)
		case dstMsg := <-dstTxEvents:
			dst.logTx(dstMsg.Events)
			go src.handleEvents(dst, dstMsg.Events, sh)
		case srcMsg := <-srcBlockEvents:
			// TODO: Add debug block logging here
			if err = sh.Update(src); err != nil {
				src.Error(err)
			}
			go dst.handleEvents(src, srcMsg.Events, sh)
		case dstMsg := <-dstBlockEvents:
			// TODO: Add debug block logging here
			if err = sh.Update(dst); err != nil {
				dst.Error(err)
			}
			go src.handleEvents(dst, dstMsg.Events, sh)
		case <-doneChan:
			src.Log(fmt.Sprintf("- [%s]:{%s} <-> [%s]:{%s} relayer shutting down",
				src.ChainID, src.PathEnd.PortID, dst.ChainID, dst.PathEnd.PortID))
			close(doneChan)
			return
		}
	}
}

func (src *Chain) handleEvents(dst *Chain, events map[string][]string, sh *SyncHeaders) {
	rlyPackets, err := relayPacketsFromEventListener(events)
	if len(rlyPackets) > 0 && err == nil {
		sendTxFromEventPackets(src, dst, rlyPackets, sh)
	}
}
