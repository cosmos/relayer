package cosmos

import (
	"context"
	"errors"
	"fmt"
	"time"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
)

const (
	defaultBroadcastWaitTimeout               = 10 * time.Minute
	errUnknown                                = "unknown"
	ErrTimeoutAfterWaitingForTxBroadcast _err = "timed out after waiting for tx to get included in the block"
)

type _err string

func (e _err) Error() string { return string(e) }

type rpcTxBroadcaster interface {
	Tx(ctx context.Context, hash []byte, prove bool) (*ctypes.ResultTx, error)
	BroadcastTxSync(context.Context, tmtypes.Tx) (*ctypes.ResultBroadcastTx, error)

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

	// TODO: maybe we need to check if the node has tx indexing enabled?
	// if not, we need to find a new way to block until inclusion in a block

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
