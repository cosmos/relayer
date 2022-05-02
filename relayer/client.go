package relayer

import (
	"context"
	"fmt"
	"time"

	"github.com/avast/retry-go/v4"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// CreateClients creates clients for src on dst and dst on src if the client ids are unspecified.
func (c *Chain) CreateClients(ctx context.Context, dst *Chain, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour, override bool) (bool, error) {
	// Query the latest heights on src and dst and retry if the query fails
	var srch, dsth int64
	if err := retry.Do(func() error {
		var err error
		srch, dsth, err = QueryLatestHeights(ctx, c, dst)
		if srch == 0 || dsth == 0 || err != nil {
			return fmt.Errorf("failed to query latest heights: %w", err)
		}
		return nil
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr); err != nil {
		return false, err
	}

	// Query the light signed headers for src & dst at the heights srch & dsth, retry if the query fails
	var srcUpdateHeader, dstUpdateHeader ibcexported.Header
	if err := retry.Do(func() error {
		var err error
		srcUpdateHeader, dstUpdateHeader, err = GetLightSignedHeadersAtHeights(ctx, c, dst, srch, dsth)
		if err != nil {
			return err
		}
		return nil
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		c.log.Info(
			"Failed to get light signed headers",
			zap.String("src_chain_id", c.ChainID()),
			zap.Int64("src_height", srch),
			zap.String("dst_chain_id", dst.ChainID()),
			zap.Int64("dst_height", dsth),
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", RtyAttNum),
			zap.Error(err),
		)
		srch, dsth, _ = QueryLatestHeights(ctx, c, dst)
	})); err != nil {
		return false, err
	}

	var modifiedSrc, modifiedDst bool
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		var err error
		// Create client on src for dst if the client id is unspecified
		modifiedSrc, err = CreateClient(egCtx, c, dst, srcUpdateHeader, dstUpdateHeader, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour, override)
		if err != nil {
			return fmt.Errorf("failed to create client on src chain{%s}: %w", c.ChainID(), err)
		}
		return nil
	})

	eg.Go(func() error {
		var err error
		// Create client on dst for src if the client id is unspecified
		modifiedDst, err = CreateClient(egCtx, dst, c, dstUpdateHeader, srcUpdateHeader, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour, override)
		if err != nil {
			return fmt.Errorf("failed to create client on dst chain{%s}: %w", dst.ChainID(), err)
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		// If one completed successfully and the other didn't, we can still report modified.
		return modifiedSrc || modifiedDst, err
	}

	c.log.Info(
		"Clients created",
		zap.String("src_client_id", c.PathEnd.ClientID),
		zap.String("src_chain_id", c.ChainID()),
		zap.String("dst_client_id", dst.PathEnd.ClientID),
		zap.String("dst_chain_id", dst.ChainID()),
	)

	return modifiedSrc || modifiedDst, nil
}

func CreateClient(ctx context.Context, src, dst *Chain, srcUpdateHeader, dstUpdateHeader ibcexported.Header, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour, override bool) (bool, error) {
	// If a client ID was specified in the path, ensure it exists.
	if src.PathEnd.ClientID != "" {
		// TODO: check client is not expired
		_, err := src.ChainProvider.QueryClientStateResponse(ctx, int64(srcUpdateHeader.GetHeight().GetRevisionHeight()), src.ClientID())
		if err != nil {
			return false, fmt.Errorf("please ensure provided on-chain client (%s) exists on the chain (%s): %v",
				src.PathEnd.ClientID, src.ChainID(), err)
		}

		return false, nil
	}

	// Otherwise, create client for the destination chain on the source chain.

	// Query the trusting period for dst and retry if the query fails
	var tp time.Duration
	if err := retry.Do(func() error {
		var err error
		tp, err = dst.GetTrustingPeriod(ctx)
		if err != nil {
			return fmt.Errorf("failed to get trusting period for chain{%s}: %w", dst.ChainID(), err)
		}
		if tp == 0 {
			return fmt.Errorf("chain %s reported invalid zero trusting period", dst.ChainID())
		}
		return nil
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr); err != nil {
		return false, err
	}

	src.log.Debug(
		"Creating client",
		zap.String("src_chain_id", src.ChainID()),
		zap.String("dst_chain_id", dst.ChainID()),
		zap.Uint64("dst_header_height", dstUpdateHeader.GetHeight().GetRevisionHeight()),
		zap.Duration("trust_period", tp),
	)

	// Query the unbonding period for dst and retry if the query fails
	var ubdPeriod time.Duration
	if err := retry.Do(func() error {
		var err error
		ubdPeriod, err = dst.ChainProvider.QueryUnbondingPeriod(ctx)
		if err != nil {
			return fmt.Errorf("failed to query unbonding period for chain{%s}: %w", dst.ChainID(), err)
		}
		return nil
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr); err != nil {
		return false, err
	}

	// Create the ClientState we want on 'src' tracking 'dst'
	clientState, err := src.ChainProvider.NewClientState(dstUpdateHeader, tp, ubdPeriod, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour)
	if err != nil {
		return false, fmt.Errorf("failed to create new client state for chain{%s} tracking chain{%s}: %w", src.ChainID(), dst.ChainID(), err)
	}

	var clientID string
	var found bool
	// Will not reuse same client if override is true
	if !override {
		// Check if an identical light client already exists
		clientID, found = src.ChainProvider.FindMatchingClient(ctx, dst.ChainProvider, clientState)
	}

	if found && !override {
		src.log.Debug(
			"Client already exists",
			zap.String("client_id", clientID),
			zap.String("src_chain_id", src.ChainID()),
			zap.String("dst_chain_id", dst.ChainID()),
		)
		src.PathEnd.ClientID = clientID
		return true, nil
	}

	src.log.Debug(
		"No client found on source chain tracking the state of counterparty chain; creating client",
		zap.String("src_chain_id", src.ChainID()),
		zap.String("dst_chain_id", dst.ChainID()),
	)

	createMsg, err := src.ChainProvider.CreateClient(clientState, dstUpdateHeader)
	if err != nil {
		return false, fmt.Errorf("failed to compose CreateClient msg for chain{%s}: %w", src.ChainID(), err)
	}

	msgs := []provider.RelayerMessage{createMsg}

	// if a matching client does not exist, create one
	var res *provider.RelayerTxResponse
	if err := retry.Do(func() error {
		var success bool
		var err error
		res, success, err = src.ChainProvider.SendMessages(ctx, msgs)
		if err != nil {
			src.LogFailedTx(res, err, msgs)
			return fmt.Errorf("failed to send messages on chain{%s}: %w", src.ChainID(), err)
		}

		if !success {
			src.LogFailedTx(res, nil, msgs)
			return fmt.Errorf("tx failed on chain{%s}: %s", src.ChainID(), res.Data)
		}

		return nil
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr); err != nil {
		return false, err
	}

	// update the client identifier
	// use index 0, the transaction only has one message
	if clientID, err = ParseClientIDFromEvents(res.Events); err != nil {
		return false, err
	}

	src.PathEnd.ClientID = clientID

	src.log.Info(
		"Client Created",
		zap.String("src_chain_id", src.ChainID()),
		zap.String("src_client_id", src.PathEnd.ClientID),
		zap.String("dst_chain_id", dst.ChainID()),
	)

	return true, nil
}

// UpdateClients updates clients for src on dst and dst on src given the configured paths
func (c *Chain) UpdateClients(ctx context.Context, dst *Chain) (err error) {
	var (
		srcUpdateHeader, dstUpdateHeader ibcexported.Header
		srch, dsth                       int64
	)

	if err = retry.Do(func() error {
		srch, dsth, err = QueryLatestHeights(ctx, c, dst)
		if err != nil {
			return err
		}
		return nil
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		c.log.Info(
			"Failed to get query latest heights when updating clients",
			zap.String("src_chain_id", c.ChainID()),
			zap.String("dst_chain_id", dst.ChainID()),
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", RtyAttNum),
			zap.Error(err),
		)
	})); err != nil {
		return err
	}

	if err = retry.Do(func() error {
		srcUpdateHeader, dstUpdateHeader, err = GetIBCUpdateHeaders(ctx, srch, dsth, c.ChainProvider, dst.ChainProvider, c.ClientID(), dst.ClientID())
		if err != nil {
			return err
		}
		return nil
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		c.log.Info(
			"Failed to get IBC update headers",
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", RtyAttNum),
			zap.Error(err),
		)
		srch, dsth, _ = QueryLatestHeights(ctx, c, dst)
	})); err != nil {
		return err
	}

	srcUpdateMsg, err := c.ChainProvider.UpdateClient(c.ClientID(), dstUpdateHeader)
	if err != nil {
		c.log.Debug(
			"Failed to update source client",
			zap.String("src_chain", c.ChainID()),
			zap.Error(err),
		)
		return err
	}

	dstUpdateMsg, err := dst.ChainProvider.UpdateClient(dst.ClientID(), srcUpdateHeader)
	if err != nil {
		dst.log.Debug(
			"Failed to update destination client",
			zap.String("dst_chain", dst.ChainID()),
			zap.Error(err),
		)
		return err
	}

	clients := &RelayMsgs{
		Src: []provider.RelayerMessage{srcUpdateMsg},
		Dst: []provider.RelayerMessage{dstUpdateMsg},
	}

	// Send msgs to both chains
	result := clients.Send(ctx, c.log, AsRelayMsgSender(c), AsRelayMsgSender(dst))
	if err := result.Error(); err != nil {
		if result.PartiallySent() {
			c.log.Info(
				"Partial success when updating clients",
				zap.String("src_chain_id", c.ChainID()),
				zap.String("dst_chain_id", dst.ChainID()),
				zap.Object("send_result", result),
			)
		}
		return err
	}

	c.log.Info(
		"Clients updated",
		zap.String("src_chain_id", c.ChainID()),
		zap.String("src_client", c.PathEnd.ClientID),
		zap.Stringer("src_height", MustGetHeight(srcUpdateHeader.GetHeight())),
		zap.Uint64("src_revision_height", srcUpdateHeader.GetHeight().GetRevisionHeight()),

		zap.String("dst_chain_id", dst.ChainID()),
		zap.String("dst_client", dst.PathEnd.ClientID),
		zap.Stringer("dst_height", MustGetHeight(dstUpdateHeader.GetHeight())),
		zap.Uint64("dst_revision_height", dstUpdateHeader.GetHeight().GetRevisionHeight()),
	)

	return nil
}

// UpgradeClients upgrades the client on src after dst chain has undergone an upgrade.
func (c *Chain) UpgradeClients(ctx context.Context, dst *Chain, height int64) error {
	dstHeader, err := dst.ChainProvider.GetLightSignedHeaderAtHeight(ctx, height)
	if err != nil {
		return err
	}

	// updates off-chain light client
	updateMsg, err := c.ChainProvider.UpdateClient(c.ClientID(), dstHeader)
	if err != nil {
		return err
	}

	if height == 0 {
		height, err = dst.ChainProvider.QueryLatestHeight(ctx)
		if err != nil {
			return err
		}
	}

	// query proofs on counterparty
	clientRes, err := dst.ChainProvider.QueryUpgradedClient(ctx, height)
	if err != nil {
		return err
	}

	consRes, err := dst.ChainProvider.QueryUpgradedConsState(ctx, height)
	if err != nil {
		return err
	}

	upgradeMsg, err := c.ChainProvider.MsgUpgradeClient(c.ClientID(), consRes, clientRes)
	if err != nil {
		return err
	}

	msgs := []provider.RelayerMessage{
		updateMsg,
		upgradeMsg,
	}

	res, _, err := c.ChainProvider.SendMessages(ctx, msgs)
	if err != nil {
		c.LogFailedTx(res, err, msgs)
		return err
	}

	return nil
}

// MustGetHeight takes the height inteface and returns the actual height
func MustGetHeight(h ibcexported.Height) clienttypes.Height {
	height, ok := h.(clienttypes.Height)
	if !ok {
		panic("height is not an instance of height!")
	}
	return height
}
