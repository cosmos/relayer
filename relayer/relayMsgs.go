package relayer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"
)

// RelayMsgs contains the msgs that need to be sent to both a src and dst chain
// after a given relay round. MaxTxSize and MaxMsgLength are ignored if they are
// set to zero.
type RelayMsgs struct {
	Src          []provider.RelayerMessage `json:"src"`
	Dst          []provider.RelayerMessage `json:"dst"`
	MaxTxSize    uint64                    `json:"max_tx_size"`    // maximum permitted size of the msgs in a bundled relay transaction
	MaxMsgLength uint64                    `json:"max_msg_length"` // maximum amount of messages in a bundled relay transaction
}

// batchSendMessageTimeout is the timeout for sending a single batch of IBC messages to an RPC node.
const batchSendMessageTimeout = 30 * time.Second

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

func (r *RelayMsgs) PrependMsgUpdateClient(
	ctx context.Context,
	src, dst *Chain,
	srch, dsth int64,
) error {
	eg, egCtx := errgroup.WithContext(ctx)
	if len(r.Src) > 0 {
		eg.Go(func() error {
			srcMsgUpdateClient, err := MsgUpdateClient(egCtx, dst, src, dsth, srch)
			if err != nil {
				return err
			}
			r.Src = append([]provider.RelayerMessage{srcMsgUpdateClient}, r.Src...)
			return nil
		})
	}
	if len(r.Dst) > 0 {
		eg.Go(func() error {
			dstMsgUpdateClient, err := MsgUpdateClient(egCtx, src, dst, srch, dsth)
			if err != nil {
				return err
			}
			r.Dst = append([]provider.RelayerMessage{dstMsgUpdateClient}, r.Dst...)
			return nil
		})
	}

	return eg.Wait()
}

func (r *RelayMsgs) IsMaxTx(msgLen, txSize uint64) bool {
	return (r.MaxMsgLength != 0 && msgLen > r.MaxMsgLength) ||
		(r.MaxTxSize != 0 && txSize > r.MaxTxSize)
}

// RelayMsgSender is a narrow subset of a Chain,
// to simplify testing methods on RelayMsgs.
type RelayMsgSender struct {
	ChainID string

	// SendMessages is a function matching the signature of the same method
	// on the ChainProvider interface.
	//
	// Accepting this narrow subset of the interface greatly simplifies testing.
	SendMessages func(context.Context, []provider.RelayerMessage, string) (*provider.RelayerTxResponse, bool, error)
}

// AsRelayMsgSender converts c to a RelayMsgSender.
func AsRelayMsgSender(c *Chain) RelayMsgSender {
	return RelayMsgSender{
		ChainID:      c.ChainID(),
		SendMessages: c.ChainProvider.SendMessages,
	}
}

// SendMsgsResult is returned by (*RelayMsgs).Send.
// It contains details about the distinct results
// of sending messages to the corresponding chains.
type SendMsgsResult struct {
	// Count of successfully sent batches,
	// where "successful" means there was no error in sending the batch across the network,
	// and the remote end sent a response indicating success.
	SuccessfulSrcBatches, SuccessfulDstBatches int

	// Accumulation of errors encountered when sending to source or destination.
	// If multiple errors occurred, these will be multierr errors
	// which are displayed nicely through zap logging.
	SrcSendError, DstSendError error
}

// SuccessfullySent reports the presence successfully sent batches
func (r SendMsgsResult) SuccessfullySent() bool {
	return (r.SuccessfulSrcBatches > 0 || r.SuccessfulDstBatches > 0)
}

// PartiallySent reports the presence of both some successfully sent batches
// and some errors.
func (r SendMsgsResult) PartiallySent() bool {
	return (r.SuccessfulSrcBatches > 0 || r.SuccessfulDstBatches > 0) &&
		(r.SrcSendError != nil || r.DstSendError != nil)
}

// Error returns any accumulated erors that occurred while sending messages.
func (r SendMsgsResult) Error() error {
	return multierr.Append(r.SrcSendError, r.DstSendError)
}

// MarshalLogObject satisfies the zapcore.ObjectMarshaler interface
// so that you can use zap.Object("send_result", r) when logging.
// This is typically useful when logging details about a partially sent result.
func (r SendMsgsResult) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddInt("successful_src_batches", r.SuccessfulSrcBatches)
	enc.AddInt("successful_dst_batches", r.SuccessfulDstBatches)
	if r.SrcSendError == nil {
		enc.AddString("src_send_errors", "<nil>")
	} else {
		enc.AddString("src_send_errors", r.SrcSendError.Error())
	}
	if r.DstSendError == nil {
		enc.AddString("dst_send_errors", "<nil>")
	} else {
		enc.AddString("dst_send_errors", r.DstSendError.Error())
	}

	return nil
}

// Send concurrently sends out r's messages to the corresponding RelayMsgSenders.
func (r *RelayMsgs) Send(ctx context.Context, log *zap.Logger, src, dst RelayMsgSender, memo string) SendMsgsResult {
	var (
		wg     sync.WaitGroup
		result SendMsgsResult
	)

	if len(r.Src) > 0 {
		wg.Add(1)
		go r.send(ctx, log, &wg, src, r.Src, memo, &result.SuccessfulSrcBatches, &result.SrcSendError)
	}
	if len(r.Dst) > 0 {
		wg.Add(1)
		go r.send(ctx, log, &wg, dst, r.Dst, memo, &result.SuccessfulDstBatches, &result.DstSendError)
	}

	wg.Wait()
	return result
}

func (r *RelayMsgs) send(
	ctx context.Context,
	log *zap.Logger,
	wg *sync.WaitGroup,
	s RelayMsgSender,
	msgs []provider.RelayerMessage,
	memo string,
	successes *int,
	errors *error,
) {
	defer wg.Done()

	var txSize, batchStartIdx uint64

	for i, msg := range msgs {
		// The previous version of this was skipping nil messages;
		// instead, we should not allow code to include nil messages.
		if msg == nil {
			panic(fmt.Errorf("send: invalid nil message at index %d", i))
		}

		bz, err := msg.MsgBytes()
		if err != nil {
			panic(err)
		}

		if !r.IsMaxTx(uint64(i)+1-batchStartIdx, txSize+uint64(len(bz))) {
			// We can add another transaction, so increase the transaction size counter
			// and proceed to the next message.
			txSize += uint64(len(bz))
			continue
		}

		// Otherwise, we have reached the message count limit or the byte size limit.
		// Send out this batch now.
		batchMsgs := msgs[batchStartIdx:i]
		batchCtx, batchCtxCancel := context.WithTimeout(ctx, batchSendMessageTimeout)
		resp, success, err := s.SendMessages(batchCtx, batchMsgs, memo)
		batchCtxCancel()
		if err != nil {
			logFailedTx(log, s.ChainID, resp, err, batchMsgs)
			multierr.AppendInto(errors, err)
			// TODO: check chain ordering.
			// If chain is unordered, we can keep sending;
			// otherwise we need to stop now.
		}
		if success {
			*successes++
		}

		// Reset counters.
		batchStartIdx = uint64(i)
		txSize = uint64(len(bz))
	}

	// If there are any messages left over, send those out too.
	if batchStartIdx < uint64(len(msgs)) {
		batchMsgs := msgs[batchStartIdx:]
		batchCtx, batchCtxCancel := context.WithTimeout(ctx, batchSendMessageTimeout)
		resp, success, err := s.SendMessages(batchCtx, batchMsgs, memo)
		batchCtxCancel()
		if err != nil {
			logFailedTx(log, s.ChainID, resp, err, batchMsgs)
			multierr.AppendInto(errors, err)
		}
		if success {
			*successes++
		}
	}
}
