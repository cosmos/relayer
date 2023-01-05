package ibctest_test

import (
	"context"
	"errors"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authTx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	"github.com/strangelove-ventures/ibctest/v5/ibc"
	"github.com/strangelove-ventures/ibctest/v5/test"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
)

// TODO: Once relayer updated for ibc-go/v6, replace with ibctest's cosmos.PollForMessage function.
func pollForUpdateClient(ctx context.Context, chain ibc.Chain, startHeight, maxHeight uint64) (*clienttypes.MsgUpdateClient, error) {
	const secondsTimeout uint = 30
	c, err := rpchttp.NewWithTimeout(chain.GetHostRPCAddress(), "/websocket", secondsTimeout)
	if err != nil {
		panic(err)
	}

	doPoll := func(ctx context.Context, height uint64) (any, error) {
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

	bp := test.BlockPoller{CurrentHeight: chain.Height, PollFunc: doPoll}
	result, err := bp.DoPoll(ctx, startHeight, maxHeight)
	if err != nil {
		return nil, err
	}
	return result.(*clienttypes.MsgUpdateClient), nil
}

func decodeTX(interfaceRegistry codectypes.InterfaceRegistry, txbz []byte) (sdk.Tx, error) {
	cdc := codec.NewProtoCodec(interfaceRegistry)
	return authTx.DefaultTxDecoder(cdc)(txbz)
}
