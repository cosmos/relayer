package cregistry

import (
	"context"

	"go.uber.org/zap"
)

// ChainRegistry is a slim interface that can be implemented to interact with a repository of chain info/metadata.
type ChainRegistry interface {
	GetChain(ctx context.Context, testnet bool, name string) (ChainInfo, error)
	ListChains(ctx context.Context) ([]string, error)
	SourceLink() string
}

// DefaultChainRegistry initializes a new instance of CosmosGithubRegistry for interacting with the Cosmos chain registry.
func DefaultChainRegistry(log *zap.Logger) ChainRegistry {
	return NewCosmosGithubRegistry(log.With(zap.String("registry", "cosmos_github")))
}
