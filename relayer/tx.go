package relayer

import (
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clientTypes "github.com/cosmos/cosmos-sdk/x/ibc/02-client/types"
	connState "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/exported"
	connTypes "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/types"
	chanState "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/exported"
	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint/types"
	commitmentypes "github.com/cosmos/cosmos-sdk/x/ibc/23-commitment/types"
)

var (
	defaultChainPrefix   = commitmentypes.NewMerklePrefix([]byte("ibc"))
	defaultIBCVersion    = "1.0.0"
	defaultIBCVersions   = []string{defaultIBCVersion}
	defaultUnbondingTime = time.Hour * 504 // 3 weeks in hours
)

// CreateClients creates clients for src on dst and dst on src given the configured paths
func (src *Chain) CreateClients(dst *Chain) (err error) {
	clients := &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}}

	// Create client for dst on src if it doesn't exist
	var srcCs, dstCs clientTypes.StateResponse
	if srcCs, err = src.QueryClientState(); err != nil {
		return err
	} else if srcCs.ClientState == nil {
		dstH, err := dst.UpdateLiteWithHeader()
		if err != nil {
			return err
		}
		if src.debug {
			src.logCreateClient(dst, dstH.GetHeight())
		}
		clients.Src = append(clients.Src, src.PathEnd.CreateClient(dstH, src.GetTrustingPeriod(), src.MustGetAddress()))
	}
	// TODO: maybe log something here that the client has been created?

	// Create client for src on dst if it doesn't exist
	if dstCs, err = dst.QueryClientState(); err != nil {
		return err
	} else if dstCs.ClientState == nil {
		srcH, err := src.UpdateLiteWithHeader()
		if err != nil {
			return err
		}
		if dst.debug {
			dst.logCreateClient(src, srcH.GetHeight())
		}
		clients.Dst = append(clients.Dst, dst.PathEnd.CreateClient(srcH, dst.GetTrustingPeriod(), dst.MustGetAddress()))
	}
	// TODO: maybe log something here that the client has been created?

	// Send msgs to both chains
	if clients.Ready() {
		if clients.Send(src, dst); clients.success {
			src.Log(fmt.Sprintf("★ Clients created: [%s]client(%s) and [%s]client(%s)",
				src.ChainID, src.PathEnd.ClientID, dst.ChainID, dst.PathEnd.ClientID))
		}
	}

	return nil
}

func (src *Chain) logCreateClient(dst *Chain, dstH uint64) {
	src.Log(fmt.Sprintf("- [%s] -> creating client for [%s]header-height{%d} trust-period(%s)", src.ChainID, dst.ChainID, dstH, dst.GetTrustingPeriod()))
}

// CreateConnection runs the connection creation messages on timeout until they pass
// TODO: add max retries or something to this function
func (src *Chain) CreateConnection(dst *Chain, to time.Duration) error {
	ticker := time.NewTicker(to)
	for ; true; <-ticker.C {
		connSteps, err := src.CreateConnectionStep(dst)
		if err != nil {
			return err
		}

		if !connSteps.Ready() {
			break
		}

		if connSteps.Send(src, dst); connSteps.success && connSteps.last {
			conns, err := QueryConnectionPair(src, dst, 0, 0)
			if err != nil {
				return err
			}

			if src.debug {
				logConnectionStates(src, dst, conns)
			}

			src.Log(fmt.Sprintf("★ Connection created: [%s]client{%s}conn{%s} -> [%s]client{%s}conn{%s}",
				src.ChainID, src.PathEnd.ClientID, src.PathEnd.ConnectionID,
				dst.ChainID, dst.PathEnd.ClientID, dst.PathEnd.ConnectionID))
			break
		}
	}

	return nil
}

// CreateConnectionStep returns the next set of messags for creating a channel
// with the given identifier between chains src and dst. If handshake hasn't started,
// CreateConnetionStep will start the handshake on src
func (src *Chain) CreateConnectionStep(dst *Chain) (*RelayMsgs, error) {
	out := &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}, last: false}

	if err := src.PathEnd.Validate(); err != nil {
		return nil, src.ErrCantSetPath(err)
	}

	if err := dst.PathEnd.Validate(); err != nil {
		return nil, dst.ErrCantSetPath(err)
	}

	hs, err := UpdatesWithHeaders(src, dst)
	if err != nil {
		return nil, err
	}

	scid, dcid := src.ChainID, dst.ChainID

	// Query Connection data from src and dst
	// NOTE: We query connection at height - 1 because of the way tendermint returns
	// proofs the commit for height n is contained in the header of height n + 1
	conn, err := QueryConnectionPair(src, dst, hs[scid].Height-1, hs[dcid].Height-1)
	if err != nil {
		return nil, err
	}

	// NOTE: We query connection at height - 1 because of the way tendermint returns
	// proofs the commit for height n is contained in the header of height n + 1
	cs, err := QueryClientStatePair(src, dst)
	if err != nil {
		return nil, err
	}

	// TODO: log these heights or something about client state? debug?

	// Store the heights
	srcConsH, dstConsH := int64(cs[scid].ClientState.GetLatestHeight()), int64(cs[dcid].ClientState.GetLatestHeight())

	// NOTE: We query connection at height - 1 because of the way tendermint returns
	// proofs the commit for height n is contained in the header of height n + 1
	cons, err := QueryClientConsensusStatePair(src, dst, hs[scid].Height-1, hs[dcid].Height-1, srcConsH, dstConsH)
	if err != nil {
		return nil, err
	}

	switch {
	// Handshake hasn't been started on src or dst, relay `connOpenInit` to src
	case conn[scid].Connection.State == connState.UNINITIALIZED && conn[dcid].Connection.State == connState.UNINITIALIZED:
		if src.debug {
			logConnectionStates(src, dst, conn)
		}
		out.Src = append(out.Src, src.PathEnd.ConnInit(dst.PathEnd, src.MustGetAddress()))

	// Handshake has started on dst (1 stepdone), relay `connOpenTry` and `updateClient` on src
	case conn[scid].Connection.State == connState.UNINITIALIZED && conn[dcid].Connection.State == connState.INIT:
		if src.debug {
			logConnectionStates(src, dst, conn)
		}
		out.Src = append(out.Src,
			src.PathEnd.UpdateClient(hs[dcid], src.MustGetAddress()),
			src.PathEnd.ConnTry(dst.PathEnd, conn[dcid], cons[dcid], dstConsH, src.MustGetAddress()),
		)

	// Handshake has started on src (1 step done), relay `connOpenTry` and `updateClient` on dst
	case conn[scid].Connection.State == connState.INIT && conn[dcid].Connection.State == connState.UNINITIALIZED:
		if dst.debug {
			logConnectionStates(dst, src, conn)
		}
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(hs[scid], dst.MustGetAddress()),
			dst.PathEnd.ConnTry(src.PathEnd, conn[scid], cons[scid], srcConsH, dst.MustGetAddress()),
		)

	// Handshake has started on src end (2 steps done), relay `connOpenAck` and `updateClient` to dst end
	case conn[scid].Connection.State == connState.TRYOPEN && conn[dcid].Connection.State == connState.INIT:
		if dst.debug {
			logConnectionStates(dst, src, conn)
		}
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(hs[scid], dst.MustGetAddress()),
			dst.PathEnd.ConnAck(conn[scid], cons[scid], srcConsH, dst.MustGetAddress()),
		)

	// Handshake has started on dst end (2 steps done), relay `connOpenAck` and `updateClient` to src end
	case conn[scid].Connection.State == connState.INIT && conn[dcid].Connection.State == connState.TRYOPEN:
		if src.debug {
			logConnectionStates(src, dst, conn)
		}
		out.Src = append(out.Src,
			src.PathEnd.UpdateClient(hs[dcid], src.MustGetAddress()),
			src.PathEnd.ConnAck(conn[dcid], cons[dcid], dstConsH, src.MustGetAddress()),
		)

	// Handshake has confirmed on dst (3 steps done), relay `connOpenConfirm` and `updateClient` to src end
	case conn[scid].Connection.State == connState.TRYOPEN && conn[dcid].Connection.State == connState.OPEN:
		if src.debug {
			logConnectionStates(src, dst, conn)
		}
		out.Src = append(out.Src,
			src.PathEnd.UpdateClient(hs[dcid], src.MustGetAddress()),
			src.PathEnd.ConnConfirm(conn[dcid], src.MustGetAddress()),
		)
		out.last = true

	// Handshake has confirmed on src (3 steps done), relay `connOpenConfirm` and `updateClient` to dst end
	case conn[scid].Connection.State == connState.OPEN && conn[dcid].Connection.State == connState.TRYOPEN:
		if dst.debug {
			logConnectionStates(dst, src, conn)
		}
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(hs[scid], dst.MustGetAddress()),
			dst.PathEnd.ConnConfirm(conn[scid], dst.MustGetAddress()),
		)
		out.last = true
	}

	return out, nil
}

func logConnectionStates(src, dst *Chain, conn map[string]connTypes.ConnectionResponse) {
	src.Log(fmt.Sprintf("- [%s]@{%d}conn(%s)-{%s} : [%s]@{%d}conn(%s)-{%s}",
		src.ChainID,
		conn[src.ChainID].ProofHeight,
		src.PathEnd.ConnectionID,
		conn[src.ChainID].Connection.GetState(),
		dst.ChainID,
		conn[dst.ChainID].ProofHeight,
		dst.PathEnd.ConnectionID,
		conn[dst.ChainID].Connection.GetState(),
	))
}

// CreateChannel runs the channel creation messages on timeout until they pass
// TODO: add max retries or something to this function
func (src *Chain) CreateChannel(dst *Chain, ordered bool, to time.Duration) error {
	var order chanState.Order
	if ordered {
		order = chanState.ORDERED
	} else {
		order = chanState.UNORDERED
	}

	ticker := time.NewTicker(to)
	for ; true; <-ticker.C {
		chanSteps, err := src.CreateChannelStep(dst, order)
		if err != nil {
			return err
		}

		if !chanSteps.Ready() {
			break
		}

		if chanSteps.Send(src, dst); chanSteps.success && chanSteps.last {
			chans, err := QueryChannelPair(src, dst, 0, 0)
			if err != nil {
				return err
			}
			if src.debug {
				logChannelStates(src, dst, chans)
			}
			src.Log(fmt.Sprintf("★ Channel created: [%s]chan{%s}port{%s} -> [%s]chan{%s}port{%s}",
				src.ChainID, src.PathEnd.ChannelID, src.PathEnd.PortID,
				dst.ChainID, dst.PathEnd.ChannelID, dst.PathEnd.PortID))
			break
		}
	}

	return nil
}

// CreateChannelStep returns the next set of messages for creating a channel with given
// identifiers between chains src and dst. If the handshake hasn't started, then CreateChannelStep
// will begin the handshake on the src chain
func (src *Chain) CreateChannelStep(dst *Chain, ordering chanState.Order) (*RelayMsgs, error) {
	var (
		out        = &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}, last: false}
		scid, dcid = src.ChainID, dst.ChainID
	)

	if err := src.PathEnd.Validate(); err != nil {
		return nil, src.ErrCantSetPath(err)
	}

	if err := dst.PathEnd.Validate(); err != nil {
		return nil, dst.ErrCantSetPath(err)
	}

	hs, err := UpdatesWithHeaders(src, dst)
	if err != nil {
		return nil, err
	}

	chans, err := QueryChannelPair(src, dst, hs[scid].Height-1, hs[dcid].Height-1)
	if err != nil {
		return nil, err
	}

	switch {
	// Handshake hasn't been started on src or dst, relay `chanOpenInit` to src
	case chans[scid].Channel.State == chanState.UNINITIALIZED && chans[dcid].Channel.State == chanState.UNINITIALIZED:
		if src.debug {
			logChannelStates(src, dst, chans)
		}
		out.Src = append(out.Src,
			src.PathEnd.ChanInit(dst.PathEnd, ordering, src.MustGetAddress()),
		)

	// Handshake has started on dst (1 step done), relay `chanOpenTry` and `updateClient` to src
	case chans[scid].Channel.State == chanState.UNINITIALIZED && chans[dcid].Channel.State == chanState.INIT:
		if src.debug {
			logChannelStates(src, dst, chans)
		}
		out.Src = append(out.Src,
			src.PathEnd.UpdateClient(hs[dcid], src.MustGetAddress()),
			src.PathEnd.ChanTry(dst.PathEnd, chans[dcid], src.MustGetAddress()),
		)

	// Handshake has started on src (1 step done), relay `chanOpenTry` and `updateClient` to dst
	case chans[scid].Channel.State == chanState.INIT && chans[dcid].Channel.State == chanState.UNINITIALIZED:
		if dst.debug {
			logChannelStates(dst, src, chans)
		}
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(hs[scid], dst.MustGetAddress()),
			dst.PathEnd.ChanTry(src.PathEnd, chans[scid], dst.MustGetAddress()),
		)

	// Handshake has started on src (2 steps done), relay `chanOpenAck` and `updateClient` to dst
	case chans[scid].Channel.State == chanState.TRYOPEN && chans[dcid].Channel.State == chanState.INIT:
		if dst.debug {
			logChannelStates(dst, src, chans)
		}
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(hs[scid], dst.MustGetAddress()),
			dst.PathEnd.ChanAck(chans[scid], dst.MustGetAddress()),
		)

	// Handshake has started on dst (2 steps done), relay `chanOpenAck` and `updateClient` to src
	case chans[scid].Channel.State == chanState.INIT && chans[dcid].Channel.State == chanState.TRYOPEN:
		if src.debug {
			logChannelStates(src, dst, chans)
		}
		out.Src = append(out.Src,
			src.PathEnd.UpdateClient(hs[dcid], src.MustGetAddress()),
			src.PathEnd.ChanAck(chans[dcid], src.MustGetAddress()),
		)

	// Handshake has confirmed on dst (3 steps done), relay `chanOpenConfirm` and `updateClient` to src
	case chans[scid].Channel.State == chanState.TRYOPEN && chans[dcid].Channel.State == chanState.OPEN:
		if src.debug {
			logChannelStates(src, dst, chans)
		}
		out.Src = append(out.Src,
			src.PathEnd.UpdateClient(hs[dcid], src.MustGetAddress()),
			src.PathEnd.ChanConfirm(chans[dcid], src.MustGetAddress()),
		)
		out.last = true

	// Handshake has confirmed on src (3 steps done), relay `chanOpenConfirm` and `updateClient` to dst
	case chans[scid].Channel.State == chanState.OPEN && chans[dcid].Channel.State == chanState.TRYOPEN:
		if dst.debug {
			logChannelStates(dst, src, chans)
		}
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(hs[scid], dst.MustGetAddress()),
			dst.PathEnd.ChanConfirm(chans[scid], dst.MustGetAddress()),
		)
		out.last = true
	}

	return out, nil
}

// CloseChannel runs the channel closing messages on timeout until they pass
// TODO: add max retries or something to this function
func (src *Chain) CloseChannel(dst *Chain, to time.Duration) error {

	ticker := time.NewTicker(to)
	for ; true; <-ticker.C {
		closeSteps, err := src.CloseChannelStep(dst)
		if err != nil {
			return err
		}

		if !closeSteps.Ready() {
			break
		}

		if closeSteps.Send(src, dst); closeSteps.success && closeSteps.last {
			chans, err := QueryChannelPair(src, dst, 0, 0)
			if err != nil {
				return err
			}
			if src.debug {
				logChannelStates(src, dst, chans)
			}
			src.Log(fmt.Sprintf("★ Closed channel between [%s]chan{%s}port{%s} -> [%s]chan{%s}port{%s}",
				src.ChainID, src.PathEnd.ChannelID, src.PathEnd.PortID,
				dst.ChainID, dst.PathEnd.ChannelID, dst.PathEnd.PortID))
			break
		}
	}
	return nil
}

// CloseChannelStep returns the next set of messages for closing a channel with given
// identifiers between chains src and dst. If the closing handshake hasn't started, then CloseChannelStep
// will begin the handshake on the src chain
func (src *Chain) CloseChannelStep(dst *Chain) (*RelayMsgs, error) {
	var (
		out        = &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}, last: false}
		scid, dcid = src.ChainID, dst.ChainID
	)

	if err := src.PathEnd.Validate(); err != nil {
		return nil, src.ErrCantSetPath(err)
	}

	if err := dst.PathEnd.Validate(); err != nil {
		return nil, dst.ErrCantSetPath(err)
	}

	hs, err := UpdatesWithHeaders(src, dst)
	if err != nil {
		return nil, err
	}

	chans, err := QueryChannelPair(src, dst, hs[scid].Height-1, hs[dcid].Height-1)
	logChannelStates(src, dst, chans)

	switch {
	// Closing handshake has not started, relay `updateClient` and `chanCloseInit` to src or dst according
	// to the channel state
	case chans[scid].Channel.State != chanState.CLOSED && chans[dcid].Channel.State != chanState.CLOSED:
		if chans[scid].Channel.State != chanState.UNINITIALIZED {
			if src.debug {
				logChannelStates(src, dst, chans)
			}
			out.Src = append(out.Src,
				src.PathEnd.UpdateClient(hs[dcid], src.MustGetAddress()),
				src.PathEnd.ChanCloseInit(src.MustGetAddress()),
			)
		} else if chans[dcid].Channel.State != chanState.UNINITIALIZED {
			if dst.debug {
				logChannelStates(dst, src, chans)
			}
			out.Dst = append(out.Dst,
				dst.PathEnd.UpdateClient(hs[scid], dst.MustGetAddress()),
				dst.PathEnd.ChanCloseInit(dst.MustGetAddress()),
			)
		}

	// Closing handshake has started on src, relay `updateClient` and `chanCloseConfirm` to dst
	case chans[scid].Channel.State == chanState.CLOSED && chans[dcid].Channel.State != chanState.CLOSED:
		if chans[dcid].Channel.State != chanState.UNINITIALIZED {
			if dst.debug {
				logChannelStates(dst, src, chans)
			}
			out.Dst = append(out.Dst,
				dst.PathEnd.UpdateClient(hs[scid], dst.MustGetAddress()),
				dst.PathEnd.ChanCloseConfirm(chans[scid], dst.MustGetAddress()),
			)
			out.last = true
		}

	// Closing handshake has started on dst, relay `updateClient` and `chanCloseConfirm` to src
	case chans[dcid].Channel.State == chanState.CLOSED && chans[scid].Channel.State != chanState.CLOSED:
		if chans[scid].Channel.State != chanState.UNINITIALIZED {
			if src.debug {
				logChannelStates(src, dst, chans)
			}
			out.Src = append(out.Src,
				src.PathEnd.UpdateClient(hs[dcid], src.MustGetAddress()),
				src.PathEnd.ChanCloseConfirm(chans[dcid], src.MustGetAddress()),
			)
			out.last = true
		}
	}
	return out, nil
}

func logChannelStates(src, dst *Chain, conn map[string]chanTypes.ChannelResponse) {
	// TODO: replace channelID with portID?
	src.Log(fmt.Sprintf("- [%s]@{%d}chan(%s)-{%s} : [%s]@{%d}chan(%s)-{%s}",
		src.ChainID,
		conn[src.ChainID].ProofHeight,
		src.PathEnd.ChannelID,
		conn[src.ChainID].Channel.GetState(),
		dst.ChainID,
		conn[dst.ChainID].ProofHeight,
		dst.PathEnd.ChannelID,
		conn[dst.ChainID].Channel.GetState(),
	))
}

// ClearQueues creates transactions to clear both queues
func ClearQueues(src, dst *Chain) error {
	// Update lite clients, headers to be used later
	hs, err := UpdatesWithHeaders(src, dst)
	if err != nil {
		return err
	}

	// find any unrelayed packets
	sp, err := UnrelayedSequences(src, dst, hs[src.ChainID].Height, hs[dst.ChainID].Height)
	if err != nil {
		return err
	}

	// create the appropriate update client messages
	msgs := &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}}
	if len(sp.Src) > 0 {
		msgs.Src = append(msgs.Src, src.PathEnd.UpdateClient(hs[dst.ChainID], src.MustGetAddress()))
	}
	if len(sp.Dst) > 0 {
		msgs.Dst = append(msgs.Dst, dst.PathEnd.UpdateClient(hs[src.ChainID], dst.MustGetAddress()))
	}

	// add messages for src -> dst
	for _, seq := range sp.Src {
		if err = addPacketMsg(src, dst, hs[src.ChainID], hs[dst.ChainID], seq, msgs, true); err != nil {
			return err
		}
	}

	// add messages for dst -> src
	for _, seq := range sp.Dst {
		if err = addPacketMsg(dst, src, hs[dst.ChainID], hs[src.ChainID], seq, msgs, false); err != nil {
			return err
		}
	}

	if !msgs.Ready() {
		src.Log(fmt.Sprintf("- No packets to relay between [%s]port{%s} and [%s]port{%s}", src.ChainID, src.PathEnd.PortID, dst.ChainID, dst.PathEnd.PortID))
		return nil
	}

	// TODO: increase the amount of gas as the number of messages increases
	// notify the user of that

	if msgs.Send(src, dst); msgs.success {
		src.Log(fmt.Sprintf("★ Clients updated: [%s]client(%s) and [%s]client(%s)",
			src.ChainID, src.PathEnd.ClientID, dst.ChainID, dst.PathEnd.ClientID))
		if len(msgs.Dst) > 1 {
			src.logPacketsRelayed(dst, len(msgs.Dst)-1)
		}
		if len(msgs.Src) > 1 {
			dst.logPacketsRelayed(src, len(msgs.Src)-1)
		}
	}

	return nil
}

func (src *Chain) logPacketsRelayed(dst *Chain, num int) {
	dst.Log(fmt.Sprintf("★ Relayed %d packets: [%s]port{%s}->[%s]port{%s}", num, dst.ChainID, dst.PathEnd.PortID, src.ChainID, src.PathEnd.PortID))
}

func addPacketMsg(src, dst *Chain, srcH, dstH *tmclient.Header, seq uint64, msgs *RelayMsgs, source bool) error {
	tx, err := src.QueryTxs(srcH.GetHeight(), 1, 1000, seqEvents(src.PathEnd.ChannelID, seq))
	if err != nil || tx.Count != 1 {
		return fmt.Errorf("error or more than one transaction returned: %w", err)
	}
	pd, err := src.txPacketData(src, tx.Txs[0])
	if err != nil {
		return err
	}
	msg, err := dst.packetMsg(src, srcH, pd, int64(seq))
	if err != nil {
		return err
	}
	// If we have a transaction to relay that hasn't been, and there are no messages yet,
	// we need to append an update_client message
	if source {
		if len(msgs.Dst) == 0 {

		}
		msgs.Dst = append(msgs.Dst, msg)
	} else {
		msgs.Src = append(msgs.Src, msg)
	}
	return nil
}

func (src *Chain) packetMsg(dst *Chain, dstH *tmclient.Header, xferPacket chanState.PacketDataI, seq int64) (sdk.Msg, error) {
	var (
		err          error
		dstCommitRes CommitmentResponse
	)

	dstCommitRes, err = dst.QueryPacketCommitment(dstH.Height-1, int64(seq))
	if err != nil {
		return nil, err
	}

	if dstCommitRes.Proof.Proof == nil {
		return nil, fmt.Errorf("[%s]@{%d} - Packet Commitment Proof is nil seq(%d)", dst.ChainID, dstH.Height-1, seq)
	}

	return src.PathEnd.MsgRecvPacket(dst.PathEnd, uint64(seq), xferPacket,
		chanTypes.NewPacketResponse(dst.PathEnd.PortID, dst.PathEnd.ChannelID, uint64(seq),
			dst.PathEnd.NewPacket(src.PathEnd, uint64(seq), xferPacket),
			dstCommitRes.Proof.Proof,
			int64(dstCommitRes.ProofHeight),
		),
		src.MustGetAddress(),
	), nil
}

func (src *Chain) txPacketData(dst *Chain, res sdk.TxResponse) (packetData chanState.PacketDataI, err error) {
	// TODO: Log what we are about to do here, maybe set behind debug flag

	// Set sdk config to use custom Bech32 account prefix
	sdkConf := sdk.GetConfig()
	sdkConf.SetBech32PrefixForAccount(dst.AccountPrefix, dst.AccountPrefix+"pub")

	for _, l := range res.Logs {
		for _, e := range l.Events {
			if e.Type == "send_packet" {
				for _, p := range e.Attributes {
					if p.Key == "packet_data" {
						if err = src.Cdc.UnmarshalJSON([]byte(p.Value), &packetData); err != nil {
							return
						}
						return
					}
				}
			}
		}
	}
	return nil, fmt.Errorf("no packet data found")
}

func seqEvents(channelID string, seq uint64) []string {
	eve, _ := ParseEvents(fmt.Sprintf("send_packet.packet_src_channel=%s&send_packet.packet_sequence=%d", channelID, seq))
	return eve
}
