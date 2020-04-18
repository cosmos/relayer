package relayer

import (
	"fmt"
	"strconv"
	"strings"

	retry "github.com/avast/retry-go"
	sdk "github.com/cosmos/cosmos-sdk/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint/types"
)

var (
	txEvents = "tm.event = 'Tx'"
	blEvents = "tm.event = 'NewBlock'"
)

// MustGetStrategy returns the strategy and panics on error
func (r *Path) MustGetStrategy() Strategy {
	strat, err := r.GetStrategy()
	if err != nil {
		panic(err)
	}
	return strat
}

// GetStrategy the strategy defined in the relay messages
func (r *Path) GetStrategy() (Strategy, error) {
	switch r.Strategy.Type {
	case NaiveStrategy{}.GetType():
		return NaiveStrategy{}.Init(r.Strategy)
	default:
		return nil, fmt.Errorf("invalid strategy: %s", r.Strategy.Type)
	}
}

// StrategyCfg defines which relaying strategy to take for a given path
type StrategyCfg struct {
	Type        string            `json:"type" yaml:"type"`
	Constraints map[string]string `json:"constraints,omitempty" yaml:"constraints,omitempty"`
}

// Strategy defines the interface that strategies must
type Strategy interface {
	// Used to initialize the strategy implemenation
	// and validate the data from the configuration
	Init(*StrategyCfg) (Strategy, error)

	// Used to return the configuration
	Cfg() *StrategyCfg

	// Used in constructing StrategyCfg
	GetType() string

	// Used in constructing StrategyCfg
	GetConstraints() map[string]string

	// Run starts the relayer
	// it returns a recieve only channel that is used to wait for
	// the strategy to be ready to relay, and a done channel
	// that shuts down the relayer when it is time to exit
	Run(*Chain, *Chain) (func(), error)
}

// NewNaiveStrategy Returns a new NaiveStrategy config
func NewNaiveStrategy() *StrategyCfg {
	return &StrategyCfg{
		Type: NaiveStrategy{}.GetType(),
	}
}

// NaiveStrategy is a relaying strategy where everything in a Path is relayed
type NaiveStrategy struct{}

// Init implements Strategy
func (nrs NaiveStrategy) Init(sc *StrategyCfg) (Strategy, error) {
	if sc.Type != nrs.GetType() {
		return nil, fmt.Errorf("wrong type")
	}
	if len(sc.Constraints) != len(nrs.GetConstraints()) {
		return nil, fmt.Errorf("invalid constraint")
	}
	return nrs, nil
}

// Cfg implements Strategy
func (nrs NaiveStrategy) Cfg() *StrategyCfg {
	return &StrategyCfg{
		Type:        nrs.GetType(),
		Constraints: nrs.GetConstraints(),
	}
}

// GetType implements Strategy
func (nrs NaiveStrategy) GetType() string {
	return "naive"
}

// GetConstraints implements Strategy
func (nrs NaiveStrategy) GetConstraints() map[string]string {
	return map[string]string{}
}

// Run implements Strategy and defines what actions are taken when the relayer runs
func (nrs NaiveStrategy) Run(src, dst *Chain) (func(), error) {
	doneChan := make(chan struct{})

	// first, we want to ensure that there are no packets remaining to be relayed
	if err := RelayUnRelayedPacketsOrderedChan(src, dst); err != nil {
		// TODO: some errors may leak here when there are no packets to be relayed
		// be on the lookout for that
		return nil, err
	}

	go nrsLoop(src, dst, doneChan)

	return func() { doneChan <- struct{}{} }, nil
}

func nrsLoop(src, dst *Chain, doneChan chan struct{}) {
	// Subscribe to source chain
	if err := src.Start(); err != nil {
		src.Error(err)
		return
	}

	srcTxEvents, srcTxCancel, err := src.Subscribe(txEvents)
	if err != nil {
		src.Error(err)
		return
	}
	defer srcTxCancel()
	src.Log(fmt.Sprintf("- listening to tx events from %s...", src.ChainID))

	srcBlockEvents, srcBlockCancel, err := src.Subscribe(blEvents)
	if err != nil {
		src.Error(err)
		return
	}
	defer srcBlockCancel()
	src.Log(fmt.Sprintf("- listening to block events from %s...", src.ChainID))

	// Subscribe to destination chain
	if err := dst.Start(); err != nil {
		dst.Error(err)
		return
	}

	dstTxEvents, dstTxCancel, err := dst.Subscribe(txEvents)
	if err != nil {
		dst.Error(err)
		return
	}
	defer dstTxCancel()
	dst.Log(fmt.Sprintf("- listening to tx events from %s...", dst.ChainID))

	dstBlockEvents, dstBlockCancel, err := dst.Subscribe(blEvents)
	if err != nil {
		src.Error(err)
		return
	}
	defer dstBlockCancel()
	dst.Log(fmt.Sprintf("- listening to block events from %s...", dst.ChainID))

	// Listen to channels and take appropriate action
	for {
		select {
		case srcMsg := <-srcTxEvents:
			src.logTx(srcMsg.Events)
			go dst.handlePacket(src, srcMsg.Events)
		case dstMsg := <-dstTxEvents:
			dst.logTx(dstMsg.Events)
			go src.handlePacket(dst, dstMsg.Events)
		case srcMsg := <-srcBlockEvents:
			go dst.handlePacket(src, srcMsg.Events)
		case dstMsg := <-dstBlockEvents:
			go src.handlePacket(dst, dstMsg.Events)
		case <-doneChan:
			src.Log(fmt.Sprintf("- [%s]:{%s} <-> [%s]:{%s} relayer shutting down",
				src.ChainID, src.PathEnd.PortID, dst.ChainID, dst.PathEnd.PortID))
			close(doneChan)
			return
		}
	}
}

func (src *Chain) handlePacket(dst *Chain, events map[string][]string) {
	byt, seq, timeout, err := src.packetDataAndTimeoutFromEvent(dst, events)
	if byt != nil && seq != 0 && err == nil {
		src.sendPacketFromEvent(dst, byt, seq, timeout)
		return
	} else if err != nil {
		src.Error(err)
	}
}

// TODO: rewrite this function to take a relayPacket
func (src *Chain) sendPacketFromEvent(dst *Chain, xferPacket []byte, seq int64, timeout uint64) {
	var (
		err          error
		dstH         *tmclient.Header
		dstCommitRes CommitmentResponse
	)

	if err = retry.Do(func() error {
		dstH, err = dst.UpdateLiteWithHeader()
		if err != nil {
			return err
		}
		dstCommitRes, err = dst.QueryPacketCommitment(dstH.Height-1, int64(seq))
		if err != nil {
			return err
		} else if dstCommitRes.Proof.Proof == nil {
			return fmt.Errorf("- [%s]@{%d} - Packet Commitment Proof is nil seq(%d)", dst.ChainID, dstH.Height-1, seq)
		}
		return nil
	}); err != nil {
		dst.Error(err)
		return
	}

	txs := &RelayMsgs{
		Src: []sdk.Msg{
			src.PathEnd.UpdateClient(dstH, src.MustGetAddress()),
			src.PathEnd.MsgRecvPacket(
				dst.PathEnd,
				uint64(seq),
				timeout,
				xferPacket,
				dstCommitRes.Proof,
				dstCommitRes.ProofHeight,
				src.MustGetAddress(),
			),
		},
		Dst: []sdk.Msg{},
	}

	// for _, pkt := range rlyPackets {
	// 	txs.Src = append(txs.Src, pkt.Msg(src, dst, dstCommitRes.Proof, dstCommitRes.ProofHeight))
	// }
	txs.Send(src, dst)
}

// TODO: rewrite this function to return a relayPacket and also return relay packets for acks and multiple msg trasactions
func (src *Chain) packetDataAndTimeoutFromEvent(dst *Chain, events map[string][]string) (packetData []byte, seq int64, timeout uint64, err error) {

	// then, get packet data and parse
	if pdval, ok := events["send_packet.packet_data"]; ok {
		packetData = []byte(pdval[0])
	}

	// next, get and parse the sequence
	if sval, ok := events["send_packet.packet_sequence"]; ok {
		seq, err = strconv.ParseInt(sval[0], 10, 64)
		if err != nil {
			return nil, 0, 0, err
		}
	}

	// finally, get and parse the timeout
	if sval, ok := events["send_packet.packet_timeout"]; ok {
		timeout, err = strconv.ParseUint(sval[0], 10, 64)
		if err != nil {
			return nil, 0, 0, err
		}
	}

	return
}

type relayPacket interface {
	Msg(src, dst *Chain) sdk.Msg
	FetchCommitResponse(src, dst *Chain)
}

type relayMsgRecvPacket struct {
	packetData []byte
	seq        uint64
	timeout    uint64
	dstComRes  *CommitmentResponse
}

func (rp *relayMsgRecvPacket) FetchCommitResponse(src, dst *Chain) {
	var (
		err          error
		dstH         *tmclient.Header
		dstCommitRes CommitmentResponse
	)
	if err = retry.Do(func() error {
		dstH, err = dst.UpdateLiteWithHeader()
		if err != nil {
			return err
		}
		dstCommitRes, err = dst.QueryPacketCommitment(dstH.Height-1, int64(rp.seq))
		if err != nil {
			return err
		} else if dstCommitRes.Proof.Proof == nil {
			return fmt.Errorf("- [%s]@{%d} - Packet Commitment Proof is nil seq(%d)", dst.ChainID, dstH.Height-1, rp.seq)
		}
		return nil
	}); err != nil {
		dst.Error(err)
		return
	}
	rp.dstComRes = &dstCommitRes
}

func (rp *relayMsgRecvPacket) Msg(src, dst *Chain) sdk.Msg {
	if rp.dstComRes == nil {
		return nil
	}
	return src.PathEnd.MsgRecvPacket(
		dst.PathEnd,
		rp.seq,
		rp.timeout,
		rp.packetData,
		rp.dstComRes.Proof,
		rp.dstComRes.ProofHeight,
		src.MustGetAddress(),
	)
}

type relayMsgPacketAck struct {
	ack       []byte
	seq       uint64
	timeout   uint64
	dstComRes *CommitmentResponse
}

func (rp *relayMsgPacketAck) Msg(src, dst *Chain) sdk.Msg {
	return src.PathEnd.MsgAck(
		dst.PathEnd,
		rp.seq,
		rp.timeout,
		rp.ack,
		rp.dstComRes.Proof,
		rp.dstComRes.ProofHeight,
		src.MustGetAddress(),
	)
}

func (rp *relayMsgPacketAck) FetchCommitResponse(src, dst *Chain) {
	var (
		err          error
		dstH         *tmclient.Header
		dstCommitRes CommitmentResponse
	)
	if err = retry.Do(func() error {
		dstH, err = dst.UpdateLiteWithHeader()
		if err != nil {
			return err
		}
		dstCommitRes, err = dst.QueryPacketAck(dstH.Height-1, int64(rp.seq))
		if err != nil {
			return err
		} else if dstCommitRes.Proof.Proof == nil {
			return fmt.Errorf("- [%s]@{%d} - Packet Ack Proof is nil seq(%d)", dst.ChainID, dstH.Height-1, rp.seq)
		}
		return nil
	}); err != nil {
		dst.Error(err)
		return
	}
	rp.dstComRes = &dstCommitRes
}

func relayPacketsFromEvent(events map[string][]string) (rlyPkts []relayPacket, err error) {
	// check for send packets
	if pdval, ok := events["send_packet.packet_data"]; ok {
		for i, pd := range pdval {
			rp := &relayMsgRecvPacket{packetData: []byte(pd)}
			// next, get and parse the sequence
			if sval, ok := events["send_packet.packet_sequence"]; ok {
				seq, err := strconv.ParseUint(sval[i], 10, 64)
				if err != nil {
					return nil, err
				}
				rp.seq = seq
			}

			// finally, get and parse the timeout
			if sval, ok := events["send_packet.packet_timeout"]; ok {
				timeout, err := strconv.ParseUint(sval[i], 10, 64)
				if err != nil {
					return nil, err
				}
				rp.timeout = timeout
			}
			rlyPkts = append(rlyPkts, rp)
		}
	}

	// then, check for recv packets
	if pdval, ok := events["recv_packet.packet_data"]; ok {
		for i, pd := range pdval {
			rp := &relayMsgPacketAck{ack: []byte(pd)}
			// next, get and parse the sequence
			if sval, ok := events["recv_packet.packet_sequence"]; ok {
				seq, err := strconv.ParseUint(sval[i], 10, 64)
				if err != nil {
					return nil, err
				}
				rp.seq = seq
			}

			// finally, get and parse the timeout
			if sval, ok := events["recv_packet.packet_timeout"]; ok {
				timeout, err := strconv.ParseUint(sval[i], 10, 64)
				if err != nil {
					return nil, err
				}
				rp.timeout = timeout
			}
			rlyPkts = append(rlyPkts, rp)
		}
	}
	return
}

func getEventHeight(events map[string][]string) int64 {
	if val, ok := events["tx.height"]; ok {
		out, _ := strconv.ParseInt(val[0], 10, 64)
		return out
	}
	return -1
}

func actions(act []string) string {
	out := ""
	for i, a := range act {
		out += fmt.Sprintf("%d:%s,", i, a)
	}
	return strings.TrimSuffix(out, ",")
}
