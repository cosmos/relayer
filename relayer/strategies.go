package relayer

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	clientTypes "github.com/cosmos/cosmos-sdk/x/ibc/02-client/types"
	connState "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/exported"
	connTypes "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/types"
	chanState "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/exported"
	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
	xferTypes "github.com/cosmos/cosmos-sdk/x/ibc/20-transfer/types"
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
type RelayStrategy func(src, dst *Chain) (RelayMsgs, error)

// RelayMsgs contains the msgs that need to be sent to both a src and dst chain
// after a given relay round
type RelayMsgs struct {
	Src []sdk.Msg
	Dst []sdk.Msg
}

// NaiveRelayStrategy returns the RelayMsgs that need to be run to relay between
// src and dst chains for all pending messages. Will also create or repair
// connections and channels
func NaiveRelayStrategy(src, dst *Chain) (RelayMsgs, error) {
	srcMsgs := make([]sdk.Msg, 0)
	dstMsgs := make([]sdk.Msg, 0)

	// Signing key for src chain
	srcAddr, err := src.Keybase.Get(src.Key)
	if err != nil {
		return RelayMsgs{}, err
	}

	// Signing key for dst chain
	dstAddr, err := dst.Keybase.Get(dst.Key)
	if err != nil {
		return RelayMsgs{}, err
	}

	// ICS2 : Clients
	// Determine if light client needs to be updated on dst
	// TODO: Do we need to randomly generate client IDs here?
	if dst.QueryConsensusState().ProofHeight < src.LatestHeight() {
		cp, err := src.GetCounterparty(dst.ChainID)
		if err != nil {
			return RelayMsgs{}, err
		}

		dstMsgs = append(dstMsgs,
			clientTypes.NewMsgUpdateClient(cp.ClientID, dst.LatestHeader(), dstAddr.GetAddress()))
	}

	// Determine if light client needs to be updated on src
	// TODO: Do we need to randomly generate client IDs here?
	if src.QueryConsensusState().ProofHeight < dst.LatestHeight() {
		cp, err := dst.GetCounterparty(src.ChainID)
		if err != nil {
			return RelayMsgs{}, err
		}

		srcMsgs = append(srcMsgs,
			clientTypes.NewMsgUpdateClient(cp.ClientID, dst.LatestHeader(), srcAddr.GetAddress()))
	}

	// ICS3 : Connections
	// - Determine if any connection handshakes are in progress
	connections := src.GetConnectionsUsingClient(dst)
	for _, srcEnd := range connections {
		dstEnd := dst.GetConnection(srcEnd.Counterparty.ConnectionID)

		// Handshake has started locally (1 step done), relay `connOpenTry` to the remote end
		if srcEnd.State == connState.INIT && dstEnd.State == connState.UNINITIALIZED {
			// TODO: move to NewMsgOpenTry and apply correct args
			dstMsgs = append(dstMsgs, connTypes.MsgConnectionOpenTry{})
		}

		// Handshake has started on the other end (2 steps done), relay `connOpenAck` to the local end
		if srcEnd.State == connState.INIT && dstEnd.State == connState.TRYOPEN {
			// TODO: move to NewMsgOpenAck and apply correct args
			srcMsgs = append(srcMsgs, connTypes.MsgConnectionOpenAck{})
		}

		// Handshake has confirmed locally (3 steps done), relay `connOpenConfirm` to the remote end
		if srcEnd.State == connState.OPEN && dstEnd.State == connState.TRYOPEN {
			// TODO: move to NewMsgOpenConfirm and apply correct args
			dstMsgs = append(dstMsgs, connTypes.MsgConnectionOpenConfirm{})
		}
	}

	// ICS4 : Channels & Packets
	// - Determine if any channel handshakes are in progress
	// - Determine if any packets, acknowledgements, or timeouts need to be relayed
	channels := src.GetChannelsUsingConnections(connections)
	for _, srcEnd := range channels {
		dstEnd := dst.GetChannel(srcEnd.Counterparty.ChannelID)
		// Deal with handshakes in progress

		// Handshake has started locally (1 step done), relay `chanOpenTry` to the remote end
		if srcEnd.State == chanState.INIT && dstEnd.State == chanState.UNINITIALIZED {
			// TODO: move to NewMsgOpenTry and apply correct args
			dstMsgs = append(dstMsgs, chanTypes.MsgChannelOpenTry{})
		}

		// Handshake has started on the other end (2 steps done), relay `chanOpenAck` to the local end
		if srcEnd.State == chanState.INIT && dstEnd.State == chanState.TRYOPEN {
			// TODO: move to NewMsgOpenAck and apply correct args
			srcMsgs = append(srcMsgs, chanTypes.MsgChannelOpenAck{})
		}

		// Handshake has confirmed locally (3 steps done), relay `chanOpenConfirm` to the remote end
		if srcEnd.State == chanState.OPEN && dstEnd.State == chanState.TRYOPEN {
			// TODO: move to NewMsgOpenConfirm and apply correct args
			dstMsgs = append(dstMsgs, chanTypes.MsgChannelOpenConfirm{})
		}

		// Deal with packets
		// TODO: Once ADR15 is merged this section needs to be completed cc @mossid @fedekunze @cwgoes

		// First, scan logs for sent packets and relay all of them
		// TODO: This is currently incorrect and will change
		for _, tx := range src.QueryTxs(src.LatestHeight(), "type:transfer") {
			for _, msg := range tx.Msgs {
				if msg.Type() == "transfer" {
					dstMsgs = append(dstMsgs, xferTypes.MsgRecvPacket{})
				}
			}
		}

		// Then, scan logs for received packets and relay acknowledgements
		// TODO: This is currently incorrect and will change
		for _, tx := range src.QueryTxs(src.LatestHeight(), "type:recv_packet") {
			for _, msg := range tx.Msgs {
				if msg.Type() == "recv_packet" {
					dstMsgs = append(dstMsgs, xferTypes.MsgRecvPacket{})
				}
			}
		}
	}

	//   Return for pending datagrams
	return RelayMsgs{srcMsgs, dstMsgs}, nil
}
