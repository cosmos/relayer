package relayer

import (
	"fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// RelayMsgs contains the msgs that need to be sent to both a src and dst chain
// after a given relay round
type RelayMsgs struct {
	Src []sdk.Msg
	Dst []sdk.Msg

	last    bool
	success bool
}

// Ready returns true if there are messages to relay
func (r *RelayMsgs) Ready() bool {
	if r == nil {
		return false
	}

	if len(r.Src) == 0 && len(r.Dst) == 0 {
		return false
	}
	return true
}

// Success returns the success var
func (r *RelayMsgs) Success() bool {
	return r.success
}

// Send sends the messages with appropriate output
func (r *RelayMsgs) Send(src, dst *Chain) {
	var failed = false
	// TODO: maybe figure out a better way to indicate error here?

	// TODO: Parallelize? Maybe?
	if len(r.Src) > 0 {
		// Submit the transactions to src chain
		res, err := src.SendMsgs(r.Src)
		if err != nil || res.Code != 0 {
			src.LogFailedTx(res, err, r.Src)
			failed = true
		} else {
			// NOTE: Add more data to this such as identifiers
			src.LogSuccessTx(res, r.Src)
		}
	}

	if len(r.Dst) > 0 {
		// Submit the transactions to dst chain
		res, err := dst.SendMsgs(r.Dst)
		if err != nil || res.Code != 0 {
			dst.LogFailedTx(res, err, r.Dst)
			failed = true
		} else {
			// NOTE: Add more data to this such as identifiers
			dst.LogSuccessTx(res, r.Dst)

		}
	}

	if failed {
		r.success = false
		return
	}
	r.success = true
}

func getMsgAction(msgs []sdk.Msg) string {
	var out string
	for i, msg := range msgs {
		out += fmt.Sprintf("%d:%s,", i, msg.Type())
	}
	return strings.TrimSuffix(out, ",")
}
