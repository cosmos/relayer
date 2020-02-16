package relayer

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	chanState "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/exported"
	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint"
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
	out := &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}}

	err := UpdateLiteDBsToLatestHeaders(src, dst)
	if err != nil {
		return nil, err
	}

	hs, err := GetLatestHeaders(src, dst)
	if err != nil {
		return nil, err
	}

	// ICS2 : Clients - DstClient
	// Fetch current client state
	dstClientState, err := dst.QueryClientConsensusState(src.ChainID, uint64(hs[dst.ChainID].Height))
	if err != nil {
		return nil, err
	}

	switch {
	// If there is no client found matching, create the client
	// TODO: ensure that this is the right condition
	case dstClientState.ConsensusState.GetRoot().GetHash() == nil:
		out.Dst = append(out.Dst, dst.CreateClient("finddstclientid", hs[src.ChainID]))

	// If there client is found update it with latest header
	case dstClientState.ProofHeight < uint64(hs[src.ChainID].Height):
		out.Dst = append(out.Dst, dst.UpdateClient("finddstclientid", hs[src.ChainID]))
	}

	// ICS2 : Clients - SrcClient
	// Determine if light client needs to be updated on src
	srcClientState, err := src.QueryClientConsensusState(dst.ChainID, uint64(hs[src.ChainID].Height))
	if err != nil {
		return nil, err
	}

	switch {
	// If there is no matching client found, create it
	// TODO: ensure that this is the right condition
	case srcClientState.ConsensusState.GetRoot().GetHash() == nil:
		out.Src = append(out.Src, src.CreateClient("findsrcclientid", hs[dst.ChainID]))

	// If there client is found update it with latest header
	case srcClientState.ProofHeight < uint64(hs[dst.ChainID].Height):
		out.Src = append(out.Src, src.UpdateClient("findsrcclientid", hs[dst.ChainID]))
	}

	// ICS3 : Connections
	// - Determine if any connection handshakes are in progress
	cp, err := src.GetCounterparty(dst.ChainID)
	if err != nil {
		return nil, err
	}

	// Fetch connections associated with clients on the source chain
	connections, err := src.QueryConnectionsUsingClient(cp.ClientID, hs[src.ChainID].Height)
	if err != nil {
		return nil, err
	}

	// Loop across the connection paths
	for _, srcConnID := range connections.ConnectionPaths {
		return src.CreateConnectionStep(dst, "findsrcclientid", "finddstclientid", srcConnID, "finddstconnid")
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

		dstEnd, err := dst.QueryChannel(srcChan.Channel.Counterparty.ChannelID, portID, hs[dst.ChainID].Height)
		if err != nil {
			return nil, err
		}

		srcEnd, err := src.QueryChannel(dstEnd.Channel.Counterparty.ChannelID, portID, hs[src.ChainID].Height)
		if err != nil {
			return nil, err
		}

		// Deal with handshakes in progress
		switch {
		// Handshake hasn't been started locally, relay `chanOpenInit` locally
		case srcEnd.Channel.State == chanState.UNINITIALIZED:
			// TODO: need to add a msgUpdateClient here?
			out.Src = append(out.Src, chanTypes.NewMsgChannelOpenInit(
				portID,
				dstEnd.Channel.GetCounterparty().GetChannelID(),
				ibcversion,
				srcEnd.Channel.GetOrdering(),
				srcEnd.Channel.ConnectionHops,
				portID,
				srcEnd.Channel.Counterparty.GetChannelID(),
				src.MustGetAddress(),
			))

		// Handshake has started locally (1 step done), relay `chanOpenTry` to the remote end
		case srcEnd.Channel.State == chanState.INIT && dstEnd.Channel.State == chanState.UNINITIALIZED:
			// TODO: need to add a msgUpdateClient here?
			out.Dst = append(out.Dst, chanTypes.NewMsgChannelOpenTry(
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
				dst.MustGetAddress(),
			))
		// Handshake has started on the other end (2 steps done), relay `chanOpenAck` to the local end
		case srcEnd.Channel.State == chanState.INIT && dstEnd.Channel.State == chanState.TRYOPEN:
			// TODO: need to add a msgUpdateClient here?
			out.Src = append(out.Src, chanTypes.NewMsgChannelOpenAck(
				portID,
				dstEnd.Channel.GetCounterparty().GetChannelID(),
				ibcversion,
				dstEnd.Proof,
				dstEnd.ProofHeight,
				src.MustGetAddress(),
			))
		// Handshake has confirmed locally (3 steps done), relay `chanOpenConfirm` to the remote end
		case srcEnd.Channel.State == chanState.OPEN && dstEnd.Channel.State == chanState.TRYOPEN:
			// TODO: need to add a msgUpdateClient here?
			out.Dst = append(out.Dst, chanTypes.NewMsgChannelOpenConfirm(
				portID,
				srcEnd.Channel.GetCounterparty().GetChannelID(),
				srcEnd.Proof,
				srcEnd.ProofHeight,
				dst.MustGetAddress(),
			))

		}

		// Deal with packets
		// TODO: Once ADR15 is merged this section needs to be completed cc @mossid @fedekunze @cwgoes

		// First, scan logs for sent packets and relay all of them
		// TODO: This is currently incorrect and will change
		srcRes, err := src.QueryTxs(uint64(hs[src.ChainID].Height), []string{"type:transfer"})
		for _, tx := range srcRes.Txs {
			for _, msg := range tx.Tx.GetMsgs() {
				if msg.Type() == "transfer" {
					out.Dst = append(out.Dst, xferTypes.MsgRecvPacket{})
				}
			}
		}

		// Then, scan logs for received packets and relay acknowledgements
		// TODO: This is currently incorrect and will change
		dstRes, err := dst.QueryTxs(uint64(hs[dst.ChainID].Height), []string{"type:recv_packet"})
		for _, tx := range dstRes.Txs {
			for _, msg := range tx.Tx.GetMsgs() {
				if msg.Type() == "recv_packet" {
					out.Dst = append(out.Dst, xferTypes.MsgRecvPacket{})
				}
			}
		}
	}

	//   Return pending datagrams
	return out, nil
}

// Group the keybase and height queries here
func addrsHeaders(src, dst *Chain) (srcAddr, dstAddr sdk.AccAddress, srcHeader, dstHeader *tmclient.Header, err error) {
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
	srcHeader, err = src.QueryLatestHeader()
	if err != nil {
		return
	}

	// Latest height on dst chain
	dstHeader, err = dst.QueryLatestHeader()
	return
}
