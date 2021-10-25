package relayer

import (
	"context"
	"fmt"

	tmservice "github.com/tendermint/tendermint/libs/service"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
)

var (
	txEvents = "tm.event='Tx'"
	blEvents = "tm.event='NewBlock'"
)

// Strategy defines
type Strategy interface {
	GetType() string
	HandleEvents(src, dst *Chain, srch, dsth int64, events map[string][]string)
	UnrelayedSequences(src, dst *Chain) (*RelaySequences, error)
	UnrelayedAcknowledgements(src, dst *Chain) (*RelaySequences, error)
	RelayPackets(src, dst *Chain, sp *RelaySequences) error
	RelayAcknowledgements(src, dst *Chain, sp *RelaySequences) error
}

// MustGetStrategy returns the strategy and panics on error
func (p *Path) MustGetStrategy() Strategy {
	strategy, err := p.GetStrategy()
	if err != nil {
		panic(err)
	}

	return strategy
}

// GetStrategy the strategy defined in the relay messages
func (p *Path) GetStrategy() (Strategy, error) {
	switch p.Strategy.Type {
	case (&NaiveStrategy{}).GetType():
		return &NaiveStrategy{}, nil
	default:
		return nil, fmt.Errorf("invalid strategy: %s", p.Strategy.Type)
	}
}

// StrategyCfg defines which relaying strategy to take for a given path
type StrategyCfg struct {
	Type string `json:"type" yaml:"type"`
}

// RunStrategy runs a given strategy
func RunStrategy(src, dst *Chain, strategy Strategy) (func(), error) {
	doneChan := make(chan struct{})

	// Fetch latest headers for each chain and store them in sync headers
	// _, _, err := UpdateLightClients(src, dst)
	// if err != nil {
	// 	return nil, err
	// }

	// Next start the goroutine that listens to each chain for block and tx events
	go relayerListenLoop(src, dst, doneChan, strategy)

	// Fetch any unrelayed sequences depending on the channel order
	sp, err := strategy.UnrelayedSequences(src, dst)
	if err != nil {
		return nil, err
	}

	if err = strategy.RelayPackets(src, dst, sp); err != nil {
		return nil, err
	}

	// Return a function to stop the relayer goroutine
	return func() { doneChan <- struct{}{} }, nil
}

func relayerListenLoop(src, dst *Chain, doneChan chan struct{}, strategy Strategy) {
	var (
		srcTxEvents, srcBlockEvents, dstTxEvents, dstBlockEvents <-chan ctypes.ResultEvent
		srcTxCancel, srcBlockCancel, dstTxCancel, dstBlockCancel context.CancelFunc
		err                                                      error
	)

	// Start client for source chain
	if err = src.Start(); err != nil {
		if err != tmservice.ErrAlreadyStarted {
			src.Error(err)
			return
		}
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
		if err != tmservice.ErrAlreadyStarted {
			dst.Error(err)
			return
		}
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
	var srch, dsth int64
	for {
		select {
		case srcMsg := <-srcTxEvents:
			src.logTx(srcMsg.Events)
			go strategy.HandleEvents(dst, src, dsth, srch, srcMsg.Events)
		case dstMsg := <-dstTxEvents:
			dst.logTx(dstMsg.Events)
			go strategy.HandleEvents(src, dst, srch, dsth, dstMsg.Events)
		case srcMsg := <-srcBlockEvents:
			bl, _ := srcMsg.Data.(tmtypes.EventDataNewBlock)
			srch = bl.Block.Height
			go strategy.HandleEvents(dst, src, dsth, srch, srcMsg.Events)
		case dstMsg := <-dstBlockEvents:
			bl, _ := dstMsg.Data.(tmtypes.EventDataNewBlock)
			dsth = bl.Block.Height
			go strategy.HandleEvents(src, dst, srch, dsth, dstMsg.Events)
		case <-doneChan:
			src.Log(fmt.Sprintf("- [%s]:{%s} <-> [%s]:{%s} relayer shutting down",
				src.ChainID, src.PathEnd.PortID, dst.ChainID, dst.PathEnd.PortID))
			close(doneChan)
			return
		}
	}
}
