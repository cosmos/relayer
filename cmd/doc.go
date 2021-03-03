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
// nolint
package cmd

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	transfertypes "github.com/cosmos/cosmos-sdk/x/ibc/applications/transfer/types"
	clienttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/types"
	conntypes "github.com/cosmos/cosmos-sdk/x/ibc/core/03-connection/types"
	chantypes "github.com/cosmos/cosmos-sdk/x/ibc/core/04-channel/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/helpers"
	"github.com/cosmos/relayer/relayer"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
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

// swagger:parameters getChain addChain updateChain deleteChain getChainStatus
// swagger:parameters getPath addPath deletePath getPathStatus
// swagger:parameters getKey addKey deleteKey restoreKey
type nameParamsWrapper struct {
	// in:path
	Name string `json:"name" yaml:"name"`
}

// swagger:parameters getKeys getKey addKey deleteKey restoreKey
// swagger:parameters getLightHeader getLightHeight initLight updateLight deleteLight
// swagger:parameters queryAccount queryBalance queryHeader queryNodeState queryValSet
// swagger:parameters queryTxs queryTx queryClient queryClients queryConn queryConns
// swagger:parameters queryClientConns queryChan queryChans queryConnChans queryIBCDenoms
type chainIDParamsWrapper struct {
	// in:path
	ChainID string `json:"chain-id" yaml:"chain-id"`
}

// swagger:parameters getLightHeader queryHeader queryValSet queryClient queryConn queryClientConns queryChan
type heightParamsWrapper struct {
	// in:query
	Height int `json:"height" yaml:"height"`
}

// swagger:parameters queryTxs queryClients queryConns queryChans queryConnChans
type paginationParamsWrapper struct {
	// in:query
	Offset uint `json:"offset" yaml:"offset"`
	// in:query
	Limit uint `json:"limit" yaml:"limit"`
}

// swagger:response stringSuccessResponse
type stringResWrapper struct {
	// in:body
	Res string `json:"res" yaml:"res"`
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
//   200: body:getChainResponse Returns chain details
//   400: errorResponse

// swagger:response getChainResponse
type getChainResWrapper struct {
	// in:body
	Chain *relayer.Chain
}

// swagger:route POST /chains/{name} Chains addChain
// Add a chain.
//
// file and url parameters in body are optional and can't use both at once.
// responses:
//   201: body:stringSuccessResponse Returns success string
//   400: errorResponse
//   500: errorResponse

// swagger:parameters addChain
type addChainParamsWrapper struct {
	// required:true
	// in:body
	Body addChainRequest `json:"body" yaml:"body"`
}

// swagger:route PUT /chains/{name} Chains updateChain
// Update chain config values.
// responses:
//   200: body:stringSuccessResponse Returns success string
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
//   200: body:stringSuccessResponse Returns success string
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

// swagger:route GET /paths Paths getPaths
// Get paths list.
// responses:
//   200: getPathsResponse

// swagger:response getPathsResponse
type getPathsResWrapper struct {
	// Returns paths list.
	// in:body
	Paths relayer.Paths
}

// swagger:route GET /paths/{name} Paths getPath
// Get single path details.
// responses:
//   200: getPathResponse
//   400: errorResponse

// swagger:response getPathResponse
type getPathResWrapper struct {
	// in:body
	Path *relayer.Path
}

// swagger:route POST /paths/{name} Paths addPath
// Add a path.
//
// file parameter in body is optional and if given, it will considered first.
//
// responses:
//   201: body:stringSuccessResponse Returns success string
//   400: errorResponse
//   500: errorResponse

// swagger:parameters addPath
type addPathParamsWrapper struct {
	// required:true
	// in:body
	Body postPathRequest `json:"body" yaml:"body"`
}

// swagger:route DELETE /paths/{name} Paths deletePath
// Delete Path.
// responses:
//   200: body:stringSuccessResponse Returns success string
//   400: errorResponse
//   500: errorResponse

// swagger:route GET /paths/{name}/status Paths getPathStatus
// Get status of a path.
// responses:
//   200: pathStatusRes
//   400: errorResponse
//   500: errorResponse

// swagger:response pathStatusRes
type pathStatusResWrapper struct {
	// in:body
	Status *relayer.PathWithStatus
}

// swagger:route GET /keys/{chain-id} Keys getKeys
// Get keys list of a chain.
// responses:
//   200: getKeysResponse
//   400: errorResponse
//   500: errorResponse

// swagger:response getKeysResponse
type getKeysResWrapper struct {
	// in:body
	Keys []keyResponse
}

// swagger:route GET /keys/{chain-id}/{name} Keys getKey
// Get details of a key in a chain.
// responses:
//   200: getKeyResponse
//   400: errorResponse
//   404: errorResponse
//   500: errorResponse

// swagger:response getKeyResponse
type getKeyResWrapper struct {
	// in:body
	Key keyResponse
}

// swagger:route POST /keys/{chain-id}/{name} Keys addKey
// Add a key in a chain.
//
// coin-type is a query parameter (optional)
//
// responses:
//   201: keyCreatedResponse
//   400: errorResponse
//   500: errorResponse

// swagger:parameters addKey restoreKey
type addKeyParamsWrapper struct {
	// required:false
	// in:query
	CoinType int `json:"coin-type" yaml:"coin-type"`
}

// swagger:response keyCreatedResponse
type keyCreatedResWrapper struct {
	// in:body
	KeyOutput helpers.KeyOutput
}

// swagger:route POST /keys/{chain-id}/{name}/restore Keys restoreKey
// Restore a key using mnemonic.
//
// coin-type is a query parameter (optional)
//
// responses:
//   200: keyCreatedResponse
//   400: errorResponse
//   500: errorResponse

// swagger:parameters restoreKey
type restoreKeyParamsWrapper struct {
	// required:true
	// in:body
	Body restoreKeyRequest `json:"body" yaml:"body"`
}

// swagger:route DELETE /keys/{chain-id}/{name} Keys deleteKey
// Delete key in a chain.
// responses:
//   200: body:stringSuccessResponse Returns success string
//   400: errorResponse
//   404: errorResponse
//   500: errorResponse

// swagger:route GET /light/{chain-id}/header Light getLightHeader
// Get light header of a chain.
// responses:
//   200: headerResponse
//   400: errorResponse
//   500: errorResponse

// swagger:response headerResponse
type headerResWrapper struct {
	// in:body
	Header *tmclient.Header
}

// swagger:route GET /light/{chain-id}/height Light getLightHeight
// Get light height of a chain.
// responses:
//   200: body:getLightHeightResponse Returns light height
//   400: errorResponse
//   500: errorResponse

// swagger:response getLightHeightResponse
type getLightHeightResWrapper struct {
	// in:body
	Height int64 `json:"height" yaml:"height"`
}

// swagger:route POST /light/{chain-id} Light initLight
// Init light header for a chain.
//
// force is optional and if given, it will be considered first,
// height and hash can be used instead of force and need to send both values.
// responses:
//   201: body:stringSuccessResponse Returns success string
//   400: errorResponse
//   500: errorResponse

// swagger:parameters initLight
type initLightParamsWrapper struct {
	// required:true
	// in:body
	Body postLightRequest `json:"body" yaml:"body"`
}

// swagger:route PUT /light/{chain-id} Light updateLight
// Update light header of a chain.
// responses:
//   200: body:stringSuccessResponse Returns success string
//   400: errorResponse
//   500: errorResponse

// swagger:route DELETE /light/{chain-id} Light deleteLight
// Delete light header of a chain.
// responses:
//   200: body:stringSuccessResponse Returns success string
//   400: errorResponse
//   500: errorResponse

// swagger:route GET /query/{chain-id}/account/{address} Query queryAccount
// Query account of a chain.
// responses:
//   200: body:queryAccountResponse Output format might change if address queried is Module Account
//   400: errorResponse
//   500: errorResponse

// swagger:parameters queryAccount queryBalance
type addressParamsWrapper struct {
	// in:path
	Address string `json:"address" yaml:"address"`
}

// swagger:response queryAccountResponse
type queryAccountResWrapper struct {
	// in:body
	Res struct {
		Account *authtypes.BaseAccount `json:"account" yaml:"account"`
	}
}

// swagger:route GET /query/{chain-id}/balance/{address} Query queryBalance
// Query balance of a chain.
// responses:
//   200: queryBalanceResponse
//   400: errorResponse
//   500: errorResponse

// swagger:parameters queryBalance
type queryBalanceParamsWrapper struct {
	// in:query
	IBCDenoms bool `json:"ibc-denoms" yaml:"ibc-denoms"`
}

// swagger:response queryBalanceResponse
type queryBalanceResWrapper struct {
	// in:body
	Balance sdk.Coins
}

// swagger:route GET /query/{chain-id}/header Query queryHeader
// Query header of a chain.
// responses:
//   200: headerResponse
//   400: errorResponse
//   500: errorResponse

// swagger:route GET /query/{chain-id}/node-state Query queryNodeState
// Query node state of a chain.
// responses:
//   200: nodeStateResponse
//   400: errorResponse
//   500: errorResponse

// swagger:response nodeStateResponse
type nodeStateResWrapper struct {
	// in:body
	ConsensusState *tmclient.ConsensusState
}

// swagger:route GET /query/{chain-id}/valset Query queryValSet
// Query node state of a chain.
// responses:
//   200: valSetResponse
//   400: errorResponse
//   500: errorResponse

// swagger:response valSetResponse
type valSetResWrapper struct {
	// in:body
	ValSet *tmproto.ValidatorSet
}

// swagger:route POST /query/{chain-id}/txs Query queryTxs
// Query Txs using events.
// responses:
//   200: txsResponse
//   400: errorResponse
//   500: errorResponse

// swagger:parameters queryTxs
type queryTxsParamsWrapper struct {
	// in:body
	Body txsRequest `json:"body" yaml:"body"`
}

// swagger:response txsResponse
type txsResWrapper struct {
	// in:body
	Txs []*ctypes.ResultTx
}

// swagger:route GET /query/{chain-id}/tx/{hash} Query queryTx
// Query Tx details by hash.
// responses:
//   200: txResponse
//   400: errorResponse
//   500: errorResponse

// swagger:parameters queryTx
type queryTxParamsWrapper struct {
	// in:path
	Hash string `json:"hash" yaml:"hash"`
}

// swagger:response txResponse
type txResWrapper struct {
	// in:body
	Txs *ctypes.ResultTx
}

// swagger:route GET /query/{chain-id}/clients/{client-id} Query queryClient
// Query client by clientID.
// responses:
//   200: queryClientResponse
//   400: errorResponse
//   500: errorResponse

// swagger:parameters queryClient
type clientParamsWrapper struct {
	// in:path
	ClientID string `json:"client-id" yaml:"client-id"`
}

// swagger:response queryClientResponse
type queryClientResWrapper struct {
	// in:body
	Client *clienttypes.QueryClientStateResponse
}

// swagger:route GET /query/{chain-id}/clients Query queryClients
// Query clients of a chain.
// responses:
//   200: queryClientsResponse
//   400: errorResponse
//   500: errorResponse

// swagger:response queryClientsResponse
type queryClientsResWrapper struct {
	// in:body
	Clients *clienttypes.QueryClientStatesResponse
}

// swagger:route GET /query/{chain-id}/connections/{conn-id} Query queryConn
// Query connection by connectionID.
// responses:
//   200: queryConnResponse
//   400: errorResponse
//   500: errorResponse

// swagger:parameters queryConn queryConnChans
type connectionParamsWrapper struct {
	// in:path
	ConnectionID string `json:"conn-id" yaml:"conn-id"`
}

// swagger:response queryConnResponse
type queryConnResWrapper struct {
	// in:body
	Connection *conntypes.QueryConnectionResponse
}

// swagger:route GET /query/{chain-id}/connections Query queryConns
// Query connections of a chain.
// responses:
//   200: queryConnsResponse
//   400: errorResponse
//   500: errorResponse

// swagger:response queryConnsResponse
type queryConnsResWrapper struct {
	// in:body
	Connections *conntypes.QueryConnectionsResponse
}

// swagger:route GET /query/{chain-id}/connections/client/{client-id} Query queryClientConns
// Query connections of a client in a chain.
// responses:
//   200: queryClientConnsResponse
//   400: errorResponse
//   500: errorResponse

// swagger:response queryClientConnsResponse
type queryClientConnsResWrapper struct {
	// in:body
	Connections *conntypes.QueryClientConnectionsResponse
}

// swagger:route GET /query/{chain-id}/channels/{chan-id}/{port-id} Query queryChan
// Query channel by channelID and portID.
// responses:
//   200: queryChanResponse
//   400: errorResponse
//   500: errorResponse

// swagger:parameters queryChan
type channelParamsWrapper struct {
	// in:path
	ChannelID string `json:"chan-id" yaml:"chan-id"`
	// in:path
	PortID string `json:"port-id" yaml:"port-id"`
}

// swagger:response queryChanResponse
type queryChanResWrapper struct {
	// in:body
	Channel *chantypes.QueryChannelResponse
}

// swagger:route GET /query/{chain-id}/channels Query queryChans
// Query channels of a chain.
// responses:
//   200: queryChansResponse
//   400: errorResponse
//   500: errorResponse

// swagger:response queryChansResponse
type queryChansResWrapper struct {
	// in:body
	Channels *chantypes.QueryChannelsResponse
}

// swagger:route GET /query/{chain-id}/channels/connection/{conn-id} Query queryConnChans
// Query channels of a connection in a chain.
// responses:
//   200: queryConnChansResponse
//   400: errorResponse
//   500: errorResponse

// swagger:response queryConnChansResponse
type queryConnChansResWrapper struct {
	// in:body
	Channels *chantypes.QueryConnectionChannelsResponse
}

// swagger:route GET /query/{chain-id}/ibc-denoms Query queryIBCDenoms
// Query ibc-denoms of a chain.
// responses:
//   200: queryIBCDenomsResponse
//   400: errorResponse
//   500: errorResponse

// swagger:response queryIBCDenomsResponse
type queryIBCDenomsResWrapper struct {
	// in:body
	IBCDenoms *transfertypes.QueryDenomTracesResponse
}
