package icon

import (
	"fmt"
	"sync"
	"time"

	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

var (
// _ provider.ChainProvider = &IconProviderConfig{}
)

type IconProviderConfig struct {
	Key               string  `json:"key" yaml:"key"`
	ChainName         string  `json:"-" yaml:"-"`
	ChainID           string  `json:"chain-id" yaml:"chain-id"`
	RPCAddr           string  `json:"rpc-addr" yaml:"rpc-addr"`
	Timeout           string  `json:"timeout" yaml:"timeout"`
	IbcHostAddress    string  `json:"ibc_host_address,omitempty"`
	IbcHandlerAddress string  `json:"ibc_handler_address,omitempty"`
}

func (pp IconProviderConfig) Validate() error {
	if _, err := time.ParseDuration(pp.Timeout); err != nil {
		return fmt.Errorf("invalid Timeout: %w", err)
	}
	return nil
}

func (pp IconProviderConfig) NewProvider(log *zap.Logger, homepath string, debug bool, chainName string) (provider.ChainProvider, error) {
	if err := pp.Validate(); err != nil {
		return nil, err
	}
	return nil, nil
}

func ChainClientConfig(pcfg *IconProviderConfig) {

}

type IconProvider struct {
	log *zap.Logger

	PCfg IconProviderConfig

	txMu sync.Mutex

	metrics *processor.PrometheusMetrics
}

type IconIBCHeader struct {
}

func (h IconIBCHeader) Height() uint64 {
	return 0
}

func (h IconIBCHeader) ConsensusState() {
	return
}
