package relayer

import (
	"context"
	"fmt"
	"time"

	"github.com/avast/retry-go/v4"
	clienttypes "github.com/cosmos/ibc-go/v4/modules/core/02-client/types"
	ibcexported "github.com/cosmos/ibc-go/v4/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// CreateClients creates clients for src on dst and dst on src if the client ids are unspecified.
func (c *Chain) CreateClients(ctx context.Context, dst *Chain, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour, override bool, customClientTrustingPeriod time.Duration, memo string) (bool, error) {
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
	var srcUpdateHeader, dstUpdateHeader provider.IBCHeader
	if err := retry.Do(func() error {
		var err error
		srcUpdateHeader, dstUpdateHeader, err = QueryIBCHeaders(ctx, c, dst, srch, dsth)
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
		modifiedSrc, err = CreateClient(egCtx, c, dst, srcUpdateHeader, dstUpdateHeader, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour, override, customClientTrustingPeriod, memo)
		if err != nil {
			return fmt.Errorf("failed to create client on src chain{%s}: %w", c.ChainID(), err)
		}
		return nil
	})

	eg.Go(func() error {
		var err error
		// Create client on dst for src if the client id is unspecified
		modifiedDst, err = CreateClient(egCtx, dst, c, dstUpdateHeader, srcUpdateHeader, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour, override, customClientTrustingPeriod, memo)
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

func CreateClient(ctx context.Context, src, dst *Chain, srcUpdateHeader, dstUpdateHeader provider.IBCHeader, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour, override bool, customClientTrustingPeriod time.Duration, memo string) (bool, error) {
	// If a client ID was specified in the path, ensure it exists.
	if src.PathEnd.ClientID != "" {
		// TODO: check client is not expired
		_, err := src.ChainProvider.QueryClientStateResponse(ctx, int64(srcUpdateHeader.Height()), src.ClientID())
		if err != nil {
			return false, fmt.Errorf("please ensure provided on-chain client (%s) exists on the chain (%s): %v",
				src.PathEnd.ClientID, src.ChainID(), err)
		}

		return false, nil
	}

	// Otherwise, create client for the destination chain on the source chain.

	// Query the trusting period for dst and retry if the query fails
	// var tp time.Duration
	tp := customClientTrustingPeriod
	if tp == 0 {
		if err := retry.Do(func() error {
			var err error
			tp, err = dst.GetTrustingPeriod(ctx)
			if err != nil {
				return fmt.Errorf("failed to get trusting period for chain{%s}: %w", dst.ChainID(), err)
			}
			if tp == 0 {
				return retry.Unrecoverable(fmt.Errorf("chain %s reported invalid zero trusting period", dst.ChainID()))
			}
			return nil
		}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr); err != nil {
			return false, err
		}
	}

	src.log.Debug(
		"Creating client",
		zap.String("src_chain_id", src.ChainID()),
		zap.String("dst_chain_id", dst.ChainID()),
		zap.Uint64("dst_header_height", dstUpdateHeader.Height()),
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

	// We want to create a light client on the src chain which tracks the state of the dst chain.
	// So we build a new client state from dst and attempt to use this for creating the light client on src.
	clientState, err := dst.ChainProvider.NewClientState(dst.ChainID(), dstUpdateHeader, tp, ubdPeriod, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour)
	if err != nil {
		return false, fmt.Errorf("failed to create new client state for chain{%s}: %w", dst.ChainID(), err)
	}

	var clientID string

	// Will not reuse same client if override is true
	if !override {
		// Check if an identical light client already exists on the src chain which matches the
		// proposed new client state from dst.
		clientID, err = findMatchingClient(ctx, src, dst, clientState)
		if err != nil {
			return false, fmt.Errorf("failed to find a matching client for the new client state: %w", err)
		}
	}

	if clientID != "" && !override {
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

	// We need to retrieve the address of the src chain account because we want to use
	// the dst chains implementation of CreateClient, to ensure the proper client/header
	// logic is executed, but the message gets submitted on the src chain which means
	// we need to sign with the address from src.

	createMsg, err := src.ChainProvider.MsgCreateClient(clientState, dstUpdateHeader.ConsensusState())
	if err != nil {
		return false, fmt.Errorf("failed to compose CreateClient msg for chain{%s} tracking the state of chain{%s}: %w",
			src.ChainID(), dst.ChainID(), err)
	}

	msgs := []provider.RelayerMessage{createMsg}

	// if a matching client does not exist, create one
	var res *provider.RelayerTxResponse
	if err := retry.Do(func() error {
		var success bool
		var err error
		res, success, err = src.ChainProvider.SendMessages(ctx, msgs, memo)
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
	if clientID, err = parseClientIDFromEvents(res.Events); err != nil {
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

// MsgUpdateClient queries for the current client state on dst,
// then queries for the latest and trusted headers on src
// in order to build a MsgUpdateClient message for dst.
func MsgUpdateClient(
	ctx context.Context,
	src, dst *Chain,
	srch, dsth int64,
) (provider.RelayerMessage, error) {
	var dstClientState ibcexported.ClientState
	if err := retry.Do(func() error {
		var err error
		dstClientState, err = dst.ChainProvider.QueryClientState(ctx, dsth, dst.ClientID())
		return err
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		dst.log.Info(
			"Failed to query client state when updating clients",
			zap.String("client_id", dst.ClientID()),
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", RtyAttNum),
			zap.Error(err),
		)
	})); err != nil {
		return nil, err
	}

	var srcHeader, dstTrustedHeader provider.IBCHeader

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return retry.Do(func() error {
			var err error
			srcHeader, err = src.ChainProvider.QueryIBCHeader(egCtx, srch)
			return err
		}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
			src.log.Info(
				"Failed to query IBC header when building update client message",
				zap.String("client_id", dst.ClientID()),
				zap.Uint("attempt", n+1),
				zap.Uint("max_attempts", RtyAttNum),
				zap.Error(err),
			)
		}))
	})
	eg.Go(func() error {
		return retry.Do(func() error {
			var err error
			dstTrustedHeader, err = src.ChainProvider.QueryIBCHeader(egCtx, int64(dstClientState.GetLatestHeight().GetRevisionHeight())+1)
			return err
		}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
			src.log.Info(
				"Failed to query IBC header when building update client message",
				zap.String("client_id", dst.ClientID()),
				zap.Uint("attempt", n+1),
				zap.Uint("max_attempts", RtyAttNum),
				zap.Error(err),
			)
		}))
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	var updateHeader ibcexported.Header
	if err := retry.Do(func() error {
		var err error
		updateHeader, err = src.ChainProvider.MsgUpdateClientHeader(srcHeader, dstClientState.GetLatestHeight().(clienttypes.Height), dstTrustedHeader)
		return err
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		src.log.Info(
			"Failed to build update client header",
			zap.String("client_id", dst.ClientID()),
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", RtyAttNum),
			zap.Error(err),
		)
	})); err != nil {
		return nil, err
	}

	// updates off-chain light client
	return dst.ChainProvider.MsgUpdateClient(dst.ClientID(), updateHeader)
}

// UpdateClients updates clients for src on dst and dst on src given the configured paths
func UpdateClients(
	ctx context.Context,
	src, dst *Chain,
	memo string,
) (err error) {
	srch, dsth, err := QueryLatestHeights(ctx, src, dst)
	if err != nil {
		return err
	}

	var srcMsgUpdateClient, dstMsgUpdateClient provider.RelayerMessage
	eg, egCtx := errgroup.WithContext(ctx) // New errgroup because previous egCtx is canceled at this point.
	eg.Go(func() error {
		var err error
		srcMsgUpdateClient, err = MsgUpdateClient(egCtx, dst, src, dsth, srch)
		return err
	})
	eg.Go(func() error {
		var err error
		dstMsgUpdateClient, err = MsgUpdateClient(egCtx, src, dst, srch, dsth)
		return err
	})

	if err = eg.Wait(); err != nil {
		return err
	}

	clients := &RelayMsgs{
		Src: []provider.RelayerMessage{srcMsgUpdateClient},
		Dst: []provider.RelayerMessage{dstMsgUpdateClient},
	}

	// Send msgs to both chains
	result := clients.Send(ctx, src.log, AsRelayMsgSender(src), AsRelayMsgSender(dst), memo)
	if err := result.Error(); err != nil {
		if result.PartiallySent() {
			src.log.Info(
				"Partial success when updating clients",
				zap.String("src_chain_id", src.ChainID()),
				zap.String("dst_chain_id", dst.ChainID()),
				zap.Object("send_result", result),
			)
		}
		return err
	}

	src.log.Info(
		"Clients updated",
		zap.String("src_chain_id", src.ChainID()),
		zap.String("src_client", src.PathEnd.ClientID),

		zap.String("dst_chain_id", dst.ChainID()),
		zap.String("dst_client", dst.PathEnd.ClientID),
	)

	return nil
}

// UpgradeClients upgrades the client on dst after src chain has undergone an upgrade.
func UpgradeClient(
	ctx context.Context,
	src, dst *Chain,
	height int64,
	memo string,
) (err error) {
	srch, dsth, err := QueryLatestHeights(ctx, src, dst)
	if err != nil {
		return err
	}

	if height != 0 {
		srch = height
	}

	updateMsg, err := MsgUpdateClient(ctx, src, dst, srch, dsth)
	if err != nil {
		return err
	}

	// query proofs on counterparty
	clientRes, err := src.ChainProvider.QueryUpgradedClient(ctx, srch)
	if err != nil {
		return err
	}

	consRes, err := src.ChainProvider.QueryUpgradedConsState(ctx, srch)
	if err != nil {
		return err
	}

	upgradeMsg, err := dst.ChainProvider.MsgUpgradeClient(dst.ClientID(), consRes, clientRes)
	if err != nil {
		return err
	}

	msgs := []provider.RelayerMessage{
		updateMsg,
		upgradeMsg,
	}

	res, _, err := dst.ChainProvider.SendMessages(ctx, msgs, memo)
	if err != nil {
		dst.LogFailedTx(res, err, msgs)
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

// findMatchingClient is a helper function that will determine if there exists a client with identical client and
// consensus states to the client which would have been created. Source is the chain that would be adding a client
// which would track the counterparty. Therefore, we query source for the existing clients
// and check if any match the counterparty. The counterparty must have a matching consensus state
// to the latest consensus state of a potential match. The provided client state is the client
// state that will be created if there exist no matches.
func findMatchingClient(ctx context.Context, src, dst *Chain, newClientState ibcexported.ClientState) (string, error) {
	var (
		clientsResp clienttypes.IdentifiedClientStates
		err         error
	)

	if err = retry.Do(func() error {
		clientsResp, err = src.ChainProvider.QueryClients(ctx)
		if err != nil {
			return err
		}
		return nil
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		src.log.Info(
			"Failed to query clients",
			zap.String("chain_id", src.ChainID()),
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", RtyAttNum),
			zap.Error(err),
		)
	})); err != nil {
		return "", err
	}

	for _, existingClientState := range clientsResp {
		clientID, err := provider.ClientsMatch(ctx, src.ChainProvider, dst.ChainProvider, existingClientState, newClientState)

		// If there is an error parsing/type asserting the client state in ClientsMatch this is going
		// to make the entire find matching client logic fail.
		// We should really never be encountering an error here and if we do it is probably a sign of a
		// larger scale problem at hand.
		if err != nil {
			return "", err
		}
		if clientID != "" {
			return clientID, nil
		}
	}

	return "", nil
}

// parseClientIDFromEvents parses events emitted from a MsgCreateClient and returns the
// client identifier.
func parseClientIDFromEvents(events []provider.RelayerEvent) (string, error) {
	for _, event := range events {
		if event.EventType == clienttypes.EventTypeCreateClient {
			for attributeKey, attributeValue := range event.Attributes {
				if attributeKey == clienttypes.AttributeKeyClientID {
					return attributeValue, nil
				}
			}
		}
	}
	return "", fmt.Errorf("client identifier event attribute not found")
}
