// Package cmd Relayer Rest Server.
//
// A REST interface for state queries.
//
//     Schemes: http
//	   Basepath: /
//     Version: 1.0.0
//     Host: localhost:5183
//
//     Consumes:
//     - application/json
//
//     Produces:
//     - application/json
//
//
// swagger:meta
package cmd

import (
	"github.com/cosmos/relayer/relayer"
)

// swagger:response errorResponse
type errResWrapper struct {
	// in:body
	Error struct {
		Err string `json:"err" yaml:"err"`
	}
}

// swagger:route GET /version Version version
// Get version.
// responses:
//   200: versionResponse

// swagger:response versionResponse
type versionResWrapper struct {
	// in:body
	VersionInfo versionInfo
}

// swagger:route GET /config Config config
// Get config.
// responses:
//   200: configResponse

// swagger:response configResponse
type configResWrapper struct {
	// Returns config of relayer
	// in:body
	Config *Config
}

// swagger:route GET /chains Chains getChains
// Get chains list.
// responses:
//   200: getChainsResponse

// swagger:response getChainsResponse
type getChainsResWrapper struct {
	// Returns chains list.
	// in:body
	Chains relayer.Chains
}

// swagger:route GET /chains/{name} Chains getChain
// Get single chain details.
// responses:
//   200: getChainResponse
//   400: errorResponse

// swagger:parameters getChain addChain updateChain deleteChain getChainStatus
type chainParamsWrapper struct {
	// in:path
	Name string `json:"name" yaml:"name"`
}

// swagger:response getChainResponse
type getChainResWrapper struct {
	// Returns chain details
	// in:body
	Chain *relayer.Chain
}

// swagger:route POST /chains/{name} Chains addChain
// Add a chain.
//
// file and url parameters in body are optional and can't use both at once.
// responses:
//   201: chainResponse
//   400: errorResponse
//   500: errorResponse

// swagger:parameters addChain
type addChainParamsWrapper struct {
	// required:true
	// in:body
	Body addChainRequest `json:"body" yaml:"body"`
}

// swagger:response chainResponse
type chainResWrapper struct {
	// in:body
	Res string `json:"res" yaml:"res"`
}

// swagger:route PUT /chains/{name} Chains updateChain
// Update chain config values.
// responses:
//   200: chainResponse
//   400: errorResponse
//   500: errorResponse

// swagger:parameters updateChain
type updateChainParamsWrapper struct {
	// required:true
	// in:body
	Body editChainRequest `json:"body" yaml:"body"`
}

// swagger:route DELETE /chains/{name} Chains deleteChain
// Delete Chain.
// responses:
//   200: chainResponse
//   400: errorResponse
//   500: errorResponse

// swagger:route GET /chains/{name}/status Chains getChainStatus
// Get status of a chain.
// responses:
//   200: chainStatusRes
//   400: errorResponse

// swagger:response chainStatusRes
type chainStatusResWrapper struct {
	// in:body
	Status chainStatusResponse
}
