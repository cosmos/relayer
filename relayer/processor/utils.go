package processor

import (
	"math"
	"strings"

	"github.com/cosmos/relayer/v2/relayer/provider"
)

const clientName = "tendermint"

func ClientIsIcon(cs provider.ClientState) bool {
	if strings.Contains(cs.ClientID, clientName) {
		return true
	}
	return false
}

func findNextGreaterHeight(headercache IBCHeaderCache, prevHeight uint64) (uint64, bool) {
	minDiff := uint64(math.MaxUint64)
	var nextGreaterHeight uint64
	found := false

	for key := range headercache {
		if key > prevHeight && key-prevHeight < minDiff {
			minDiff = key - prevHeight
			nextGreaterHeight = key
			found = true
		}
	}

	if found {
		return nextGreaterHeight, true
	}
	return 0, false
}

func nextIconIBCHeader(heightMap IBCHeaderCache, height uint64) (provider.IBCHeader, bool) {
	var nextHeight uint64
	nextHeight = math.MaxUint64

	if height == 0 {
		return nil, false
	}
	for h := range heightMap {
		if h > height && h < nextHeight {
			nextHeight = h
		}
	}
	if nextHeight == math.MaxUint64 {
		return nil, false
	}

	header, ok := heightMap[nextHeight]
	return header, ok
}

// The next header is {<nil> false [] 0}  true
