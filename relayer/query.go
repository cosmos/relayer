package relayer

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/avast/retry-go/v4"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v4/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v4/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v4/modules/core/exported"
	tmclient "github.com/cosmos/ibc-go/v4/modules/light-clients/07-tendermint/types"
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

func QueryPortChannel(ctx context.Context, src *Chain, portID string) (*chantypes.IdentifiedChannel, error) {
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
	var sb strings.Builder
	for i, channel := range srcChannels {
		if channel.PortId == portID {
			return channel, nil
		}
		if i != 0 {
			sb.WriteString(",")
		}
		sb.WriteString(channel.ChannelId)
		sb.WriteString(":")
		sb.WriteString(channel.PortId)
	}

	return nil, fmt.Errorf("channel with port{%s} not found for [%s] -> client{%s}@connection{%s}channels{%s}",
		portID, src.ChainID(), src.ClientID(), src.ConnectionID(), sb.String())
}

// GetIBCUpdateHeaders returns a pair of IBC update headers which can be used to update an on chain light client
func GetIBCUpdateHeaders(ctx context.Context, srch, dsth int64, src, dst provider.ChainProvider, srcClientID, dstClientID string) (srcHeader, dstHeader ibcexported.Header, err error) {
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

func GetLightSignedHeadersAtHeights(ctx context.Context, src, dst *Chain, srch, dsth int64) (srcUpdateHeader, dstUpdateHeader ibcexported.Header, err error) {
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

// QueryBalance is a helper function for query balance
func QueryBalance(ctx context.Context, chain *Chain, address string, showDenoms bool) (sdk.Coins, error) {
	coins, err := chain.ChainProvider.QueryBalanceWithAddress(ctx, address)
	if err != nil {
		return nil, err
	}

	if showDenoms {
		return coins, nil
	}

	h, err := chain.ChainProvider.QueryLatestHeight(ctx)
	if err != nil {
		return nil, err
	}

	dts, err := chain.ChainProvider.QueryDenomTraces(ctx, 0, 1000, h)
	if err != nil {
		return nil, err
	}

	if len(dts) == 0 {
		return coins, nil
	}

	var out sdk.Coins
	for _, c := range coins {
		if c.Amount.Equal(sdk.NewInt(0)) {
			continue
		}

		for i, d := range dts {
			if strings.EqualFold(c.Denom, d.IBCDenom()) {
				out = append(out, sdk.Coin{Denom: d.GetFullDenomPath(), Amount: c.Amount})
				break
			}

			if i == len(dts)-1 {
				out = append(out, c)
			}
		}
	}
	return out, nil
}

// QueryHeader is a helper function for query header
func QueryHeader(ctx context.Context, chain *Chain, opts ...string) (ibcexported.Header, error) {
	if len(opts) > 0 {
		height, err := strconv.ParseInt(opts[0], 10, 64) //convert to int64
		if err != nil {
			return nil, err
		}

		return chain.ChainProvider.QueryHeaderAtHeight(ctx, height)
	}

	return chain.ChainProvider.GetLightSignedHeaderAtHeight(ctx, 0)
}
