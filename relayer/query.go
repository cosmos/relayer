package relayer

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	sdk "github.com/cosmos/cosmos-sdk/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// QueryLatestHeights queries the heights of multiple chains at once
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

// QueryClientStates queries the client state of multiple chains at once
func QueryClientStates(ctx context.Context,
	srch, dsth int64,
	src, dst *Chain,
) (ibcexported.ClientState, ibcexported.ClientState, error) {
	var (
		srcClientState ibcexported.ClientState
		dstClientState ibcexported.ClientState
	)
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return retry.Do(func() error {
			var err error
			srcClientState, err = src.ChainProvider.QueryClientState(egCtx, srch, src.ClientID())
			if err != nil {
				return err
			}
			return nil
		}, retry.Context(egCtx), RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
			src.log.Info(
				"Failed to query client state when updating clients",
				zap.String("client_id", src.ClientID()),
				zap.Uint("attempt", n+1),
				zap.Uint("max_attempts", RtyAttNum),
				zap.Error(err),
			)
		}))
	})

	eg.Go(func() error {
		return retry.Do(func() error {
			var err error
			dstClientState, err = dst.ChainProvider.QueryClientState(egCtx, dsth, dst.ClientID())
			if err != nil {
				return err
			}
			return nil
		}, retry.Context(egCtx), RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
			dst.log.Info(
				"Failed to query client state when updating clients",
				zap.String("client_id", dst.ClientID()),
				zap.Uint("attempt", n+1),
				zap.Uint("max_attempts", RtyAttNum),
				zap.Error(err),
			)
		}))
	})
	if err := eg.Wait(); err != nil {
		return nil, nil, err
	}
	return srcClientState, dstClientState, nil
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

// QueryIBCUpdateHeaders returns a pair of IBC update headers which can be used to update an on chain light client
func QueryIBCUpdateHeaders(
	ctx context.Context,
	srcClientID, dstClientID string,
	src, dst provider.ChainProvider,
	srch, dsth int64,
	srcTrustedH, dstTrustedH int64,
) (srcHeader, dstHeader, srcTrustedHeader, dstTrustedHeader provider.IBCHeader, err error) {
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		var err error
		srcHeader, err = src.QueryIBCHeader(egCtx, srch)
		return err
	})
	eg.Go(func() error {
		var err error
		dstHeader, err = dst.QueryIBCHeader(egCtx, dsth)
		return err
	})
	eg.Go(func() error {
		var err error
		srcTrustedHeader, err = src.QueryIBCHeader(egCtx, srcTrustedH+1)
		return err
	})
	eg.Go(func() error {
		var err error
		dstTrustedHeader, err = dst.QueryIBCHeader(egCtx, dstTrustedH+1)
		return err
	})
	err = eg.Wait()
	return
}

func QueryIBCHeaders(ctx context.Context, src, dst *Chain, srch, dsth int64) (srcUpdateHeader, dstUpdateHeader provider.IBCHeader, err error) {
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		var err error
		srcUpdateHeader, err = src.ChainProvider.QueryIBCHeader(egCtx, srch)
		return err
	})
	eg.Go(func() error {
		var err error
		dstUpdateHeader, err = dst.ChainProvider.QueryIBCHeader(egCtx, dsth)
		return err
	})
	err = eg.Wait()
	return
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

func QueryClientExpiration(ctx context.Context, src, dst *Chain) (time.Time, ClientStateInfo, error) {
	latestHeight, err := src.ChainProvider.QueryLatestHeight(ctx)
	if err != nil {
		return time.Time{}, ClientStateInfo{}, err
	}

	clientStateRes, err := src.ChainProvider.QueryClientStateResponse(ctx, latestHeight, src.ClientID())
	if err != nil {
		return time.Time{}, ClientStateInfo{}, err
	}

	clientInfo, err := ClientInfoFromClientState(clientStateRes.ClientState)
	if err != nil {
		return time.Time{}, ClientStateInfo{}, err
	}

	clientTime, err := dst.ChainProvider.BlockTime(ctx, int64(clientInfo.LatestHeight.GetRevisionHeight()))
	if err != nil {
		return time.Time{}, ClientStateInfo{}, err
	}

	return clientTime.Add(clientInfo.TrustingPeriod), clientInfo, nil
}

func SPrintClientExpiration(chain *Chain, expiration time.Time, clientInfo ClientStateInfo) string {
	now := time.Now()
	remainingTime := expiration.Sub(now)
	expirationFormatted := expiration.Format(time.RFC822)

	var status string
	if remainingTime <= 0 {
		status = "EXPIRED"
	} else {
		status = "GOOD"
	}

	legacyOutput := fmt.Sprintf(`
	client: %s (%s)
		HEALTH:              %s
		TIME:                %s (%s)
		LAST UPDATE HEIGHT:  %d
		TRUSTING PERIOD:     %s
	`,
		chain.ClientID(), chain.ChainID(), status, expirationFormatted, remainingTime.Round(time.Second), clientInfo.LatestHeight.GetRevisionHeight(), clientInfo.TrustingPeriod.String())

	return legacyOutput

}

// Returns clientExpiration data in JSON format.
func SPrintClientExpirationJson(chain *Chain, expiration time.Time, clientInfo ClientStateInfo) string {
	now := time.Now()
	remainingTime := expiration.Sub(now)
	expirationFormatted := expiration.Format(time.RFC822)

	var status string
	if remainingTime <= 0 {
		status = "EXPIRED"
	} else {
		status = "GOOD"
	}

	data := map[string]string{
		"client":             fmt.Sprintf("%s (%s)", chain.ClientID(), chain.ChainID()),
		"HEALTH":             status,
		"TIME":               fmt.Sprintf("%s (%s)", expirationFormatted, remainingTime.Round(time.Second)),
		"LAST UPDATE HEIGHT": strconv.FormatUint(clientInfo.LatestHeight.GetRevisionHeight(), 10),
		"TRUSTING PERIOD":    clientInfo.TrustingPeriod.String(),
	}

	jsonOutput, err := json.Marshal(data)
	if err != nil {
		jsonOutput = []byte{}
	}

	return string(jsonOutput)
}
