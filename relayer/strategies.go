package relayer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
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

	dstEvents, dstCancel, err := dst.Subscribe(events)
	if err != nil {
		return err
	}
	defer dstCancel()

	done := trapSignal()
	defer close(done)

	for {
		select {
		case srcMsg := <-srcEvents:
			byt, err := json.Marshal(srcMsg.Events)
			if err != nil {
				src.Error(err)
			}
			src.Log(string(byt))
		case dstMsg := <-dstEvents:
			byt, err := json.Marshal(dstMsg.Events)
			if err != nil {
				dst.Error(err)
			}
			dst.Log(string(byt))
		default:
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
