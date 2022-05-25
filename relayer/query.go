package relayer

import (
	"context"
	"fmt"

	"github.com/avast/retry-go/v4"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v3/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	tmclient "github.com/cosmos/ibc-go/v3/modules/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// QueryLatestHeights returns the heights of multiple chains at once
func QueryLatestHeights(ctx context.Context, src, dst *Chain) (srch, dsth int64, err error) {
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		var err error
		srch, err = src.ChainProvider.QueryLatestHeight(egCtx)
		return err
	})
	eg.Go(func() error {
		var err error
		dsth, err = dst.ChainProvider.QueryLatestHeight(egCtx)
		return err
	})
	err = eg.Wait()
	return
}

// QueryConnectionPair returns a pair of connection responses
func QueryConnectionPair(ctx context.Context, src, dst *Chain, srcH, dstH int64) (srcConn, dstConn *conntypes.QueryConnectionResponse, err error) {
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		var err error
		srcConn, err = src.ChainProvider.QueryConnection(egCtx, srcH, src.ConnectionID())
		return err
	})
	eg.Go(func() error {
		var err error
		dstConn, err = dst.ChainProvider.QueryConnection(egCtx, dstH, dst.ConnectionID())
		return err
	})
	err = eg.Wait()
	return
}

// QueryChannelPair returns a pair of channel responses
func QueryChannelPair(ctx context.Context, src, dst *Chain, srcH, dstH int64, srcChanID, dstChanID, srcPortID, dstPortID string) (srcChan, dstChan *chantypes.QueryChannelResponse, err error) {
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		var err error
		srcChan, err = src.ChainProvider.QueryChannel(egCtx, srcH, srcChanID, srcPortID)
		return err
	})
	eg.Go(func() error {
		var err error
		dstChan, err = dst.ChainProvider.QueryChannel(egCtx, dstH, dstChanID, dstPortID)
		return err
	})
	if err = eg.Wait(); err != nil {
		return nil, nil, err
	}
	return
}

func QueryChannel(ctx context.Context, src *Chain, channelID string) (*chantypes.IdentifiedChannel, error) {
	var (
		srch        int64
		err         error
		srcChannels []*chantypes.IdentifiedChannel
	)

	// Query the latest height
	if err = retry.Do(func() error {
		var err error
		srch, err = src.ChainProvider.QueryLatestHeight(ctx)
		return err
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr); err != nil {
		return nil, err
	}

	// Query all channels for the given connection
	if err = retry.Do(func() error {
		srcChannels, err = src.ChainProvider.QueryConnectionChannels(ctx, srch, src.ConnectionID())
		return err
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		src.log.Info(
			"Failed to query connection channels",
			zap.String("conn_id", src.ConnectionID()),
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", RtyAttNum),
			zap.Error(err),
		)
	})); err != nil {
		return nil, err
	}

	// Find the specified channel in the slice of all channels
	for _, channel := range srcChannels {
		if channel.ChannelId == channelID {
			return channel, nil
		}
	}

	return nil, fmt.Errorf("channel{%s} not found for [%s] -> client{%s}@connection{%s}",
		channelID, src.ChainID(), src.ClientID(), src.ConnectionID())
}

// GetIBCUpdateHeaders returns a pair of IBC update headers which can be used to update an on chain light client
func GetIBCUpdateHeaders(ctx context.Context, srch, dsth int64, src, dst provider.ChainProvider, srcClientID, dstClientID string) (srcHeader, dstHeader ibcexported.ClientMessage, err error) {
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		var err error
		srcHeader, err = src.GetIBCUpdateHeader(egCtx, srch, dst, dstClientID)
		return err
	})
	eg.Go(func() error {
		var err error
		dstHeader, err = dst.GetIBCUpdateHeader(egCtx, dsth, src, srcClientID)
		return err
	})
	if err = eg.Wait(); err != nil {
		return nil, nil, err
	}
	return
}

func GetLightSignedHeadersAtHeights(ctx context.Context, src, dst *Chain, srch, dsth int64) (srcUpdateHeader, dstUpdateHeader ibcexported.ClientMessage, err error) {
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		var err error
		srcUpdateHeader, err = src.ChainProvider.GetLightSignedHeaderAtHeight(egCtx, srch)
		return err
	})
	eg.Go(func() error {
		var err error
		dstUpdateHeader, err = dst.ChainProvider.GetLightSignedHeaderAtHeight(egCtx, dsth)
		return err
	})
	if err := eg.Wait(); err != nil {
		return nil, nil, err
	}
	return
}

// QueryTMClientState retrieves the latest consensus state for a client in state at a given height
// and unpacks/cast it to tendermint clientstate
func (c *Chain) QueryTMClientState(ctx context.Context, height int64) (*tmclient.ClientState, error) {
	clientStateRes, err := c.ChainProvider.QueryClientStateResponse(ctx, height, c.ClientID())
	if err != nil {
		return &tmclient.ClientState{}, err
	}

	return CastClientStateToTMType(clientStateRes.ClientState)
}

// CastClientStateToTMType casts client state to tendermint type
func CastClientStateToTMType(cs *codectypes.Any) (*tmclient.ClientState, error) {
	clientStateExported, err := clienttypes.UnpackClientState(cs)
	if err != nil {
		return &tmclient.ClientState{}, err
	}

	// cast from interface to concrete type
	clientState, ok := clientStateExported.(*tmclient.ClientState)
	if !ok {
		return &tmclient.ClientState{},
			fmt.Errorf("error when casting exported clientstate to tendermint type")
	}

	return clientState, nil
}

//// QueryHistoricalInfo returns historical header data
//func (c *Chain) QueryHistoricalInfo(height clienttypes.Height) (*stakingtypes.QueryHistoricalInfoResponse, error) {
//	//TODO: use epoch number in query once SDK gets updated
//	qc := stakingtypes.NewQueryClient(c.CLIContext(0))
//	return qc.HistoricalInfo(context.Background(), &stakingtypes.QueryHistoricalInfoRequest{
//		Height: int64(height.GetRevisionHeight()),
//	})
//}
//
//// QueryValsetAtHeight returns the validator set at a given height
//func (c *Chain) QueryValsetAtHeight(height clienttypes.Height) (*tmproto.ValidatorSet, error) {
//	res, err := c.QueryHistoricalInfo(height)
//	if err != nil {
//		return nil, fmt.Errorf("chain(%s): %s", c.ChainID, err)
//	}
//
//	// create tendermint ValidatorSet from SDK Validators
//	tmVals, err := c.toTmValidators(res.Hist.Valset)
//	if err != nil {
//		return nil, err
//	}
//
//	sort.Sort(tmtypes.ValidatorsByVotingPower(tmVals))
//	tmValSet := &tmtypes.ValidatorSet{
//		Validators: tmVals,
//	}
//	tmValSet.GetProposer()
//
//	return tmValSet.ToProto()
//}
//
//func (c *Chain) toTmValidators(vals stakingtypes.Validators) ([]*tmtypes.Validator, error) {
//	validators := make([]*tmtypes.Validator, len(vals))
//	var err error
//	for i, val := range vals {
//		validators[i], err = c.toTmValidator(val)
//		if err != nil {
//			return nil, err
//		}
//	}
//
//	return validators, nil
//}
//
//func (c *Chain) toTmValidator(val stakingtypes.Validator) (*tmtypes.Validator, error) {
//	var pk cryptotypes.PubKey
//	if err := c.Encoding.Marshaler.UnpackAny(val.ConsensusPubkey, &pk); err != nil {
//		return nil, err
//	}
//	tmkey, err := cryptocodec.ToTmPubKeyInterface(pk)
//	if err != nil {
//		return nil, fmt.Errorf("pubkey not a tendermint pub key %s", err)
//	}
//	return tmtypes.NewValidator(tmkey, val.ConsensusPower(sdk.DefaultPowerReduction)), nil
//}
