package chains

import (
	abci "github.com/cometbft/cometbft/abci/types"
	legacyabci "github.com/strangelove-ventures/cometbft/abci/types"
)

// Results is a generalized type used in the relayer to handle the breaking changes in the CometBFT type,
// ResultBlockResults, that were introduced in v0.38.0.
type Results struct {
	TxsResults []*TxResult
	Events     []abci.Event
}

// TxResult is a generalized type used in the relayer to handle the breaking changes in the CometBFT type,
// ResultBlockResults, that were introduced in v0.38.0.
type TxResult struct {
	Code   uint32
	Events []abci.Event
}

// ConvertEvents converts a slice of the abci Event type imported from our forked CometBFT repo into
// a slice of the abci Event type imported from the upstream CometBFT repo.
func ConvertEvents(events []legacyabci.Event) []abci.Event {
	var cometEvents []abci.Event

	for _, event := range events {
		newEvent := abci.Event{
			Type: event.Type,
		}

		var attributes []abci.EventAttribute
		for _, attr := range event.Attributes {
			attributes = append(attributes, abci.EventAttribute{
				Key:   attr.Key,
				Value: attr.Value,
				Index: attr.Index,
			})
		}

		newEvent.Attributes = attributes

		cometEvents = append(cometEvents, newEvent)
	}

	return cometEvents
}

// ConvertTxResults converts a slice of the ResponseDeliverTx type imported from our forked CometBFT repo
// into a slice of our generalized TxResult type, so that we can properly handle the breaking changes introduced
// in CometBFT v0.38.0.
func ConvertTxResults(results []*legacyabci.ResponseDeliverTx) []*TxResult {
	var res []*TxResult

	for _, r := range results {
		newResult := &TxResult{
			Code:   r.Code,
			Events: ConvertEvents(r.Events),
		}

		res = append(res, newResult)
	}

	return res
}
