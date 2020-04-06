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
	defaultChainPrefix   = commitmentypes.NewMerklePrefix([]byte("ibc"))
	defaultIBCVersion    = "1.0.0"
	defaultIBCVersions   = []string{defaultIBCVersion}
	defaultUnbondingTime = time.Hour * 504 // 3 weeks in hours
	defaultPacketTimeout = 1000
	defaultPacketQuery   = "send_packet.packet_src_channel=%s&send_packet.packet_sequence=%d"
)

// RelayUnRelayedPacketsOrderedChan creates transactions to clear both queues
func RelayUnRelayedPacketsOrderedChan(src, dst *Chain) error {
	// Update lite clients, headers to be used later
	hs, err := UpdatesWithHeaders(src, dst)
	if err != nil {
		return err
	}

	// find any unrelayed packets
	sp, err := UnrelayedSequences(src, dst, hs[src.ChainID].Height-1, hs[dst.ChainID].Height-1)
	if err != nil {
		return err
	}

	// create the appropriate update client messages
	msgs := &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}}
	if len(sp.Src) > 0 {
		msgs.Dst = append(msgs.Dst, dst.PathEnd.UpdateClient(hs[src.ChainID], dst.MustGetAddress()))
	}
	if len(sp.Dst) > 0 {
		msgs.Src = append(msgs.Src, src.PathEnd.UpdateClient(hs[dst.ChainID], src.MustGetAddress()))
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

	// MsgTransfer will call SendPacket on src chain
	txs := RelayMsgs{
		Src: []sdk.Msg{src.PathEnd.MsgTransfer(dst.PathEnd, dstHeader.GetHeight(), sdk.NewCoins(amount), dstAddr, src.MustGetAddress())},
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

	// reconstructing packet data here instead of retrieving from an indexed node
	xferPacket := src.PathEnd.XferPacket(
		sdk.NewCoins(amount),
		src.MustGetAddress(),
		dstAddr,
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
				chanTypes.NewPacketResponse(
					src.PathEnd.PortID,
					src.PathEnd.ChannelID,
					seqSend-1,
					src.PathEnd.NewPacket(
						dst.PathEnd,
						seqSend-1,
						xferPacket,
						timeoutHeight,
					),
					srcCommitRes.Proof.Proof,
					int64(srcCommitRes.ProofHeight),
				),
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

	// MsgTransfer will call SendPacket on src chain
	txs := RelayMsgs{
		Src: []sdk.Msg{src.PathEnd.MsgTransfer(dst.PathEnd, dstHeader.GetHeight(), sdk.NewCoins(amount), dstAddr, src.MustGetAddress())},
		Dst: []sdk.Msg{},
	}

	if txs.Send(src, dst); !txs.success {
		return fmt.Errorf("failed to send transfer message")
	}
	return nil
}

func addPacketMsg(src, dst *Chain, srcH, dstH *tmclient.Header, seq uint64, msgs *RelayMsgs, source bool) error {
	eve, err := ParseEvents(fmt.Sprintf(defaultPacketQuery, src.PathEnd.ChannelID, seq))
	if err != nil {
		return err
	}

	tx, err := src.QueryTxs(srcH.GetHeight(), 1, 1000, eve)
	switch {
	case err != nil:
		return err
	case tx.Count == 0:
		return fmt.Errorf("no transactions returned with query")
	case tx.Count > 1:
		return fmt.Errorf("more than one transaction returned with query")
	}

	pd, to, qSeq, err := src.packetDataAndTimeoutFromQueryResponse(src, tx.Txs[0])
	if err != nil {
		return err
	}

	if seq != qSeq {
		return fmt.Errorf("Different sequence number from query (%d vs %d)", seq, qSeq)
	}

	var (
		srcCommitRes CommitmentResponse
	)

	if err = retry.Do(func() error {
		srcCommitRes, err = src.QueryPacketCommitment(srcH.Height-1, int64(seq))
		if err != nil {
			return err
		} else if srcCommitRes.Proof.Proof == nil {
			return fmt.Errorf("[%s]@{%d} - Packet Commitment Proof is nil seq(%d)", src.ChainID, srcH.Height-1, seq)
		}
		return nil
	}); err != nil {
		return err
	}

	msg, err := dst.PacketMsg(src, pd, to, int64(seq), srcCommitRes)
	if err != nil {
		return err
	}

	if source {
		msgs.Dst = append(msgs.Dst, msg)
	} else {
		msgs.Src = append(msgs.Src, msg)
	}
	return nil
}

func (src *Chain) packetDataAndTimeoutFromQueryResponse(dst *Chain, res sdk.TxResponse) (packetData []byte, timeout uint64, seq uint64, err error) {
	// Set sdk config to use custom Bech32 account prefix
	sdkConf := sdk.GetConfig()
	sdkConf.SetBech32PrefixForAccount(dst.AccountPrefix, dst.AccountPrefix+"pub")

	for _, l := range res.Logs {
		for _, e := range l.Events {
			if e.Type == "send_packet" {
				for _, p := range e.Attributes {
					if p.Key == "packet_data" {
						packetData = []byte(p.Value)
					}
					if p.Key == "packet_timeout" {
						timeout, _ = strconv.ParseUint(p.Value, 10, 64)
					}
					if p.Key == "packet_sequence" {
						seq, _ = strconv.ParseUint(p.Value, 10, 64)

					}
				}
				if packetData != nil && timeout != 0 {
					return
				}
			}
		}
	}
	return nil, 0, 0, fmt.Errorf("no packet data found")
}
