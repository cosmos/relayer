package relayer

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	chanState "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/exported"
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

// Ready returns true if there are messages to relay
func (r *RelayMsgs) Ready() bool {
	if len(r.Src) == 0 && len(r.Dst) == 0 {
		return true
	}
	return false
}

// NaiveRelayStrategy returns the RelayMsgs that need to be run to relay between
// src and dst chains for all pending messages. Will also create or repair
// connections and channels
func NaiveRelayStrategy(src, dst *Chain) (*RelayMsgs, error) {
	out := &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}}

	hs, err := UpdatesWithHeaders(src, dst)
	if err != nil {
		return nil, err
	}

	// ICS2 : Clients - DstClient
	// Fetch current client state
	srcClientState, err := src.QueryClientConsensusState(uint64(hs[src.ChainID].Height))
	if err != nil {
		return nil, err
	}

	switch {
	// If there is no matching client found, create it
	// TODO: ensure that this is the right condition
	case srcClientState.ConsensusState.GetRoot().GetHash() == nil:
		out.Src = append(out.Src, src.CreateClient(hs[dst.ChainID]))

	// If there client is found update it with latest header
	case srcClientState.ProofHeight < uint64(hs[dst.ChainID].Height):
		out.Src = append(out.Src, src.UpdateClient(hs[dst.ChainID]))
	}

	dstClientState, err := dst.QueryClientConsensusState(uint64(hs[dst.ChainID].Height))
	if err != nil {
		return nil, err
	}

	switch {
	// If there is no client found matching, create the client
	// TODO: ensure that this is the right condition
	case dstClientState.ConsensusState.GetRoot().GetHash() == nil:
		out.Dst = append(out.Dst, dst.CreateClient(hs[src.ChainID]))

	// If there client is found update it with latest header
	case dstClientState.ProofHeight < uint64(hs[src.ChainID].Height):
		out.Dst = append(out.Dst, dst.UpdateClient(hs[src.ChainID]))
	}

	// Return here and move on to the next iteration
	if out.Ready() {
		return out, nil
	}

	// ICS3 : Connections
	// - Determine if any connection handshakes are in progress
	// Fetch connections associated with clients on the source chain
	connections, err := src.QueryConnectionsUsingClient(hs[src.ChainID].Height)
	if err != nil {
		return nil, err
	}

	// Loop across the connection paths
	for _, srcConnID := range connections.ConnectionPaths {
		if srcConnID == src.PathEnd.ConnectionID {
			return src.CreateConnectionStep(dst)
		}
	}

	// Return here and move on to the next iteration
	if out.Ready() {
		return out, nil
	}

	// ICS4 : Channels
	// - Determine if any channel handshakes are in progress

	channels, err := src.QueryChannelsUsingConnections(connections.ConnectionPaths)
	if err != nil {
		return nil, err
	}

	for _, srcChan := range channels {
		if srcChan.Channel.GetCounterparty().GetChannelID() == dst.PathEnd.ChannelID {
			return src.CreateChannelStep(dst, chanState.ORDERED)
		}
	}

	// Return here and move on to the next iteration
	if out.Ready() {
		return out, nil
	}

	// ICS?: Packet Messages
	// - Determine if any packets, acknowledgements, or timeouts need to be relayed
	for _, srcChan := range channels {
		if srcChan.Channel.GetCounterparty().GetChannelID() == dst.PathEnd.ChannelID {
			// Deal with packets
			// TODO: Once ADR15 is merged this section needs to be completed cc @mossid @fedekunze @cwgoes

			// First, scan logs for sent packets and relay all of them
			// TODO: This is currently incorrect and will change
			srcRes, err := src.QueryTxs(uint64(hs[src.ChainID].Height), []string{"type:transfer"})
			if err != nil {
				return nil, err
			}

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
			if err != nil {
				return nil, err
			}

			for _, tx := range dstRes.Txs {
				for _, msg := range tx.Tx.GetMsgs() {
					if msg.Type() == "recv_packet" {
						out.Dst = append(out.Dst, xferTypes.MsgRecvPacket{})
					}
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
