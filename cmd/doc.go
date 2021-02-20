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

// swagger:parameters getChain addChain
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
// Add a chain
// responses:
//   201: addChainResponse
//   400: errorResponse
//   500: errorResponse

// swagger:parameters addChain
type addChainParamsWrapper struct {
	// in:body
	Body addChainRequest
}

// swagger:response addChainResponse
type addChainResWrapper struct {
	// in:body
	Res string
}
