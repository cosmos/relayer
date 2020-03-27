package relayer

import (
	"fmt"
	"strconv"
	"strings"

	retry "github.com/avast/retry-go"
	sdk "github.com/cosmos/cosmos-sdk/types"
	chanState "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/exported"
	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint/types"
)

var (
	txEvents = "tm.event = 'Tx'"
	// blEvents = "tm.event = 'NewBlock'"
)

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
	Run(*Chain, *Chain) (chan<- struct{}, error)
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
func (nrs NaiveStrategy) Run(src, dst *Chain) (chan<- struct{}, error) {
	doneChan := make(chan struct{})

	// first, we want to ensure that there are no packets remaining to be relayed
	if err := RelayUnRelayedPacketsOrderedChan(src, dst); err != nil {
		// TODO: better error handling here
		return nil, err
	}

	go nrsLoop(src, dst, doneChan)

	return doneChan, nil
}

func nrsLoop(src, dst *Chain, doneChan chan struct{}) {
	srcEvents, srcCancel, err := src.Subscribe(txEvents)
	if err != nil {
		src.Error(err)
		return
	}
	defer srcCancel()
	src.Log(fmt.Sprintf("- listening to events from %s...", src.ChainID))

	dstEvents, dstCancel, err := dst.Subscribe(txEvents)
	if err != nil {
		dst.Error(err)
		return
	}
	defer dstCancel()
	dst.Log(fmt.Sprintf("- listening to events from %s...", dst.ChainID))

	for {
		select {
		case srcMsg := <-srcEvents:
			go dst.handlePacket(src, srcMsg.Events)
		case dstMsg := <-dstEvents:
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
	byt, seq, err := src.parsePacketData(dst, events)
	if byt != nil && seq != 0 && err == nil {
		src.sendPacket(dst, byt, seq)
	} else if err != nil {
		src.Error(err)
	}
}

func (src *Chain) sendPacket(dst *Chain, xferPacket chanState.PacketDataI, seq int64) {
	var (
		err          error
		dstH         *tmclient.Header
		dstCommitRes CommitmentResponse
	)

	err = dst.WaitForNBlocks(2)
	if err != nil {
		dst.Error(err)
	}

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
				xferPacket,
				chanTypes.NewPacketResponse(
					dst.PathEnd.PortID,
					dst.PathEnd.ChannelID,
					uint64(seq),
					dst.PathEnd.NewPacket(
						src.PathEnd,
						uint64(seq),
						xferPacket,
					),
					dstCommitRes.Proof.Proof,
					int64(dstCommitRes.ProofHeight),
				),
				src.MustGetAddress(),
			),
		},
		Dst: []sdk.Msg{},
	}
	txs.Send(src, dst)
}

func (src *Chain) parsePacketData(dst *Chain, events map[string][]string) (packetData chanState.PacketDataI, seq int64, err error) {
	// first, we log the actions and msg hash
	src.logTx(events)

	// Set sdk config to use custom Bech32 account prefix
	sdkConf := sdk.GetConfig()
	sdkConf.SetBech32PrefixForAccount(dst.AccountPrefix, dst.AccountPrefix+"pub")

	// then, get packet data and parse
	if pdval, ok := events["send_packet.packet_data"]; ok {
		err = src.Cdc.UnmarshalJSON([]byte(pdval[0]), &packetData)
		if err != nil {
			return nil, 0, err
		}
	}

	// finally, get and parse the sequence
	if sval, ok := events["send_packet.packet_sequence"]; ok {
		seq, err = strconv.ParseInt(sval[0], 10, 64)
		if err != nil {
			return nil, 0, err
		}
	}
	return
}

func (src *Chain) logTx(events map[string][]string) {
	src.Log(fmt.Sprintf("• [%s]@{%d} - actions(%s) hash(%s)",
		src.ChainID,
		getEventHeight(events),
		actions(events["message.action"]),
		events["tx.hash"][0]),
	)
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
