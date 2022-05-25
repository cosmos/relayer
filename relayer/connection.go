package relayer

import (
	"context"
	"fmt"
	"time"

	"github.com/avast/retry-go/v4"
	conntypes "github.com/cosmos/ibc-go/v3/modules/core/03-connection/types"
	"github.com/cosmos/ibc-go/v3/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

// CreateOpenConnections runs the connection creation messages on timeout until they pass.
// The returned boolean indicates that the path end has been modified.
func (c *Chain) CreateOpenConnections(
	ctx context.Context,
	dst *Chain,
	maxRetries uint64,
	timeout time.Duration,
) (modified bool, err error) {
	// client identifiers must be filled in
	if err = ValidateClientPaths(c, dst); err != nil {
		return modified, err
	}

	ticker := time.NewTicker(timeout)
	defer ticker.Stop()

	// Populate the immediate channel so we begin processing right away.
	immediate := make(chan struct{}, 1)
	immediate <- struct{}{}

	var failed uint64
	for {
		// Block until the immediate signal or the ticker fires.
		select {
		case <-immediate:
			// Keep going.
		case <-ticker.C:
			// Keep going.
		case <-ctx.Done():
			return modified, ctx.Err()
		}
		success, lastStep, recentlyModified, err := ExecuteConnectionStep(ctx, c, dst)
		if err != nil {
			c.log.Info("Error executing connection step", zap.Error(err))
		}

		if recentlyModified {
			modified = true
		}

		switch {
		// In the case of success and this being the last transaction
		// debug logging, log created connection and break
		case success && lastStep:
			if c.debug {
				srcH, dstH, err := QueryLatestHeights(ctx, c, dst)
				if err != nil {
					return modified, err
				}
				srcConn, dstConn, err := QueryConnectionPair(ctx, c, dst, srcH, dstH)
				if err != nil {
					return modified, err
				}
				logConnectionStates(c, dst, srcConn, dstConn)
			}

			c.log.Info(
				"Connection created",
				zap.String("src_chain_id", c.ChainID()),
				zap.String("src_client_id", c.ClientID()),
				zap.String("src_connection_id", c.ConnectionID()),
				zap.String("dst_chain_id", dst.ChainID()),
				zap.String("dst_client_id", dst.ClientID()),
				zap.String("dst_connection_id", dst.ConnectionID()),
			)
			return modified, nil

		// reset the failures counter
		case success:
			failed = 0

			if !recentlyModified {
				c.log.Debug("Short delay before retrying connection open transaction, because the last check was a no-op...")
				select {
				case <-time.After(timeout / 8):
					// Nothing to do.
				case <-ctx.Done():
					return false, ctx.Err()
				}
			}

			select {
			case immediate <- struct{}{}:
				// Proceed immediately to the next step if possible.
			default:
				// If can't write to ch -- could that ever happen? -- that's fine, don't block here.
			}

			continue

		// increment the failures counter and exit if we used all retry attempts
		case !success:
			failed++
			if failed > maxRetries {
				return modified, fmt.Errorf("! Connection failed: [%s]client{%s}conn{%s} -> [%s]client{%s}conn{%s}",
					c.ChainID(), c.ClientID(), c.ConnectionID(),
					dst.ChainID(), dst.ClientID(), dst.ConnectionID())
			}

			c.log.Debug("Delaying before retrying connection open transaction...")
			select {
			case <-time.After(5 * time.Second):
				// Nothing to do.
			case <-ctx.Done():
				return modified, ctx.Err()
			}
		}
	}

	panic("unreachable")
}

// ExecuteConnectionStep executes the next connection step based on the
// states of two connection ends specified by the relayer configuration
// file. The booleans return indicate if the message was successfully
// executed and if this was the last handshake step.
func ExecuteConnectionStep(ctx context.Context, src, dst *Chain) (success, last, modified bool, err error) {
	var (
		msgs                 []provider.RelayerMessage
		res                  *provider.RelayerTxResponse
		srcHeader, dstHeader exported.ClientMessage
		srch, dsth           int64
	)

	// Query the latest heights on src and dst and retry if the query fails
	if err = retry.Do(func() error {
		srch, dsth, err = QueryLatestHeights(ctx, src, dst)
		if err != nil || srch == 0 || dsth == 0 {
			return fmt.Errorf("failed to query latest heights. Err: %w", err)
		}
		return err
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr); err != nil {
		return success, last, modified, err
	}

	// get headers to update light clients on chain
	// if either identifier is missing, an existing connection that matches the required fields
	// is chosen or a new connection is created.
	// This will perform either an OpenInit or OpenTry step and return
	if err = retry.Do(func() error {
		srcHeader, dstHeader, err = GetIBCUpdateHeaders(ctx, srch, dsth, src.ChainProvider, dst.ChainProvider, src.ClientID(), dst.ClientID())
		return err
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		src.log.Info(
			"Failed to get IBC update headers",
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", RtyAttNum),
			zap.Error(err),
		)
		srch, dsth, _ = QueryLatestHeights(ctx, src, dst)
	})); err != nil {
		return success, last, modified, err
	}

	if src.ConnectionID() == "" || dst.ConnectionID() == "" {
		src.log.Debug(
			"Initializing connection",
			zap.String("src_chain_id", src.ChainID()),
			zap.String("src_client_id", src.ClientID()),
			zap.String("dst_chain_id", dst.ChainID()),
			zap.String("dst_client_id", dst.ClientID()),
		)

		success, modified, err = InitializeConnection(ctx, src, dst)
		if err != nil {
			return false, false, false, err
		}

		return success, last, modified, nil
	}

	// Query Connection data from src and dst
	srcConn, dstConn, err := QueryConnectionPair(ctx, src, dst, srch-1, dsth-1)
	if err != nil {
		return false, false, false, err
	}

	switch {

	// OpenTry on source if both connections are at INIT (crossing hellos)
	// obtain proof of counterparty in INIT state and submit to source chain to update state
	// from INIT to TRYOPEN.
	case srcConn.Connection.State == conntypes.INIT && dstConn.Connection.State == conntypes.INIT:
		if src.debug {
			logConnectionStates(src, dst, srcConn, dstConn)
		}

		msgs, err = src.ChainProvider.ConnectionOpenTry(ctx, dst.ChainProvider, dstHeader, src.ClientID(), dst.ClientID(), src.ConnectionID(), dst.ConnectionID())
		if err != nil {
			return false, false, false, err
		}

		res, success, err = src.ChainProvider.SendMessages(ctx, msgs)
		if err != nil {
			src.LogFailedTx(res, err, msgs)
		}
		if !success {
			return false, false, false, err
		}

	// OpenAck on source if dst is at TRYOPEN and src is on INIT or TRYOPEN (crossing hellos case)
	// obtain proof of counterparty in TRYOPEN state and submit to source chain to update state
	// from INIT/TRYOPEN to OPEN.
	case (srcConn.Connection.State == conntypes.INIT || srcConn.Connection.State == conntypes.TRYOPEN) &&
		dstConn.Connection.State == conntypes.TRYOPEN:
		if src.debug {
			logConnectionStates(src, dst, srcConn, dstConn)
		}

		msgs, err = src.ChainProvider.ConnectionOpenAck(ctx, dst.ChainProvider, dstHeader, src.ClientID(), src.ConnectionID(), dst.ClientID(), dst.ConnectionID())
		if err != nil {
			return false, false, false, err
		}

		res, success, err = src.ChainProvider.SendMessages(ctx, msgs)
		if err != nil {
			src.LogFailedTx(res, err, msgs)
		}
		if !success {
			return false, false, false, err
		}

	// OpenAck on counterparty
	// obtain proof of source in TRYOPEN state and submit to counterparty chain to update state
	// from INIT to OPEN.
	case srcConn.Connection.State == conntypes.TRYOPEN && dstConn.Connection.State == conntypes.INIT:
		if dst.debug {
			logConnectionStates(dst, src, dstConn, srcConn)
		}

		msgs, err = dst.ChainProvider.ConnectionOpenAck(ctx, src.ChainProvider, srcHeader, dst.ClientID(), dst.ConnectionID(), src.ClientID(), src.ConnectionID())
		if err != nil {
			return false, false, false, err
		}

		res, success, err = dst.ChainProvider.SendMessages(ctx, msgs)
		if err != nil {
			dst.LogFailedTx(res, err, msgs)
		}
		if !success {
			return false, false, false, err
		}

	// OpenConfirm on source
	case srcConn.Connection.State == conntypes.TRYOPEN && dstConn.Connection.State == conntypes.OPEN:
		if src.debug {
			logConnectionStates(src, dst, srcConn, dstConn)
		}

		msgs, err = src.ChainProvider.ConnectionOpenConfirm(ctx, dst.ChainProvider, dstHeader, dst.ConnectionID(), src.ClientID(), src.ConnectionID())
		if err != nil {
			return false, false, false, err
		}

		res, success, err = src.ChainProvider.SendMessages(ctx, msgs)
		if err != nil {
			src.LogFailedTx(res, err, msgs)
		}
		if !success {
			return false, false, false, err
		}

		last = true

	// ,OpenConfirm on counterparty
	case srcConn.Connection.State == conntypes.OPEN && dstConn.Connection.State == conntypes.TRYOPEN:
		if dst.debug {
			logConnectionStates(dst, src, dstConn, srcConn)
		}

		msgs, err = dst.ChainProvider.ConnectionOpenConfirm(ctx, src.ChainProvider, srcHeader, src.ConnectionID(), dst.ClientID(), dst.ConnectionID())
		if err != nil {
			return false, false, false, err
		}

		res, success, err = dst.ChainProvider.SendMessages(ctx, msgs)
		if err != nil {
			dst.LogFailedTx(res, err, msgs)
		}
		if !success {
			return false, false, false, err
		}

		last = true

	case srcConn.Connection.State == conntypes.OPEN && dstConn.Connection.State == conntypes.OPEN:
		last = true

	}

	return true, last, false, nil
}

// InitializeConnection creates a new connection on either the source or destination chain .
// The identifiers set in the PathEnd's are used to determine which connection ends need to be
// initialized. The PathEnds are updated upon a successful transaction.
// NOTE: This function may need to be called twice if neither connection exists.
func InitializeConnection(ctx context.Context, src, dst *Chain) (success, modified bool, err error) {
	var (
		srcHeader, dstHeader exported.ClientMessage
		srch, dsth           int64
		msgs                 []provider.RelayerMessage
		res                  *provider.RelayerTxResponse
	)

	// Query the latest heights on src and dst and retry if the query fails
	if err = retry.Do(func() error {
		srch, dsth, err = QueryLatestHeights(ctx, src, dst)
		if srch == 0 || dsth == 0 || err != nil {
			return fmt.Errorf("failed to query latest heights. Err: %w", err)
		}
		return err
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr); err != nil {
		return false, false, err
	}

	// Get IBC Update Headers for src and dst which can be used to update an on chain light client on the counterparty
	if err = retry.Do(func() error {
		srcHeader, dstHeader, err = GetIBCUpdateHeaders(ctx, srch, dsth, src.ChainProvider, dst.ChainProvider, src.ClientID(), dst.ClientID())
		return err
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		src.log.Info(
			"Failed to get IBC update headers",
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", RtyAttNum),
			zap.Error(err),
		)
		srch, dsth, _ = QueryLatestHeights(ctx, src, dst)
	})); err != nil {
		return false, false, err
	}

	switch {

	// OpenInit on source
	// Neither connection has been initialized
	case src.PathEnd.ConnectionID == "" && dst.PathEnd.ConnectionID == "":
		src.log.Debug(
			"Attempting to create new connection ends",
			zap.String("src_chain_id", src.ChainID()),
			zap.String("dst_chain_id", dst.ChainID()),
		)

		connectionID, found := FindMatchingConnection(ctx, src, dst)
		if !found {
			// construct OpenInit message to be submitted on source chain
			msgs, err = src.ChainProvider.ConnectionOpenInit(src.ClientID(), dst.ClientID(), dstHeader)
			if err != nil {
				return false, false, err
			}

			res, success, err = src.ChainProvider.SendMessages(ctx, msgs)
			if err != nil {
				src.LogFailedTx(res, err, msgs)
			}
			if !success {
				return false, false, err
			}

			// update connection identifier in PathEnd
			// use index 1, connection open init is the second message in the transaction
			connectionID, err = ParseConnectionIDFromEvents(res.Events)
			if err != nil {
				return false, false, err
			}
		} else {
			src.log.Debug(
				"Connection end already exists",
				zap.String("conn_id", connectionID),
				zap.String("src_chain_id", src.ChainID()),
				zap.String("dst_chain_id", dst.ChainID()),
			)
		}

		src.PathEnd.ConnectionID = connectionID

		return true, true, nil

	// OpenTry on source
	// source connection does not exist, but counterparty connection exists
	case src.PathEnd.ConnectionID == "" && dst.PathEnd.ConnectionID != "":
		src.log.Debug(
			"Attempting to open connection end",
			zap.String("src_chain_id", src.ChainID()),
			zap.String("dst_chain_id", dst.ChainID()),
		)

		connectionID, found := FindMatchingConnection(ctx, src, dst)
		if !found {
			msgs, err = src.ChainProvider.ConnectionOpenTry(ctx, dst.ChainProvider, dstHeader, src.ClientID(), dst.ClientID(), src.ConnectionID(), dst.ConnectionID())
			if err != nil {
				return false, false, err
			}

			res, success, err = src.ChainProvider.SendMessages(ctx, msgs)
			if err != nil {
				src.LogFailedTx(res, err, msgs)
			}
			if !success {
				return false, false, err
			}

			// update connection identifier in PathEnd
			// use index 1, connection open try is the second message in the transaction
			connectionID, err = ParseConnectionIDFromEvents(res.Events)
			if err != nil {
				return false, false, err
			}
		} else {
			src.log.Debug(
				"Connection end already exists",
				zap.String("conn_id", connectionID),
				zap.String("src_chain_id", src.ChainID()),
				zap.String("dst_chain_id", dst.ChainID()),
			)
		}

		src.PathEnd.ConnectionID = connectionID

		return true, true, nil

	// OpenTry on counterparty
	// source connection exists, but counterparty connection does not exist
	case src.PathEnd.ConnectionID != "" && dst.PathEnd.ConnectionID == "":
		dst.log.Debug(
			"Attempting to open connection end",
			zap.String("src_chain_id", dst.ChainID()),
			zap.String("dst_chain_id", src.ChainID()),
		)

		connectionID, found := FindMatchingConnection(ctx, dst, src)
		if !found {
			msgs, err = dst.ChainProvider.ConnectionOpenTry(ctx, src.ChainProvider, srcHeader, dst.ClientID(), src.ClientID(), dst.ConnectionID(), src.ConnectionID())
			if err != nil {
				return false, false, err
			}

			res, success, err = dst.ChainProvider.SendMessages(ctx, msgs)
			if err != nil {
				dst.LogFailedTx(res, err, msgs)
			}
			if !success {
				return false, false, err
			}

			// update connection identifier in PathEnd
			// use index 1, connection open try is the second message in the transaction
			connectionID, err = ParseConnectionIDFromEvents(res.Events)
			if err != nil {
				return false, false, err
			}
		} else {
			dst.log.Debug(
				"Connection end already exists",
				zap.String("conn_id", connectionID),
				zap.String("src_chain_id", dst.ChainID()),
				zap.String("dst_chain_id", src.ChainID()),
			)
		}

		dst.PathEnd.ConnectionID = connectionID

		return true, true, nil

	default:
		return false, true, fmt.Errorf("connection ends already created")
	}
}

// FindMatchingConnection will determine if there already exists a connection between source and counterparty
// that matches the parameters set in the relayer config.
func FindMatchingConnection(ctx context.Context, source, counterparty *Chain) (string, bool) {
	// TODO: add appropriate offset and limits
	var (
		err             error
		connectionsResp []*conntypes.IdentifiedConnection
	)

	if err = retry.Do(func() error {
		connectionsResp, err = source.ChainProvider.QueryConnections(ctx)
		if err != nil {
			return err
		}

		return nil
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		source.log.Info(
			"Querying connections failed",
			zap.String("src_chain_id", source.ChainID()),
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", RtyAttNum),
			zap.Error(err),
		)
	})); err != nil {
		return "", false
	}

	for _, connection := range connectionsResp {
		if IsMatchingConnection(source, counterparty, connection) {
			// unused connection found
			return connection.Id, true
		}
	}

	return "", false
}

// IsMatchingConnection determines if given connection matches required conditions
func IsMatchingConnection(source, counterparty *Chain, connection *conntypes.IdentifiedConnection) bool {
	// determines version we use is matching with given versions
	_, isVersionMatched := conntypes.FindSupportedVersion(conntypes.DefaultIBCVersion,
		conntypes.ProtoVersionsToExported(connection.Versions))
	return connection.ClientId == source.PathEnd.ClientID &&
		connection.Counterparty.ClientId == counterparty.PathEnd.ClientID &&
		isVersionMatched && connection.DelayPeriod == defaultDelayPeriod &&
		connection.Counterparty.Prefix.String() == defaultChainPrefix.String() &&
		(((connection.State == conntypes.INIT || connection.State == conntypes.TRYOPEN) &&
			connection.Counterparty.ConnectionId == "") ||
			(connection.State == conntypes.OPEN && (counterparty.PathEnd.ConnectionID == "" ||
				connection.Counterparty.ConnectionId == counterparty.PathEnd.ConnectionID)))
}
