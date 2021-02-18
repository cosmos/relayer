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
