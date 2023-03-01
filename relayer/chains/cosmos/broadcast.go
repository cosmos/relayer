package cosmos

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
)

const (
	ErrTimeoutAfterWaitingForTxBroadcast _err = "timed out after waiting for tx to get included in the block"
)

type _err string

func (e _err) Error() string { return string(e) }

type rpcTxBroadcaster interface {
	Tx(ctx context.Context, hash []byte, prove bool) (*ctypes.ResultTx, error)
	BroadcastTxSync(context.Context, tmtypes.Tx) (*ctypes.ResultBroadcastTx, error)
	Block(ctx context.Context, height *int64) (*ctypes.ResultBlock, error)
	BlockResults(ctx context.Context, height *int64) (*ctypes.ResultBlockResults, error)
	Status(context.Context) (*ctypes.ResultStatus, error)

	// TODO: implement commit and async as well
	// BroadcastTxCommit(context.Context, tmtypes.Tx) (*ctypes.ResultBroadcastTxCommit, error)
	// BroadcastTxAsync(context.Context, tmtypes.Tx) (*ctypes.ResultBroadcastTx, error)
}

func (cc *CosmosProvider) BroadcastTx(ctx context.Context, tx []byte) (*sdk.TxResponse, error) {
	var (
		blockTimeout = defaultBroadcastWaitTimeout
		err          error
	)

	if cc.PCfg.BlockTimeout != "" {
		blockTimeout, err = time.ParseDuration(cc.PCfg.BlockTimeout)
		if err != nil {
			// Did you call Validate() method on ChainClientConfig struct
			// before coming here?
			return nil, err
		}
	}

	return broadcastTx(
		ctx,
		cc.RPCClient,
		cc.Codec.TxConfig.TxDecoder(),
		tx,
		blockTimeout,
	)
}

// broadcastTx broadcasts a TX and then waits for the TX to be included in the block.
// The waiting will either be canceled after the waitTimeout has run out or the context
// exited.
func broadcastTx(
	ctx context.Context,
	broadcaster rpcTxBroadcaster,
	txDecoder sdk.TxDecoder,
	tx []byte,
	waitTimeout time.Duration,
) (*sdk.TxResponse, error) {
	// broadcast tx sync waits for check tx to pass
	// NOTE: this can return w/ a timeout
	// need to investigate if this will leave the tx
	// in the mempool or we can retry the broadcast at that
	// point

	syncRes, err := broadcaster.BroadcastTxSync(ctx, tx)
	if err != nil {
		if syncRes == nil {
			// There are some cases where BroadcastTxSync will return an error but the associated
			// ResultBroadcastTx will be nil.
			return nil, err
		}
		return &sdk.TxResponse{
			Code:      syncRes.Code,
			Codespace: syncRes.Codespace,
			TxHash:    syncRes.Hash.String(),
		}, err
	}

	// ABCIError will return an error other than "unknown" if syncRes.Code is a registered error in syncRes.Codespace
	// This catches all of the sdk errors https://github.com/cosmos/cosmos-sdk/blob/f10f5e5974d2ecbf9efc05bc0bfe1c99fdeed4b6/types/errors/errors.go
	err = errors.Unwrap(sdkerrors.ABCIError(syncRes.Codespace, syncRes.Code, "error broadcasting transaction"))
	if err.Error() != errUnknown {
		return nil, err
	}

	// Check is indexing is disabled...
	// if tx index is enabled and we found the result, send the results
	// if tx index is disabled, poll for inclusion
	// if tx index is enabled and we didn't find the tx, begin polling.

	resTx, err := broadcaster.Tx(ctx, syncRes.Hash, false)
	if err == nil {
		return mkTxResult(txDecoder, resTx)
	}
	if err != nil && strings.Contains(err.Error(), "transaction indexing is disabled") {
		return waitForBlockInclusion(ctx, broadcaster, txDecoder, syncRes.Hash, waitTimeout)
	}

	// wait for tx to be included in a block
	exitAfter := time.After(waitTimeout)
	for {
		select {
		case <-exitAfter:
			return nil, fmt.Errorf("timed out after: %d; %w", waitTimeout, ErrTimeoutAfterWaitingForTxBroadcast)
		// TODO: this is potentially less than optimal and may
		// be better as something configurable
		case <-time.After(time.Millisecond * 100):
			resTx, err := broadcaster.Tx(ctx, syncRes.Hash, false)
			if err == nil {
				return mkTxResult(txDecoder, resTx)
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// waitForBlockInclusion will wait for a transaction to be included in a block, up to waitTimeout or context cancellation.
func waitForBlockInclusion(
	ctx context.Context,
	broadcaster rpcTxBroadcaster,
	txDecoder sdk.TxDecoder,
	txHash []byte,
	waitTimeout time.Duration,
) (*sdk.TxResponse, error) {

	// Figure out what the current height is
	res, err := broadcaster.Status(ctx)
	if err != nil {
		return nil, err
	}

	// This is likely overly cautious.
	// We're in a bit of a race, between the transactions broadcast, attempting to lookup by hash, getting status.
	// Let's check back two blocks and iterate a couple times until we find the txn or die
	nextHeight := res.SyncInfo.LatestBlockHeight - 2
	exitAfter := time.After(waitTimeout)

	for {
		select {
		case <-exitAfter:
			return nil, fmt.Errorf("timed out after: %d; %w", waitTimeout, ErrTimeoutAfterWaitingForTxBroadcast)
		// This fixed poll is fine because it's only for logging and updating prometheus metrics currently.
		case <-time.After(time.Millisecond * 100):
			// Look up the latest block
			res, err := broadcaster.Block(ctx, &nextHeight)
			if err != nil {
				// If this fails, try again
				continue
			}

			// Is the transaction in this block?
			index := res.Block.Txs.IndexByHash(txHash)
			if index != -1 {
				// Transaction is not in the block, look to the next block
				nextHeight++
				continue
			}

			txResp, err := mkTxResultFromResultBlock(ctx, broadcaster, txDecoder, txHash, res, index)
			if err != nil {
				// If this fails, try again with the same block
				continue
			}

			return txResp, nil

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// mkTxResultFromResultBlock decodes a tendermint block into an SDK TxResponse.
func mkTxResultFromResultBlock(ctx context.Context, broadcaster rpcTxBroadcaster, txDecoder sdk.TxDecoder, txHash []byte, res *coretypes.ResultBlock, index int) (*sdk.TxResponse, error) {

	// We need the block results, the decoded txn and the parsed logs
	results, err := broadcaster.BlockResults(ctx, &res.Block.Height)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve block results after finding block at height: %d", res.Block.Height)
	}

	// Let's now roll this up into a ResultTx and use the existing mkTxResult
	resTx := coretypes.ResultTx{
		Hash:     txHash,
		Height:   res.Block.Height,
		Index:    uint32(index),
		TxResult: *results.TxsResults[index],
		Tx:       res.Block.Txs[index],
		Proof:    res.Block.Txs.Proof(index),
	}

	return mkTxResult(txDecoder, &resTx)
}

func mkTxResult(txDecoder sdk.TxDecoder, resTx *ctypes.ResultTx) (*sdk.TxResponse, error) {
	txb, err := txDecoder(resTx.Tx)
	if err != nil {
		return nil, err
	}
	p, ok := txb.(intoAny)
	if !ok {
		return nil, fmt.Errorf("expecting a type implementing intoAny, got: %T", txb)
	}
	any := p.AsAny()
	// TODO: maybe don't make up the time here?
	// we can fetch the block for the block time buts thats
	// more round trips
	// TODO: logs get rendered as base64 encoded, need to fix this somehow
	return sdk.NewResponseResultTx(resTx, any, time.Now().Format(time.RFC3339)), nil
}

// Deprecated: this interface is used only internally for scenario we are
// deprecating (StdTxConfig support)
type intoAny interface {
	AsAny() *codectypes.Any
}
