package cregistry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"time"

	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// AssetList describes the various chain asset metadata found in the cosmos chain registry.
type AssetList struct {
	Schema  string `json:"$schema"`
	ChainID string `json:"chain_id"`
	Assets  []struct {
		Description string `json:"description"`
		DenomUnits  []struct {
			Denom    string `json:"denom"`
			Exponent int    `json:"exponent"`
		} `json:"denom_units"`
		Base     string `json:"base"`
		Name     string `json:"name"`
		Display  string `json:"display"`
		Symbol   string `json:"symbol"`
		LogoURIs struct {
			Png string `json:"png"`
			Svg string `json:"svg"`
		} `json:"logo_URIs"`
		CoingeckoID string `json:"coingecko_id"`
	} `json:"assets"`
}

// ChainInfo describes the canonical chain metadata found in the cosmos chain registry.
type ChainInfo struct {
	log *zap.Logger

	Schema       string `json:"$schema"`
	ChainName    string `json:"chain_name"`
	Status       string `json:"status"`
	NetworkType  string `json:"network_type"`
	PrettyName   string `json:"pretty_name"`
	ChainID      string `json:"chain_id"`
	Bech32Prefix string `json:"bech32_prefix"`
	DaemonName   string `json:"daemon_name"`
	NodeHome     string `json:"node_home"`
	Genesis      struct {
		GenesisURL string `json:"genesis_url"`
	} `json:"genesis"`
	Slip44           *int   `json:"slip44"`
	SigningAlgorithm string `json:"signing-algorithm"`
	Codebase         struct {
		GitRepo            string   `json:"git_repo"`
		RecommendedVersion string   `json:"recommended_version"`
		CompatibleVersions []string `json:"compatible_versions"`
	} `json:"codebase"`
	Peers struct {
		Seeds []struct {
			ID       string `json:"id"`
			Address  string `json:"address"`
			Provider string `json:"provider,omitempty"`
		} `json:"seeds"`
		PersistentPeers []struct {
			ID      string `json:"id"`
			Address string `json:"address"`
		} `json:"persistent_peers"`
	} `json:"peers"`
	Apis struct {
		RPC []struct {
			Address  string `json:"address"`
			Provider string `json:"provider"`
		} `json:"rpc"`
		Rest []struct {
			Address  string `json:"address"`
			Provider string `json:"provider"`
		} `json:"rest"`
	} `json:"apis"`
	MaxGasAmount     uint64                     `json:"max_gas_amount"`
	ExtraCodecs      []string                   `json:"extra_codecs"`
	ExtensionOptions []provider.ExtensionOption `json:"extension_options"`
}

// NewChainInfo returns a ChainInfo that is uninitialized other than the provided zap.Logger.
// Typically, the caller will unmarshal JSON content into the ChainInfo after initialization.
func NewChainInfo(log *zap.Logger) ChainInfo {
	return ChainInfo{log: log}
}

// GetAllRPCEndpoints returns a slice of strings representing the available RPC endpoints found in the
// cosmos chain registry for this particular chain.
func (c ChainInfo) GetAllRPCEndpoints() (out []string, err error) {
	for _, endpoint := range c.Apis.RPC {
		u, err := url.Parse(endpoint.Address)
		if err != nil {
			return nil, err
		}

		var port string
		if u.Port() == "" {
			switch u.Scheme {
			case "https":
				port = "443"
			case "http":
				port = "80"
			default:
				return nil, fmt.Errorf("invalid or unsupported url scheme: %v", u.Scheme)
			}
		} else {
			port = u.Port()
		}

		out = append(out, fmt.Sprintf("%s://%s:%s%s", u.Scheme, u.Hostname(), port, u.Path))
	}

	return
}

// IsHealthyRPC returns an error if the specified endpoint is not caught up with the current chain tip.
// Otherwise it returns nil.
func IsHealthyRPC(ctx context.Context, endpoint string) error {
	cl, err := cosmos.NewRPCClient(endpoint, 5*time.Second)
	if err != nil {
		return err
	}
	stat, err := cl.Status(ctx)
	if err != nil {
		return err
	}

	if stat.SyncInfo.CatchingUp {
		return errors.New("still catching up")
	}

	return nil
}

// GetRPCEndpoints returns a slice of strings representing the healthy available RPC endpoints found in the
// cosmos chain registry for this particular chain.
func (c ChainInfo) GetRPCEndpoints(ctx context.Context) (out []string, err error) {
	allRPCEndpoints, err := c.GetAllRPCEndpoints()
	if err != nil {
		return nil, err
	}

	var eg errgroup.Group
	var endpoints []string
	healthy := 0
	unhealthy := 0
	for _, endpoint := range allRPCEndpoints {
		endpoint := endpoint
		eg.Go(func() error {
			err := IsHealthyRPC(ctx, endpoint)
			if err != nil {
				unhealthy += 1
				c.log.Debug(
					"Ignoring endpoint due to error",
					zap.String("endpoint", endpoint),
					zap.Error(err),
				)
				return nil
			}
			healthy += 1
			c.log.Debug("Verified healthy endpoint", zap.String("endpoint", endpoint))
			endpoints = append(endpoints, endpoint)
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	c.log.Info("Endpoints queried",
		zap.String("chain_name", c.ChainName),
		zap.Int("healthy", healthy),
		zap.Int("unhealthy", unhealthy),
	)
	return endpoints, nil
}

// GetRandomRPCEndpoint returns a string representing a random RPC endpoint from the cosmos chain registry for this chain.
func (c ChainInfo) GetRandomRPCEndpoint(ctx context.Context, forceAdd bool) (string, error) {
	rpcs, err := c.GetRPCEndpoints(ctx)
	if err != nil {
		return "", err
	}

	if len(rpcs) == 0 {
		if !forceAdd {
			return "", fmt.Errorf("no working RPCs found, consider using --force-add")
		} else {
			return "", nil
		}
	}

	randomGenerator := rand.New(rand.NewSource(time.Now().UnixNano()))
	endpoint := rpcs[randomGenerator.Intn(len(rpcs))]
	c.log.Info("Endpoint selected",
		zap.String("chain_name", c.ChainName),
		zap.String("endpoint", endpoint),
	)
	return endpoint, nil
}

// GetAssetList returns the asset metadata from the cosmos chain registry for this particular chain.
func (c ChainInfo) GetAssetList(ctx context.Context, testnet bool, name string) (AssetList, error) {
	var chainRegURL string
	if testnet {
		chainRegURL = fmt.Sprintf("https://raw.githubusercontent.com/cosmos/chain-registry/master/testnets/%s/assetlist.json", name)

	} else {
		chainRegURL = fmt.Sprintf("https://raw.githubusercontent.com/cosmos/chain-registry/master/%s/assetlist.json", name)

	}
	res, err := http.Get(chainRegURL)
	if err != nil {
		return AssetList{}, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return AssetList{}, fmt.Errorf("chain not found on registry: response code: %d: GET failed: %s", res.StatusCode, chainRegURL)
	}
	if res.StatusCode != http.StatusOK {
		return AssetList{}, fmt.Errorf("response code: %d: GET failed: %s", res.StatusCode, chainRegURL)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return AssetList{}, err
	}

	var assetList AssetList
	if err := json.Unmarshal(body, &assetList); err != nil {
		return AssetList{}, err
	}
	return assetList, nil

}

// GetChainConfig returns a CosmosProviderConfig composed from the details found in the cosmos chain registry for
// this particular chain.
func (c ChainInfo) GetChainConfig(ctx context.Context, forceAdd, testnet bool, name string) (*cosmos.CosmosProviderConfig, error) {
	debug := viper.GetBool("debug")
	home := viper.GetString("home")

	assetList, err := c.GetAssetList(ctx, testnet, name)
	if err != nil {
		return nil, err
	}

	var gasPrices string
	if len(assetList.Assets) > 0 {
		gasPrices = fmt.Sprintf("%.2f%s", 0.01, assetList.Assets[0].Base)
	}

	rpc, err := c.GetRandomRPCEndpoint(ctx, forceAdd)
	if err != nil {
		return nil, err
	}

	return &cosmos.CosmosProviderConfig{
		Key:              "default",
		ChainID:          c.ChainID,
		RPCAddr:          rpc,
		AccountPrefix:    c.Bech32Prefix,
		KeyringBackend:   "test",
		GasAdjustment:    1.2,
		GasPrices:        gasPrices,
		KeyDirectory:     home,
		Debug:            debug,
		Timeout:          "20s",
		OutputFormat:     "json",
		SignModeStr:      "direct",
		Slip44:           c.Slip44,
		SigningAlgorithm: c.SigningAlgorithm,
		ExtraCodecs:      c.ExtraCodecs,
		MaxGasAmount:     c.MaxGasAmount,
		ExtensionOptions: c.ExtensionOptions,
	}, nil
}
