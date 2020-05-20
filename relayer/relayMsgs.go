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

	MaxMsgLength uint64
	MaxTxSize    uint64

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

func (r *RelayMsgs) IsMaxTx(msgLen, txSize uint64) bool {
	return (r.MaxMsgLength != 0 && msgLen > r.MaxMsgLength) ||
		(r.MaxTxSize != 0 && txSize > r.MaxTxSize)
}

// Send sends the messages with appropriate output
// TODO: Parallelize? Maybe?
func (r *RelayMsgs) Send(src, dst *Chain) {
	var msgLen, txSize uint64
	var msgs []sdk.Msg

	r.success = true

	// submit batches of relay transactions
	for _, msg := range r.Src {
		msgLen++
		txSize += uint64(len(msg.GetSignBytes()))

		if r.IsMaxTx(msgLen, txSize) {
			// Submit the transactions to src chain and update its status
			r.success = r.success && send(src, msgs)

			// clear the current batch
			msgs = []sdk.Msg{}
		}
		msgs = append(msgs, msg)
	}

	// submit leftover msgs
	if len(msgs) > 0 && !send(src, msgs) {
		r.success = false
	}

	// reset variables
	msgLen, txSize = 0, 0
	msgs = []sdk.Msg{}

	for _, msg := range r.Dst {
		msgLen++
		txSize += uint64(len(msg.GetSignBytes()))

		if r.IsMaxTx(msgLen, txSize) {
			// Submit the transaction to dst chain and update its status
			r.success = r.success && send(dst, msgs)

			// clear the current batch
			msgs = []sdk.Msg{}
		}
		msgs = append(msgs, msg)
	}

	// submit leftover msgs
	if len(msgs) > 0 && !send(dst, msgs) {
		r.success = false
	}
}

func send(chain *Chain, msgs []sdk.Msg) bool {
	res, err := chain.SendMsgs(msgs)
	if err != nil || res.Code != 0 {
		chain.LogFailedTx(res, err, msgs)
		return false
	} else {
		// NOTE: Add more data to this such as identifiers
		chain.LogSuccessTx(res, msgs)
	}
	return true
}

func getMsgAction(msgs []sdk.Msg) string {
	var out string
	for i, msg := range msgs {
		out += fmt.Sprintf("%d:%s,", i, msg.Type())
	}
	return strings.TrimSuffix(out, ",")
}
