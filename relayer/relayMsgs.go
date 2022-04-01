package relayer

import (
	"context"

	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	"go.uber.org/zap"
)

// RelayMsgs contains the msgs that need to be sent to both a src and dst chain
// after a given relay round. MaxTxSize and MaxMsgLength are ignored if they are
// set to zero.
type RelayMsgs struct {
	Src          []provider.RelayerMessage `json:"src"`
	Dst          []provider.RelayerMessage `json:"dst"`
	MaxTxSize    uint64                    `json:"max_tx_size"`    // maximum permitted size of the msgs in a bundled relay transaction
	MaxMsgLength uint64                    `json:"max_msg_length"` // maximum amount of messages in a bundled relay transaction

	Last      bool `json:"last"`
	Succeeded bool `json:"success"`
}

// NewRelayMsgs returns an initialized version of relay messages
func NewRelayMsgs() *RelayMsgs {
	return &RelayMsgs{Src: []provider.RelayerMessage{}, Dst: []provider.RelayerMessage{}, Last: false, Succeeded: false}
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
	return r.Succeeded
}

func (r *RelayMsgs) IsMaxTx(msgLen, txSize uint64) bool {
	return (r.MaxMsgLength != 0 && msgLen > r.MaxMsgLength) ||
		(r.MaxTxSize != 0 && txSize > r.MaxTxSize)
}

func EncodeMsgs(c *Chain, msgs []provider.RelayerMessage) []string {
	outMsgs := make([]string, 0, len(msgs))
	for _, msg := range msgs {
		bz, err := c.Encoding.Amino.MarshalJSON(msg)
		if err != nil {
			msgField := zap.Skip()
			if cm, ok := msg.(cosmos.CosmosMessage); ok {
				msgField = zap.Object("msg", cm)
			}
			c.log.Warn(
				"Failed to marshal message to amino JSON",
				msgField,
				zap.Error(err),
			)
		} else {
			outMsgs = append(outMsgs, string(bz))
		}
	}
	return outMsgs
}

func DecodeMsgs(c *Chain, msgs []string) []provider.RelayerMessage {
	outMsgs := make([]provider.RelayerMessage, 0, len(msgs))
	for _, msg := range msgs {
		var sm provider.RelayerMessage
		err := c.Encoding.Amino.UnmarshalJSON([]byte(msg), &sm)
		if err != nil {
			c.log.Warn(
				"Failed to unmarshal amino JSON message",
				zap.Binary("msg", []byte(msg)), // Although presented as a string, this is a binary blob.
				zap.Error(err),
			)
		} else {
			outMsgs = append(outMsgs, sm)
		}
	}
	return outMsgs
}

func (r *RelayMsgs) Send(ctx context.Context, log *zap.Logger, src, dst *Chain) {
	//nolint:prealloc // can not be pre allocated
	var (
		msgLen, txSize uint64
		msgs           []provider.RelayerMessage
	)

	r.Succeeded = true

	// submit batches of relay transactions
	for _, msg := range r.Src {
		if msg != nil {
			bz, err := msg.MsgBytes()
			if err != nil {
				panic(err)
			}

			msgLen++
			txSize += uint64(len(bz))

			if r.IsMaxTx(msgLen, txSize) {
				// Submit the transactions to src chain and update its status
				res, success, err := src.ChainProvider.SendMessages(ctx, msgs)
				if err != nil {
					logFailedTx(log, src.ChainID(), res, err, msgs)
				}
				r.Succeeded = r.Succeeded && success

				// clear the current batch and reset variables
				msgLen, txSize = 1, uint64(len(bz))
				msgs = []provider.RelayerMessage{}
			}
			msgs = append(msgs, msg)
		}
	}

	// submit leftover msgs
	if len(msgs) > 0 {
		res, success, err := src.ChainProvider.SendMessages(ctx, msgs)
		if err != nil {
			logFailedTx(log, src.ChainID(), res, err, msgs)
		}

		r.Succeeded = success
	}

	// reset variables
	msgLen, txSize = 0, 0
	msgs = []provider.RelayerMessage{}

	for _, msg := range r.Dst {
		if msg != nil {
			bz, err := msg.MsgBytes()
			if err != nil {
				panic(err)
			}

			msgLen++
			txSize += uint64(len(bz))

			if r.IsMaxTx(msgLen, txSize) {
				// Submit the transaction to dst chain and update its status
				res, success, err := dst.ChainProvider.SendMessages(ctx, msgs)
				if err != nil {
					logFailedTx(log, dst.ChainID(), res, err, msgs)
				}

				r.Succeeded = r.Succeeded && success

				// clear the current batch and reset variables
				msgLen, txSize = 1, uint64(len(bz))
				msgs = []provider.RelayerMessage{}
			}
			msgs = append(msgs, msg)
		}
	}

	// submit leftover msgs
	if len(msgs) > 0 {
		res, success, err := dst.ChainProvider.SendMessages(ctx, msgs)
		if err != nil {
			logFailedTx(log, dst.ChainID(), res, err, msgs)
		}

		r.Succeeded = success
	}
}
