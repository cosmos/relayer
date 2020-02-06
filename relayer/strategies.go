package relayer

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	connState "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/exported"
	connTypes "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/types"
	chanState "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/exported"
	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
	xferTypes "github.com/cosmos/cosmos-sdk/x/ibc/20-transfer/types"
)

// TODO: Figure out a better way to deal with these
const (
	ibcversion = "1.0.0"
	portID     = "bankbankbank"
)

// Strategy determines which relayer strategy to use
// NOTE: To make a strategy available via config you need to add it to
// this switch statement
func Strategy(name string) RelayStrategy {
	switch name {
	case "naive":
		return NaiveRelayStrategy
	default:
		return nil
	}
}

// RelayStrategy describes the function signature for a relay strategy
type RelayStrategy func(src, dst *Chain) (*RelayMsgs, error)

// RelayMsgs contains the msgs that need to be sent to both a src and dst chain
// after a given relay round
type RelayMsgs struct {
	Src []sdk.Msg
	Dst []sdk.Msg
}

// NaiveRelayStrategy returns the RelayMsgs that need to be run to relay between
// src and dst chains for all pending messages. Will also create or repair
// connections and channels
func NaiveRelayStrategy(src, dst *Chain) (*RelayMsgs, error) {
	srcMsgs := make([]sdk.Msg, 0)
	dstMsgs := make([]sdk.Msg, 0)

	// Addresses and heights for both the src and dst chains
	srcAddr, dstAddr, srcHeight, dstHeight, err := addrsHeights(src, dst)
	if err != nil {
		return nil, err
	}

	// ICS2 : Clients - DstClient
	// Fetch current client state
	dstClientState, err := dst.QueryClientConsensusState(src.ChainID, uint64(dstHeight))
	if err != nil {
		return nil, err
	}

	switch {
	// If there is no client found matching, create the client
	// TODO: ensure that this is the right condition
	case dstClientState.ConsensusState.GetRoot().GetHash() == nil:
		cc, err := dst.CreateClient(src, srcHeight, dstAddr)
		if err != nil {
			return nil, err
		}
		dstMsgs = append(dstMsgs, cc)

	// If there client is found update it with latest header
	case dstClientState.ProofHeight < uint64(srcHeight):
		uc, err := dst.UpdateClient(src, srcHeight, dstAddr)
		if err != nil {
			return nil, err
		}
		dstMsgs = append(dstMsgs, uc)
	}

	// ICS2 : Clients - SrcClient
	// Determine if light client needs to be updated on src
	srcClientState, err := src.QueryClientConsensusState(dst.ChainID, uint64(srcHeight))
	if err != nil {
		return nil, err
	}

	switch {
	// If there is no matching client found, create it
	// TODO: ensure that this is the right condition
	case srcClientState.ConsensusState.GetRoot().GetHash() == nil:
		cc, err := src.CreateClient(dst, dstHeight, srcAddr)
		if err != nil {
			return nil, err
		}
		srcMsgs = append(srcMsgs, cc)

	// If there client is found update it with latest header
	case srcClientState.ProofHeight < uint64(dstHeight):
		uc, err := src.UpdateClient(dst, dstHeight, srcAddr)
		if err != nil {
			return nil, err
		}
		srcMsgs = append(srcMsgs, uc)
	}

	// ICS3 : Connections
	// - Determine if any connection handshakes are in progress
	cp, err := src.GetCounterparty(dst.ChainID)
	if err != nil {
		return nil, err
	}

	// Fetch connections associated with clients on the source chain
	connections, err := src.QueryConnectionsUsingClient(cp.ClientID, srcHeight)
	if err != nil {
		return nil, err
	}

	// Loop across the connection paths
	for _, srcConnID := range connections.ConnectionPaths {
		// Fetch the dstEnd
		dstEnd, err := dst.QueryConnection(srcConnID, dstHeight)
		if err != nil {
			return nil, err
		}

		// Fetch the srcEnd
		srcEnd, err := src.QueryConnection(dstEnd.Connection.GetCounterparty().GetConnectionID(), srcHeight)
		if err != nil {
			return nil, err
		}

		switch {
		// Handshake hasn't been started locally, relay `connOpenInit` locally
		case srcEnd.Connection.State == connState.UNINITIALIZED:
			// TODO: need to add a msgUpdateClient here?
			srcMsgs = append(srcMsgs, connTypes.NewMsgConnectionOpenInit(
				dstEnd.Connection.GetCounterparty().GetConnectionID(),
				srcEnd.Connection.GetClientID(),
				srcEnd.Connection.GetCounterparty().GetConnectionID(),
				dstEnd.Connection.GetClientID(),
				srcEnd.Connection.Counterparty.GetPrefix(),
				srcAddr,
			))

		// Handshake has started locally (1 step done), relay `connOpenTry` to the remote end
		case srcEnd.Connection.State == connState.INIT && dstEnd.Connection.State == connState.UNINITIALIZED:
			// TODO: need to add a msgUpdateClient here?
			dstMsgs = append(dstMsgs, connTypes.NewMsgConnectionOpenTry(
				srcEnd.Connection.GetCounterparty().GetConnectionID(),
				dstEnd.Connection.GetClientID(),
				dstEnd.Connection.GetCounterparty().GetConnectionID(),
				srcEnd.Connection.GetClientID(),
				srcEnd.Connection.Counterparty.GetPrefix(),
				[]string{ibcversion},
				srcEnd.Proof,
				srcEnd.Proof,
				srcEnd.ProofHeight,
				uint64(dstHeight),
				dstAddr,
			))

		// Handshake has started on the other end (2 steps done), relay `connOpenAck` to the local end
		case srcEnd.Connection.State == connState.INIT && dstEnd.Connection.State == connState.TRYOPEN:
			// TODO: need to add a msgUpdateClient here?
			srcMsgs = append(srcMsgs, connTypes.NewMsgConnectionOpenAck(
				dstEnd.Connection.Counterparty.GetConnectionID(),
				dstEnd.Proof,
				dstEnd.Proof,
				dstEnd.ProofHeight,
				uint64(srcHeight),
				ibcversion,
				srcAddr,
			))

		// Handshake has confirmed locally (3 steps done), relay `connOpenConfirm` to the remote end
		case srcEnd.Connection.State == connState.OPEN && dstEnd.Connection.State == connState.TRYOPEN:
			// TODO: need to add a msgUpdateClient here?
			dstMsgs = append(dstMsgs, connTypes.NewMsgConnectionOpenConfirm(
				srcEnd.Connection.Counterparty.GetConnectionID(),
				srcEnd.Proof,
				srcEnd.ProofHeight,
				dstAddr,
			))
		}
	}

	// ICS4 : Channels & Packets
	// - Determine if any channel handshakes are in progress
	// - Determine if any packets, acknowledgements, or timeouts need to be relayed
	channels, err := src.QueryChannelsUsingConnections(connections.ConnectionPaths)
	if err != nil {
		return nil, err
	}

	for _, srcChan := range channels {
		// TODO: need to figure out how to handle channel and port identifiers

		dstEnd, err := dst.QueryChannel(srcChan.Channel.Counterparty.ChannelID, portID, dstHeight)
		if err != nil {
			return nil, err
		}

		srcEnd, err := src.QueryChannel(dstEnd.Channel.Counterparty.ChannelID, portID, srcHeight)
		if err != nil {
			return nil, err
		}

		// Deal with handshakes in progress
		switch {
		// Handshake hasn't been started locally, relay `chanOpenInit` locally
		case srcEnd.Channel.State == chanState.UNINITIALIZED:
			// TODO: need to add a msgUpdateClient here?
			srcMsgs = append(srcMsgs, chanTypes.NewMsgChannelOpenInit(
				portID,
				dstEnd.Channel.GetCounterparty().GetChannelID(),
				ibcversion,
				srcEnd.Channel.GetOrdering(),
				srcEnd.Channel.ConnectionHops,
				portID,
				srcEnd.Channel.Counterparty.GetChannelID(),
				srcAddr,
			))

		// Handshake has started locally (1 step done), relay `chanOpenTry` to the remote end
		case srcEnd.Channel.State == chanState.INIT && dstEnd.Channel.State == chanState.UNINITIALIZED:
			// TODO: need to add a msgUpdateClient here?
			dstMsgs = append(dstMsgs, chanTypes.NewMsgChannelOpenTry(
				portID,
				srcEnd.Channel.GetCounterparty().GetChannelID(),
				ibcversion,
				dstEnd.Channel.GetOrdering(),
				dstEnd.Channel.GetConnectionHops(),
				portID,
				dstEnd.Channel.Counterparty.GetChannelID(),
				ibcversion,
				srcEnd.Proof,
				srcEnd.ProofHeight,
				dstAddr,
			))
		// Handshake has started on the other end (2 steps done), relay `chanOpenAck` to the local end
		case srcEnd.Channel.State == chanState.INIT && dstEnd.Channel.State == chanState.TRYOPEN:
			// TODO: need to add a msgUpdateClient here?
			srcMsgs = append(srcMsgs, chanTypes.NewMsgChannelOpenAck(
				portID,
				dstEnd.Channel.GetCounterparty().GetChannelID(),
				ibcversion,
				dstEnd.Proof,
				dstEnd.ProofHeight,
				srcAddr,
			))
		// Handshake has confirmed locally (3 steps done), relay `chanOpenConfirm` to the remote end
		case srcEnd.Channel.State == chanState.OPEN && dstEnd.Channel.State == chanState.TRYOPEN:
			// TODO: need to add a msgUpdateClient here?
			dstMsgs = append(dstMsgs, chanTypes.NewMsgChannelOpenConfirm(
				portID,
				srcEnd.Channel.GetCounterparty().GetChannelID(),
				srcEnd.Proof,
				srcEnd.ProofHeight,
				dstAddr,
			))

		}

		// Deal with packets
		// TODO: Once ADR15 is merged this section needs to be completed cc @mossid @fedekunze @cwgoes

		// First, scan logs for sent packets and relay all of them
		// TODO: This is currently incorrect and will change
		srcRes, err := src.QueryTxs(uint64(srcHeight), []string{"type:transfer"})
		for _, tx := range srcRes.Txs {
			for _, msg := range tx.Tx.GetMsgs() {
				if msg.Type() == "transfer" {
					dstMsgs = append(dstMsgs, xferTypes.MsgRecvPacket{})
				}
			}
		}

		// Then, scan logs for received packets and relay acknowledgements
		// TODO: This is currently incorrect and will change
		dstRes, err := dst.QueryTxs(uint64(dstHeight), []string{"type:recv_packet"})
		for _, tx := range dstRes.Txs {
			for _, msg := range tx.Tx.GetMsgs() {
				if msg.Type() == "recv_packet" {
					dstMsgs = append(dstMsgs, xferTypes.MsgRecvPacket{})
				}
			}
		}
	}

	//   Return pending datagrams
	return &RelayMsgs{srcMsgs, dstMsgs}, nil
}

// Group the keybase and height queries here
func addrsHeights(src, dst *Chain) (srcAddr, dstAddr sdk.AccAddress, srcHeight, dstHeight int64, err error) {
	// Signing key for src chain
	srcAddr, err = src.GetAddress()
	if err != nil {
		return
	}

	// Signing key for dst chain
	dstAddr, err = dst.GetAddress()
	if err != nil {
		return
	}

	// Latest height on src chain
	srcHeight, err = src.QueryLatestHeight()
	if err != nil {
		return
	}

	// Latest height on dst chain
	dstHeight, err = dst.QueryLatestHeight()

	return
}
