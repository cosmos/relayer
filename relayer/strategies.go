package relayer

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	chanState "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/exported"
	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint/types"
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
	Run(*Chain, *Chain) error
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
func (nrs NaiveStrategy) Run(src, dst *Chain) error {
	events := "tm.event = 'Tx'"

	srcEvents, srcCancel, err := src.Subscribe(events)
	if err != nil {
		return err
	}
	defer srcCancel()
	src.Log(fmt.Sprintf("listening to events from %s...", src.ChainID))

	dstEvents, dstCancel, err := dst.Subscribe(events)
	if err != nil {
		return err
	}
	defer dstCancel()
	dst.Log(fmt.Sprintf("listening to events from %s...", dst.ChainID))

	done := trapSignal()
	defer close(done)

	for {
		select {
		case srcMsg := <-srcEvents:
			byt := src.parsePacketData(srcMsg.Events)
			if byt != nil {
				src.sendPacket(dst, byt)
			}
		case dstMsg := <-dstEvents:
			byt := src.parsePacketData(dstMsg.Events)
			if byt != nil {
				dst.sendPacket(src, byt)
			}
		default:
			time.Sleep(1 * time.Millisecond)
			// NOTE: This causes the for loop to run continuously and not to
			//  wait for messages before advancing. This allows for quick exit
		}

		// If there are msgs in the done channel, quit
		if len(done) > 0 {
			<-done
			fmt.Println("shutdown activated")
			break
		}
	}

	return nil
}

func (src *Chain) sendPacket(dst *Chain, xferPacket chanState.PacketDataI) {
	var (
		err          error
		hs           map[string]*tmclient.Header
		seqRecv      chanTypes.RecvResponse
		seqSend      uint64
		srcCommitRes CommitmentResponse
	)

	hs, err = UpdatesWithHeaders(src, dst)
	if err != nil {
		src.Error(err)
	}

	seqRecv, err = src.QueryNextSeqRecv(hs[src.ChainID].Height)
	if err != nil {
		src.Error(err)
	}

	for {
		seqSend, err = dst.QueryNextSeqSend(hs[dst.ChainID].Height)
		if err != nil {
			src.Error(err)
		}

		srcCommitRes, err = dst.QueryPacketCommitment(hs[dst.ChainID].Height-1, int64(seqSend-1))
		if err != nil {
			src.Error(err)
		}

		if srcCommitRes.Proof.Proof == nil {
			_, err = dst.ListenForNextBlock()
			if err != nil {
				src.Error(err)
			}
			hs, err = UpdatesWithHeaders(src, dst)
			if err != nil {
				src.Error(err)
			}
			continue
		} else {
			break
		}
	}

	//
	// Debugging by simply passing in the packet information that we know was sent earlier in the SendPacket
	// part of the command. In a real relayer, this would be a separate command that retrieved the packet
	// information from an indexing node
	txs := RelayMsgs{
		Src: []sdk.Msg{
			src.PathEnd.UpdateClient(hs[dst.ChainID], src.MustGetAddress()),
			dst.PathEnd.MsgRecvPacket(
				src.PathEnd,
				seqRecv.NextSequenceRecv,
				xferPacket,
				chanTypes.NewPacketResponse(
					dst.PathEnd.PortID,
					dst.PathEnd.ChannelID,
					seqSend-1,
					dst.PathEnd.NewPacket(
						dst.PathEnd,
						seqSend-1,
						xferPacket,
					),
					srcCommitRes.Proof.Proof,
					int64(srcCommitRes.ProofHeight),
				),
				src.MustGetAddress(),
			),
		},
		Dst: []sdk.Msg{},
	}

	txs.Send(src, dst)

}

func trapSignal() chan bool {
	sigCh := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		fmt.Println("Signal Recieved:", sig.String())
		close(sigCh)
		done <- true
	}()

	return done
}

func (src *Chain) parsePacketData(events map[string][]string) (out chanState.PacketDataI) {
	if val, ok := events["send_packet.packet_data"]; ok {
		err := src.Cdc.UnmarshalJSON([]byte(val[0]), &out)
		if err != nil {
			src.Error(err)
		}
		return
	}

	src.Log(fmt.Sprintf("[%s]@{%d} - actions(%s) hash(%s)",
		src.ChainID,
		getEventHeight(events),
		actions(events["message.action"]),
		events["tx.hash"][0]),
	)

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
