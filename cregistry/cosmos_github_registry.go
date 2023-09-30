package cregistry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v43/github"
	"go.uber.org/zap"
)

// CosmosGithubRegistry provides an API for interacting with the Cosmos chain registry.
// See: https://github.com/cosmos/chain-registry
type CosmosGithubRegistry struct {
	log *zap.Logger
}

// NewCosmosGithubRegistry initializes a new instance of CosmosGithubRegistry.
func NewCosmosGithubRegistry(log *zap.Logger) CosmosGithubRegistry {
	return CosmosGithubRegistry{log: log}
}

// ListChains attempts to connect to GitHub and get the tree object for the cosmos chain registry.
// It then builds a slice of chain names using the entries in the tree.
func (c CosmosGithubRegistry) ListChains(ctx context.Context) ([]string, error) {
	client := github.NewClient(http.DefaultClient)
	var chains []string

	ctx, cancel := context.WithTimeout(ctx, time.Minute*5)
	defer cancel()

	tree, res, err := client.Git.GetTree(
		ctx,
		"cosmos",
		"chain-registry",
		"master",
		false)
	if err != nil || res.StatusCode != 200 {
		return chains, err
	}

	for _, entry := range tree.Entries {
		if *entry.Type == "tree" && !strings.HasPrefix(*entry.Path, ".") {
			chains = append(chains, *entry.Path)
		}
	}
	return chains, nil
}

// GetChain attempts to fetch ChainInfo for the specified chain name from the cosmos chain registry.
func (c CosmosGithubRegistry) GetChain(ctx context.Context, testnet bool, name string) (ChainInfo, error) {
	var chainRegURL string
	if testnet {
		chainRegURL = fmt.Sprintf("https://raw.githubusercontent.com/cosmos/chain-registry/master/testnets/%s/chain.json", name)
	} else {
		chainRegURL = fmt.Sprintf("https://raw.githubusercontent.com/cosmos/chain-registry/master/%s/chain.json", name)
	}

	res, err := http.Get(chainRegURL)
	if err != nil {
		return ChainInfo{}, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return ChainInfo{}, fmt.Errorf("chain not found on registry: response code: %d: GET failed: %s", res.StatusCode, chainRegURL)
	}
	if res.StatusCode != http.StatusOK {
		return ChainInfo{}, fmt.Errorf("response code: %d: GET failed: %s", res.StatusCode, chainRegURL)
	}

	result := NewChainInfo(c.log.With(zap.String("chain_name", name)))
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return ChainInfo{}, err
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return ChainInfo{}, err
	}
	return result, nil
}

// SourceLink returns the string representation of the cosmos chain registry URL.
func (c CosmosGithubRegistry) SourceLink() string {
	return "https://github.com/cosmos/chain-registry"
}
