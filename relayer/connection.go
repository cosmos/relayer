package relayer

import (
	"fmt"
	"time"

	conntypes "github.com/cosmos/ibc-go/v2/modules/core/03-connection/types"
)

// CreateOpenConnections runs the connection creation messages on timeout until they pass.
// The returned boolean indicates that the path end has been modified.
func (c *Chain) CreateOpenConnections(dst *Chain, maxRetries uint64, to time.Duration) (modified bool, err error) {
	// client identifiers must be filled in
	if err := ValidateClientPaths(c, dst); err != nil {
		return modified, err
	}

	ticker := time.NewTicker(to)
	failed := uint64(0)
	for ; true; <-ticker.C {
		success, lastStep, recentlyModified, err := ExecuteConnectionStep(c, dst)
		if err != nil {
			c.Log(fmt.Sprintf("%v", err))
		}

		if recentlyModified {
			modified = true
		}

		switch {
		// In the case of success and this being the last transaction
		// debug logging, log created connection and break
		case success && lastStep:
			if c.debug {
				srcH, dstH, err := QueryLatestHeights(c, dst)
				if err != nil {
					return modified, err
				}
				srcConn, dstConn, err := QueryConnectionPair(c, dst, srcH, dstH)
				if err != nil {
					return modified, err
				}
				logConnectionStates(c, dst, srcConn, dstConn)
			}

			c.Log(fmt.Sprintf("★ Connection created: [%s]client{%s}conn{%s} -> [%s]client{%s}conn{%s}",
				c.ChainID(), c.ClientID(), c.ConnectionID(),
				dst.ChainID(), dst.ClientID(), dst.ConnectionID()))
			return modified, nil

		// reset the failures counter
		case success:
			failed = 0
			continue

		// increment the failures counter and exit if we used all retry attempts
		case !success:
			failed++
			c.Log("retrying transaction...")
			time.Sleep(5 * time.Second)

			if failed > maxRetries {
				return modified, fmt.Errorf("! Connection failed: [%s]client{%s}conn{%s} -> [%s]client{%s}conn{%s}",
					c.ChainID(), c.ClientID(), c.ConnectionID(),
					dst.ChainID(), dst.ClientID(), dst.ConnectionID())
			}
		}
	}

	return modified, nil // lgtm [go/unreachable-statement]
}

// ExecuteConnectionStep executes the next connection step based on the
// states of two connection ends specified by the relayer configuration
// file. The booleans return indicate if the message was successfully
// executed and if this was the last handshake step.
func ExecuteConnectionStep(src, dst *Chain) (success, last, modified bool, err error) {
	srch, dsth, err := QueryLatestHeights(src, dst)
	if err != nil {
		return false, false, false, err
	}

	srcHeader, err := src.ChainProvider.GetIBCUpdateHeader(srch, dst.ChainProvider, dst.ClientID())
	if err != nil {
		return false, false, false, err
	}

	dstHeader, err := dst.ChainProvider.GetIBCUpdateHeader(dsth, src.ChainProvider, src.ClientID())
	if err != nil {
		return false, false, false, err
	}

	// TODO: add back retries due to commit delay/update
	// get headers to update light clients on chain
	// if either identifier is missing, an existing connection that matches the required fields
	// is chosen or a new connection is created.
	// This will perform either an OpenInit or OpenTry step and return
	if src.ConnectionID() == "" || dst.ConnectionID() == "" {
		success, modified, err := InitializeConnection(src, dst)
		if err != nil {
			return false, false, false, err
		}

		return success, false, modified, nil
	}

	// Query Connection data from src and dst
	srcConn, dstConn, err := QueryConnectionPair(src, dst, srch-1, dsth-1)
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

		msgs, err := src.ChainProvider.ConnectionOpenTry(dst.ChainProvider, dstHeader, src.ClientID(), dst.ClientID(), src.ConnectionID(), dst.ConnectionID())
		if err != nil {
			return false, false, false, err
		}

		res, success, err := src.ChainProvider.SendMessages(msgs)
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

		msgs, err := src.ChainProvider.ConnectionOpenAck(dst.ChainProvider, dstHeader, src.ClientID(), src.ConnectionID(), dst.ClientID(), dst.ConnectionID())
		if err != nil {
			return false, false, false, err
		}

		res, success, err := src.ChainProvider.SendMessages(msgs)
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

		msgs, err := dst.ChainProvider.ConnectionOpenAck(src.ChainProvider, srcHeader, dst.ClientID(), dst.ConnectionID(), src.ClientID(), src.ConnectionID())
		if err != nil {
			return false, false, false, err
		}

		res, success, err := dst.ChainProvider.SendMessages(msgs)
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

		msgs, err := src.ChainProvider.ConnectionOpenConfirm(dst.ChainProvider, dstHeader, dst.ConnectionID(), src.ClientID(), src.ConnectionID())
		if err != nil {
			return false, false, false, err
		}

		res, success, err := src.ChainProvider.SendMessages(msgs)
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

		msgs, err := dst.ChainProvider.ConnectionOpenConfirm(src.ChainProvider, srcHeader, src.ConnectionID(), dst.ClientID(), dst.ConnectionID())
		if err != nil {
			return false, false, false, err
		}

		res, success, err := dst.ChainProvider.SendMessages(msgs)
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
func InitializeConnection(src, dst *Chain) (success, modified bool, err error) {
	srch, dsth, err := QueryLatestHeights(src, dst)
	if err != nil {
		return false, false, err
	}

	srcHeader, err := src.ChainProvider.GetIBCUpdateHeader(srch, dst.ChainProvider, dst.ClientID())
	if err != nil {
		return false, false, err
	}

	dstHeader, err := dst.ChainProvider.GetIBCUpdateHeader(dsth, src.ChainProvider, src.ClientID())
	if err != nil {
		return false, false, err
	}

	switch {

	// OpenInit on source
	// Neither connection has been initialized
	case src.PathEnd.ConnectionID == "" && dst.PathEnd.ConnectionID == "":
		if src.debug {
			src.logOpenInit(dst, "connection")
		}

		connectionID, found := FindMatchingConnection(src, dst)
		if !found {
			// construct OpenInit message to be submitted on source chain
			msgs, err := src.ChainProvider.ConnectionOpenInit(src.ClientID(), dst.ClientID(), dstHeader)
			if err != nil {
				return false, false, err
			}

			res, success, err := src.ChainProvider.SendMessages(msgs)
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
		} else if src.debug {
			src.logIdentifierExists(dst, "connection end", connectionID)
		}

		src.PathEnd.ConnectionID = connectionID

		return true, true, nil

	// OpenTry on source
	// source connection does not exist, but counterparty connection exists
	case src.PathEnd.ConnectionID == "" && dst.PathEnd.ConnectionID != "":
		if src.debug {
			src.logOpenTry(dst, "connection")
		}

		connectionID, found := FindMatchingConnection(src, dst)
		if !found {
			msgs, err := src.ChainProvider.ConnectionOpenTry(dst.ChainProvider, dstHeader, src.ClientID(), dst.ClientID(), src.ConnectionID(), dst.ConnectionID())
			if err != nil {
				return false, false, err
			}

			res, success, err := src.ChainProvider.SendMessages(msgs)
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
		} else if src.debug {
			src.logIdentifierExists(dst, "connection end", connectionID)
		}

		src.PathEnd.ConnectionID = connectionID

		return true, true, nil

	// OpenTry on counterparty
	// source connection exists, but counterparty connection does not exist
	case src.PathEnd.ConnectionID != "" && dst.PathEnd.ConnectionID == "":
		if dst.debug {
			dst.logOpenTry(src, "connection")
		}

		connectionID, found := FindMatchingConnection(dst, src)
		if !found {
			msgs, err := dst.ChainProvider.ConnectionOpenTry(src.ChainProvider, srcHeader, dst.ClientID(), src.ClientID(), dst.ConnectionID(), src.ConnectionID())
			if err != nil {
				return false, false, err
			}

			res, success, err := dst.ChainProvider.SendMessages(msgs)
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
		} else if dst.debug {
			dst.logIdentifierExists(src, "connection end", connectionID)
		}

		dst.PathEnd.ConnectionID = connectionID

		return true, true, nil

	default:
		return false, true, fmt.Errorf("connection ends already created")
	}
}

// FindMatchingConnection will determine if there already exists a connection between source and counterparty
// that matches the parameters set in the relayer config.
func FindMatchingConnection(source, counterparty *Chain) (string, bool) {
	// TODO: add appropriate offset and limits, along with retries
	connectionsResp, err := source.ChainProvider.QueryConnections()
	if err != nil {
		if source.debug {
			source.Log(fmt.Sprintf("Error: querying connections on %s failed: %v", source.ChainID(), err))
		}
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
