package relayer

import (
	"fmt"
	"strconv"
	"time"

	retry "github.com/avast/retry-go"
	sdk "github.com/cosmos/cosmos-sdk/types"
	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint/types"
	commitmentypes "github.com/cosmos/cosmos-sdk/x/ibc/23-commitment/types"
)

var (
	defaultChainPrefix     = commitmentypes.NewMerklePrefix([]byte("ibc"))
	defaultIBCVersion      = "1.0.0"
	defaultIBCVersions     = []string{defaultIBCVersion}
	defaultTransferVersion = "ics20-1"
	defaultUnbondingTime   = time.Hour * 504 // 3 weeks in hours
	defaultPacketTimeout   = 1000
	defaultPacketQuery     = "send_packet.packet_src_channel=%s&send_packet.packet_sequence=%d"
	// defaultPacketAckQuery  = "recv_packet.packet_src_channel=%s&recv_packet.packet_sequence=%d"
)

// RelayPacketsOrderedChan creates transactions to clear both queues
// CONTRACT: the SyncHeaders passed in here must be up to date or being kept updated
func RelayPacketsOrderedChan(src, dst *Chain, sh *SyncHeaders, sp *RelaySequences) error {

	// create the appropriate update client messages
	msgs := &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}}
	if len(sp.Src) > 0 {
		msgs.Dst = append(msgs.Dst, dst.PathEnd.UpdateClient(sh.GetHeader(src.ChainID), dst.MustGetAddress()))
	}
	if len(sp.Dst) > 0 {
		msgs.Src = append(msgs.Src, src.PathEnd.UpdateClient(sh.GetHeader(dst.ChainID), src.MustGetAddress()))
	}

	// add messages for src -> dst
	for _, seq := range sp.Src {
		msg, err := packetMsgFromTxQuery(src, dst, sh, seq)
		if err != nil {
			return err
		}
		msgs.Dst = append(msgs.Dst, msg)
	}

	// add messages for dst -> src
	for _, seq := range sp.Dst {
		msg, err := packetMsgFromTxQuery(dst, src, sh, seq)
		if err != nil {
			return err
		}
		msgs.Src = append(msgs.Src, msg)
	}

	if !msgs.Ready() {
		src.Log(fmt.Sprintf("- No packets to relay between [%s]port{%s} and [%s]port{%s}", src.ChainID, src.PathEnd.PortID, dst.ChainID, dst.PathEnd.PortID))
		return nil
	}

	// TODO: increase the amount of gas as the number of messages increases
	// notify the user of that
	if msgs.Send(src, dst); msgs.success {
		if len(msgs.Dst) > 1 {
			dst.logPacketsRelayed(src, len(msgs.Dst)-1)
		}
		if len(msgs.Src) > 1 {
			src.logPacketsRelayed(dst, len(msgs.Src)-1)
		}
	}

	return nil
}

// SendTransferBothSides sends a ICS20 packet from src to dst
func (src *Chain) SendTransferBothSides(dst *Chain, amount sdk.Coin, dstAddr sdk.AccAddress, source bool) error {

	if source {
		amount.Denom = fmt.Sprintf("%s/%s/%s", dst.PathEnd.PortID, dst.PathEnd.ChannelID, amount.Denom)
	} else {
		amount.Denom = fmt.Sprintf("%s/%s/%s", src.PathEnd.PortID, src.PathEnd.ChannelID, amount.Denom)
	}

	dstHeader, err := dst.UpdateLiteWithHeader()
	if err != nil {
		return err
	}

	timeoutHeight := dstHeader.GetHeight() + uint64(defaultPacketTimeout)

	// Properly render the address string
	done := dst.UseSDKContext()
	dstAddrString := dstAddr.String()
	done()

	// MsgTransfer will call SendPacket on src chain
	txs := RelayMsgs{
		Src: []sdk.Msg{src.PathEnd.MsgTransfer(
			dst.PathEnd, dstHeader.GetHeight(), sdk.NewCoins(amount), dstAddrString, src.MustGetAddress(),
		)},
		Dst: []sdk.Msg{},
	}

	if txs.Send(src, dst); !txs.Success() {
		return fmt.Errorf("failed to send first transaction")
	}

	// Working on SRC chain :point_up:
	// Working on DST chain :point_down:

	var (
		hs           map[string]*tmclient.Header
		seqRecv      chanTypes.RecvResponse
		seqSend      uint64
		srcCommitRes CommitmentResponse
	)

	if err = retry.Do(func() error {
		hs, err = UpdatesWithHeaders(src, dst)
		if err != nil {
			return err
		}

		seqRecv, err = dst.QueryNextSeqRecv(hs[dst.ChainID].Height)
		if err != nil {
			return err
		}

		seqSend, err = src.QueryNextSeqSend(hs[src.ChainID].Height)
		if err != nil {
			return err
		}

		srcCommitRes, err = src.QueryPacketCommitment(hs[src.ChainID].Height-1, int64(seqSend-1))
		if err != nil {
			return err
		}

		if srcCommitRes.Proof.Proof == nil {
			return fmt.Errorf("proof nil, retrying")
		}

		return nil
	}); err != nil {
		return err
	}

	// Properly render the source and destination address strings
	done = src.UseSDKContext()
	srcAddrString := src.MustGetAddress().String()
	done()

	done = dst.UseSDKContext()
	dstAddrString = dstAddr.String()
	done()

	// reconstructing packet data here instead of retrieving from an indexed node
	xferPacket := src.PathEnd.XferPacket(
		sdk.NewCoins(amount),
		srcAddrString,
		dstAddrString,
	)

	// Debugging by simply passing in the packet information that we know was sent earlier in the SendPacket
	// part of the command. In a real relayer, this would be a separate command that retrieved the packet
	// information from an indexing node
	txs = RelayMsgs{
		Dst: []sdk.Msg{
			dst.PathEnd.UpdateClient(hs[src.ChainID], dst.MustGetAddress()),
			dst.PathEnd.MsgRecvPacket(
				src.PathEnd,
				seqRecv.NextSequenceRecv,
				timeoutHeight,
				xferPacket,
				srcCommitRes.Proof,
				srcCommitRes.ProofHeight,
				dst.MustGetAddress(),
			),
		},
		Src: []sdk.Msg{},
	}

	txs.Send(src, dst)
	return nil
}

// SendTransferMsg initiates an ibs20 transfer from src to dst with the specified args
func (src *Chain) SendTransferMsg(dst *Chain, amount sdk.Coin, dstAddr sdk.AccAddress, source bool) error {

	if source {
		amount.Denom = fmt.Sprintf("%s/%s/%s", dst.PathEnd.PortID, dst.PathEnd.ChannelID, amount.Denom)
	} else {
		amount.Denom = fmt.Sprintf("%s/%s/%s", src.PathEnd.PortID, src.PathEnd.ChannelID, amount.Denom)
	}

	dstHeader, err := dst.UpdateLiteWithHeader()
	if err != nil {
		return err
	}

	// Properly render the address string
	done := dst.UseSDKContext()
	dstAddrString := dstAddr.String()
	done()

	// MsgTransfer will call SendPacket on src chain
	txs := RelayMsgs{
		Src: []sdk.Msg{src.PathEnd.MsgTransfer(
			dst.PathEnd, dstHeader.GetHeight(), sdk.NewCoins(amount), dstAddrString, src.MustGetAddress(),
		)},
		Dst: []sdk.Msg{},
	}

	if txs.Send(src, dst); !txs.success {
		return fmt.Errorf("failed to send transfer message")
	}
	return nil
}

// SendPacket sends arbitrary bytes from src to dst
func (src *Chain) SendPacket(dst *Chain, packetData []byte) error {
	dstHeader, err := dst.UpdateLiteWithHeader()
	if err != nil {
		return err
	}

	// MsgSendPacket will call SendPacket on src chain
	txs := RelayMsgs{
		Src: []sdk.Msg{src.PathEnd.MsgSendPacket(
			dst.PathEnd, packetData, dstHeader.GetHeight()+uint64(defaultPacketTimeout), src.MustGetAddress(),
		)},
		Dst: []sdk.Msg{},
	}

	if txs.Send(src, dst); !txs.success {
		return fmt.Errorf("failed to send packet")
	}
	return nil
}

// packetMsgFromTxQuery returns a sdk.Msg to relay a packet with a given seq on src
func packetMsgFromTxQuery(src, dst *Chain, sh *SyncHeaders, seq uint64) (sdk.Msg, error) {
	eve, err := ParseEvents(fmt.Sprintf(defaultPacketQuery, src.PathEnd.ChannelID, seq))
	if err != nil {
		return nil, err
	}

	tx, err := src.QueryTxs(sh.GetHeight(src.ChainID), 1, 1000, eve)
	switch {
	case err != nil:
		return nil, err
	case tx.Count == 0:
		return nil, fmt.Errorf("no transactions returned with query")
	case tx.Count > 1:
		return nil, fmt.Errorf("more than one transaction returned with query")
	}

	rlyPackets, err := relayPacketFromQueryResponse(tx.Txs[0])
	switch {
	case err != nil:
		return nil, err
	case len(rlyPackets) == 0:
		return nil, fmt.Errorf("no relay msgs created from query response")
	case len(rlyPackets) > 1:
		return nil, fmt.Errorf("more than one relay msg found in tx query")
	}

	// sanity check the sequence number against the one we are querying for
	// TODO: move this into relayPacketFromQueryResponse?
	if seq != rlyPackets[0].Seq() {
		return nil, fmt.Errorf("Different sequence number from query (%d vs %d)", seq, rlyPackets[0].Seq())
	}

	// fetch the proof from the sending chain
	if err = rlyPackets[0].FetchCommitResponse(dst, src, sh); err != nil {
		return nil, err
	}

	// return the sending msg
	return rlyPackets[0].Msg(dst, src), nil
}

// relayPacketFromQueryResponse looks through the events in a sdk.Response
// and returns relayPackets with the appropriate data
func relayPacketFromQueryResponse(res sdk.TxResponse) (rlyPackets []relayPacket, err error) {
	for _, l := range res.Logs {
		for _, e := range l.Events {
			if e.Type == "send_packet" {
				rp := &relayMsgRecvPacket{}
				for _, p := range e.Attributes {
					if p.Key == "packet_data" {
						rp.packetData = []byte(p.Value)
					}
					if p.Key == "packet_timeout" {
						timeout, _ := strconv.ParseUint(p.Value, 10, 64)
						rp.timeout = timeout
					}
					if p.Key == "packet_sequence" {
						seq, _ := strconv.ParseUint(p.Value, 10, 64)
						rp.seq = seq
					}
				}
				rlyPackets = append(rlyPackets, rp)
			}
		}
	}

	if len(rlyPackets) > 0 {
		return
	}

	return nil, fmt.Errorf("no packet data found")
}
