package interchaintest_test

import (
	"context"
	"errors"

	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authTx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
)

// TODO: Once relayer updated for ibc-go/v7, replace with interchaintest's cosmos.PollForMessage function.
func pollForUpdateClient(ctx context.Context, chain ibc.Chain, startHeight, maxHeight uint64) (*clienttypes.MsgUpdateClient, error) {
	const secondsTimeout uint = 30
	c, err := rpchttp.NewWithTimeout(chain.GetHostRPCAddress(), "/websocket", secondsTimeout)
	if err != nil {
		panic(err)
	}

	doPoll := func(ctx context.Context, height uint64) (*clienttypes.MsgUpdateClient, error) {
		h := int64(height)
		block, err := c.Block(ctx, &h)
		if err != nil {
			return nil, err
		}
		for _, tx := range block.Block.Txs {
			sdkTx, err := decodeTX(chain.Config().EncodingConfig.InterfaceRegistry, tx)
			if err != nil {
				return nil, err
			}
			for _, msg := range sdkTx.GetMsgs() {
				if found, ok := msg.(*clienttypes.MsgUpdateClient); ok {
					return found, nil
				}
			}
		}
		return nil, errors.New("not found")
	}

	bp := testutil.BlockPoller[*clienttypes.MsgUpdateClient]{CurrentHeight: chain.Height, PollFunc: doPoll}
	result, err := bp.DoPoll(ctx, startHeight, maxHeight)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func decodeTX(interfaceRegistry codectypes.InterfaceRegistry, txbz []byte) (sdk.Tx, error) {
	cdc := codec.NewProtoCodec(interfaceRegistry)
	return authTx.DefaultTxDecoder(cdc)(txbz)
}
