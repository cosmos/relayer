package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	clienttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/helpers"
	"github.com/cosmos/relayer/relayer"
	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
)

// ServicesManager represents the manager of the various services the relayer is running
type ServicesManager struct {
	Services map[string]*Service

	sync.Mutex
}

// NewServicesManager returns a new instance of a services manager
func NewServicesManager() *ServicesManager {
	return &ServicesManager{Services: make(map[string]*Service)}
}

// Service represents a relayer listen service
// TODO: sync services to disk so that they can survive restart
type Service struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Src    string `json:"src"`
	SrcKey string `json:"src-key"`
	Dst    string `json:"dst"`
	DstKey string `json:"dst-key"`

	doneFunc func()
}

// NewService returns a new instance of Service
func NewService(name, path string, src, dst *relayer.Chain, doneFunc func()) *Service {
	return &Service{name, path, src.ChainID, src.Key, dst.ChainID, dst.Key, doneFunc}
}

const (
	version    = "version"
	cfg        = "config"
	chains     = "chains"
	paths      = "paths"
	keys       = "keys"
	nameArg    = "{name}"
	status     = "status"
	header     = "header"
	height     = "height"
	query      = "query"
	light      = "light"
	chainIDArg = "{chain-id}"
)

func getAPICmd() *cobra.Command {
	apiCmd := &cobra.Command{
		Use: "api",
		// Aliases: []string{""},
		Short: "Start the relayer API",
		RunE: func(cmd *cobra.Command, args []string) error {
			r := mux.NewRouter()
			sm := NewServicesManager()
			// NOTE: there is no hardening of this API. It is meant to be run in a secure environment and
			// accessed via private networking

			// VERSION
			// version get
			r.HandleFunc(fmt.Sprintf("/%s", version), VersionHandler).Methods("GET")
			// CONFIG
			// config get
			r.HandleFunc(fmt.Sprintf("/%s", cfg), ConfigHandler).Methods("GET")
			// CHAINS
			// chains get
			r.HandleFunc(fmt.Sprintf("/%s", chains), GetChainsHandler).Methods("GET")
			// chain get
			r.HandleFunc(fmt.Sprintf("/%s/%s", chains, nameArg), GetChainHandler).Methods("GET")
			// chain status get
			r.HandleFunc(fmt.Sprintf("/%s/%s/%s", chains, nameArg, status), GetChainStatusHandler).Methods("GET")
			// chain add
			// NOTE: when updating the config, we need to update the global config object ()
			// as well as the file on disk updating the file on disk may cause contention, just
			// retry the `file.Open` until the file can be opened, rewrite it with the new config
			// and close it
			r.HandleFunc(fmt.Sprintf("/%s/%s", chains, nameArg), PostChainHandler).Methods("POST")
			// chain update
			r.HandleFunc(fmt.Sprintf("/%s/%s", chains, nameArg), PutChainHandler).Methods("PUT")
			// chain delete
			r.HandleFunc(fmt.Sprintf("/%s/%s", chains, nameArg), DeleteChainHandler).Methods("DELETE")
			// PATHS
			// paths get
			r.HandleFunc(fmt.Sprintf("/%s", paths), GetPathsHandler).Methods("GET")
			// path get
			r.HandleFunc(fmt.Sprintf("/%s/%s", paths, nameArg), GetPathHandler).Methods("GET")
			// path status get
			r.HandleFunc(fmt.Sprintf("/%s/%s/%s", paths, nameArg, status), GetPathStatusHandler).Methods("GET")
			// path add
			r.HandleFunc(fmt.Sprintf("/%s/%s", paths, nameArg), PostPathHandler).Methods("POST")
			// path update
			r.HandleFunc(fmt.Sprintf("/%s/%s", paths, nameArg), PutPathHandler).Methods("PUT")
			// path delete
			r.HandleFunc(fmt.Sprintf("/%s/%s", paths, nameArg), DeletePathHandler).Methods("DELETE")
			// KEYS
			// keys get
			r.HandleFunc(fmt.Sprintf("/%s/%s", keys, chainIDArg), GetKeysHandler).Methods("GET")
			// key get
			r.HandleFunc(fmt.Sprintf("/%s/%s/%s", keys, chainIDArg, nameArg), GetKeyHandler).Methods("GET")
			// key add
			r.HandleFunc(fmt.Sprintf("/%s/%s/%s", keys, chainIDArg, nameArg), PostKeyHandler).Methods("POST")
			// key delete
			r.HandleFunc(fmt.Sprintf("/%s/%s/%s", keys, chainIDArg, nameArg), DeleteKeyHandler).Methods("DELETE")
			// key restore
			r.HandleFunc(fmt.Sprintf("/%s/%s/%s/restore", keys, chainIDArg, nameArg), RestoreKeyHandler).Methods("POST")
			// LIGHT
			// light header, if no ?height={height} is passed, latest
			r.HandleFunc(fmt.Sprintf("/%s/%s/%s", light, chainIDArg, header), GetLightHeader).Methods("GET")
			// light height
			r.HandleFunc(fmt.Sprintf("/%s/%s/%s", light, chainIDArg, height), GetLightHeight).Methods("GET")
			// light create
			r.HandleFunc(fmt.Sprintf("/%s/%s", light, chainIDArg), PostLight).Methods("POST")
			// light update
			r.HandleFunc(fmt.Sprintf("/%s/%s", light, chainIDArg), PutLight).Methods("PUT")
			// light delete
			r.HandleFunc(fmt.Sprintf("/%s/%s", light, chainIDArg), DeleteLight).Methods("DELETE")
			// QUERY
			// query account info for an address
			r.HandleFunc(fmt.Sprintf("/%s/%s/account/{address}", query, chainIDArg), QueryAccountHandler).Methods("GET")
			// query balance info for an address, if ibc-denoms=true?, then display ibc denominations
			r.HandleFunc(fmt.Sprintf("/%s/%s/balance/{address}", query, chainIDArg), QueryBalanceHandler).Methods("GET")
			// query header at height, if no ?height={height} is passed, latest
			r.HandleFunc(fmt.Sprintf("/%s/%s/header", query, chainIDArg), QueryHeaderHandler).Methods("GET")
			// query node-state
			r.HandleFunc(fmt.Sprintf("/%s/%s/node-state", query, chainIDArg), QueryNodeStateHandler).Methods("GET")
			// query valset at height, if no ?height={height} is passed, latest
			r.HandleFunc(fmt.Sprintf("/%s/%s/valset", query, chainIDArg), QueryValSetHandler).Methods("GET")
			// query txs passing in a query via POST request
			r.HandleFunc(fmt.Sprintf("/%s/%s/txs", query, chainIDArg), QuerytxsHandler).Methods("POST")
			// query tx by hash
			r.HandleFunc(fmt.Sprintf("/%s/%s/tx/{hash}", query, chainIDArg), QueryTxHandler).Methods("GET")
			// query client by chain-id and client-id, if no ?height={height} is passed, latest
			r.HandleFunc(fmt.Sprintf("/%s/%s/clients/{client-id}", query, chainIDArg), QueryClientHandler).Methods("GET")
			// query clients by chain-id with pagination (offset and limit query params)
			r.HandleFunc(fmt.Sprintf("/%s/%s/clients", query, chainIDArg), QueryClientsHandler).Methods("GET")
			// query connection by chain-id and conn-id, if no ?height={height} is passed, latest
			r.HandleFunc(fmt.Sprintf("/%s/%s/connections/{conn-id}", query, chainIDArg), QueryConnectionHandler).Methods("GET")
			// query connections by chain-id with pagination (offset and limit query params)
			r.HandleFunc(fmt.Sprintf("/%s/%s/connections", query, chainIDArg), QueryConnectionsHandler).Methods("GET")
			// query connections by chain-id and client-id, if no ?height={height} is passed, latest
			r.HandleFunc(fmt.Sprintf("/%s/%s/connections/client/{client-id}", query, chainIDArg), QueryClientConnectionsHandler).Methods("GET")
			// query channel by chain-id chan-id and port-id, if no ?height={height} is passed, latest
			r.HandleFunc(fmt.Sprintf("/%s/%s/channels/{chan-id}/{port-id}", query, chainIDArg), QueryChannelHandler).Methods("GET")
			// query channels by chain-id with pagination (offset and limit query params)
			r.HandleFunc(fmt.Sprintf("/%s/%s/channels", query, chainIDArg), QueryChannelsHandler).Methods("GET")
			// query channels by chain-id and conn-id with pagination (offset and limit query params)
			r.HandleFunc(fmt.Sprintf("/%s/%s/channels/connection/{conn-id}", query, chainIDArg), QueryConnectionChannelsHandler).Methods("GET")
			// query a chain's ibc denoms
			r.HandleFunc(fmt.Sprintf("/%s/%s/ibc-denoms", query, chainIDArg), QueryIBCDenomsHandler).Methods("GET")

			// TODO: this particular function needs some work, we need to listen on chains in configuration and
			// route all the events (both block and tx) though and event bus to allow for multiple subscribers
			// on update of config we need to handle that case
			// Data for this should be stored in the ServicesManager struct
			r.HandleFunc("/listen/{path}/{strategy}/{name}", PostRelayerListenHandler(sm)).Methods("POST")

			// TODO: do we want to add the transaction commands here to?
			// initial thoughts: expose high level transactions
			// tx create-clients
			// POST /paths/{name}/clients
			// tx update-clients
			// PUT /paths/{name}/clients
			// tx connection
			// POST /paths/{name}/connections
			// tx channel
			// POST /paths/{name}/channels
			// tx link
			// POST /paths/{name}/link
			// tx relay-packets
			// POST /paths/{name}/relay/packets
			// tx relay-acks
			// POST /paths/{name}/relay/acks
			// tx transfer
			// POST /paths/{name}/transfers

			// TODO: listen validation in config
			fmt.Println("listening on", config.Global.APIListenPort)

			return http.ListenAndServe(config.Global.APIListenPort, r)
		},
	}
	return apiCmd
}

// ConfigHandler handles the route
func ConfigHandler(w http.ResponseWriter, r *http.Request) {
	helpers.SuccessJSONResponse(http.StatusOK, config, w)
}

// PostChainHandler handles the route
func PostChainHandler(w http.ResponseWriter, r *http.Request) {}

// PutChainHandler handles the route
func PutChainHandler(w http.ResponseWriter, r *http.Request) {}

// DeleteChainHandler handles the route
func DeleteChainHandler(w http.ResponseWriter, r *http.Request) {}

// PostPathHandler handles the route
func PostPathHandler(w http.ResponseWriter, r *http.Request) {}

// PutPathHandler handles the route
func PutPathHandler(w http.ResponseWriter, r *http.Request) {}

// DeletePathHandler handles the route
func DeletePathHandler(w http.ResponseWriter, r *http.Request) {}

type keyResponse struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

func formatKey(info keyring.Info) keyResponse {
	return keyResponse{
		Name:    info.GetName(),
		Address: info.GetAddress().String(),
	}
}

// GetKeysHandler handles the route
func GetKeysHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}
	info, err := chain.Keybase.List()
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}

	keys := make([]keyResponse, len(info))
	for index, key := range info {
		keys[index] = formatKey(key)
	}
	helpers.SuccessJSONResponse(http.StatusOK, keys, w)
}

// GetKeyHandler handles the route
func GetKeyHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	keyName := vars["name"]
	if !chain.KeyExists(keyName) {
		helpers.WriteErrorResponse(http.StatusNotFound, errKeyDoesntExist(keyName), w)
		return
	}

	info, err := chain.Keybase.Key(keyName)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	helpers.SuccessJSONResponse(http.StatusOK, formatKey(info), w)
}

// PostKeyHandler handles the route
func PostKeyHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	keyName := vars["name"]
	if chain.KeyExists(keyName) {
		helpers.WriteErrorResponse(http.StatusBadRequest, errKeyExists(keyName), w)
		return
	}

	ko, err := helpers.KeyAddOrRestore(chain, keyName)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	helpers.SuccessJSONResponse(http.StatusCreated, ko, w)
}

type restoreKeyRequest struct {
	Mnemonic string `json:"mnemonic"`
}

// RestoreKeyHandler handles the route
func RestoreKeyHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	keyName := vars["name"]
	if chain.KeyExists(keyName) {
		helpers.WriteErrorResponse(http.StatusNotFound, errKeyExists(keyName), w)
		return
	}

	var request restoreKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	ko, err := helpers.KeyAddOrRestore(chain, keyName, request.Mnemonic)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	helpers.SuccessJSONResponse(http.StatusOK, ko, w)
}

// DeleteKeyHandler handles the route
func DeleteKeyHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	keyName := vars["name"]
	if !chain.KeyExists(keyName) {
		helpers.WriteErrorResponse(http.StatusNotFound, errKeyDoesntExist(keyName), w)
		return
	}

	err = chain.Keybase.Delete(keyName)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	helpers.SuccessJSONResponse(http.StatusOK, fmt.Sprintf("key %s deleted", keyName), w)
}

// GetLightHeader handles the route
func GetLightHeader(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	var header *tmclient.Header
	height := r.URL.Query().Get("height")

	if len(height) == 0 {
		header, err = helpers.GetLightHeader(chain)
		if err != nil {
			helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
			return
		}
	} else {
		header, err = helpers.GetLightHeader(chain, height)
		if err != nil {
			helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
			return
		}
	}
	helpers.SuccessProtoResponse(http.StatusOK, chain, header, w)
}

// GetLightHeight handles the route
func GetLightHeight(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	height, err := chain.GetLatestLightHeight()
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	helpers.SuccessJSONResponse(http.StatusOK, height, w)
}

// PostLight handles the route
func PostLight(w http.ResponseWriter, r *http.Request) {}

// PutLight handles the route
func PutLight(w http.ResponseWriter, r *http.Request) {}

// DeleteLight handles the route
func DeleteLight(w http.ResponseWriter, r *http.Request) {}

// QueryAccountHandler handles the route
func QueryAccountHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	res, err := authtypes.NewQueryClient(chain.CLIContext(0)).Account(
		context.Background(),
		&types.QueryAccountRequest{
			Address: vars["address"],
		})
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	helpers.SuccessProtoResponse(http.StatusOK, chain, res, w)
}

// QueryBalanceHandler handles the route
func QueryBalanceHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	ibcDenoms := r.URL.Query().Get("ibc-denoms")

	showDenoms := false
	if ibcDenoms == "true" {
		showDenoms = true
	}

	res, err := helpers.QueryBalance(chain, vars["address"], showDenoms)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	helpers.SuccessJSONResponse(http.StatusOK, res, w)
}

// QueryHeaderHandler handles the route
func QueryHeaderHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	var header *tmclient.Header
	height := r.URL.Query().Get("height")

	if len(height) == 0 {
		header, err = helpers.QueryHeader(chain)
		if err != nil {
			helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
			return
		}
	} else {
		header, err = helpers.QueryHeader(chain, height)
		if err != nil {
			helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
			return
		}
	}
	helpers.SuccessProtoResponse(http.StatusOK, chain, header, w)
}

// QueryNodeStateHandler handles the route
func QueryNodeStateHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	consensusState, _, err := chain.QueryConsensusState(0)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	helpers.SuccessProtoResponse(http.StatusOK, chain, consensusState, w)
}

// QueryValSetHandler handles the route
func QueryValSetHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	height, err := helpers.ParseHeightFromRequest(r, chain)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	version := clienttypes.ParseChainID(vars["chain-id"])

	res, err := chain.QueryValsetAtHeight(clienttypes.NewHeight(version, uint64(height)))
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	helpers.SuccessProtoResponse(http.StatusOK, chain, res, w)
}

type TxsRequest struct {
	Events []string `json:"events"`
}

// QuerytxsHandler handles the route
func QuerytxsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	offset, limit, err := helpers.ParsePaginationParams(r)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	// Setting default values for pagination if query params not given
	if len(r.URL.Query().Get("offset")) == 0 {
		offset = 1
	}

	if len(r.URL.Query().Get("limit")) == 0 {
		limit = 100
	}

	var request TxsRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	txs, err := helpers.QueryTxs(chain, strings.Join(request.Events, "&"), offset, limit)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	helpers.SuccessJSONResponse(http.StatusOK, txs, w)
}

// QueryTxHandler handles the route
func QueryTxHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	tx, err := chain.QueryTx(vars["hash"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	helpers.SuccessJSONResponse(http.StatusOK, tx, w)
}

// QueryClientHandler handles the route
func QueryClientHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	height, err := helpers.ParseHeightFromRequest(r, chain)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	if err = chain.AddPath(vars["client-id"], dcon, dcha, dpor, dord); err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}

	res, err := chain.QueryClientState(height)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	helpers.SuccessProtoResponse(http.StatusOK, chain, res, w)
}

// QueryClientsHandler handles the route
func QueryClientsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	offset, limit, err := helpers.ParsePaginationParams(r)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	res, err := chain.QueryClients(offset, limit)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	helpers.SuccessProtoResponse(http.StatusOK, chain, res, w)
}

// QueryConnectionHandler handles the route
func QueryConnectionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	height, err := helpers.ParseHeightFromRequest(r, chain)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	if err = chain.AddPath(dcli, vars["conn-id"], dcha, dpor, dord); err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}

	res, err := chain.QueryConnection(height)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	helpers.SuccessProtoResponse(http.StatusOK, chain, res, w)
}

// QueryConnectionsHandler handles the route
func QueryConnectionsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	offset, limit, err := helpers.ParsePaginationParams(r)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	res, err := chain.QueryConnections(offset, limit)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	helpers.SuccessProtoResponse(http.StatusOK, chain, res, w)
}

// QueryClientConnectionsHandler handles the route
func QueryClientConnectionsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	height, err := helpers.ParseHeightFromRequest(r, chain)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	if err = chain.AddPath(vars["client-id"], dcon, dcha, dpor, dord); err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}

	res, err := chain.QueryConnectionsUsingClient(height)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	helpers.SuccessProtoResponse(http.StatusOK, chain, res, w)
}

// QueryChannelHandler handles the route
func QueryChannelHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	height, err := helpers.ParseHeightFromRequest(r, chain)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	if err = chain.AddPath(dcli, dcon, vars["chan-id"], vars["port-id"], dord); err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}

	res, err := chain.QueryChannel(height)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	helpers.SuccessProtoResponse(http.StatusOK, chain, res, w)
}

// QueryChannelsHandler handles the route
func QueryChannelsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	offset, limit, err := helpers.ParsePaginationParams(r)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	res, err := chain.QueryChannels(offset, limit)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	helpers.SuccessProtoResponse(http.StatusOK, chain, res, w)
}

// QueryConnectionChannelsHandler handles the route
func QueryConnectionChannelsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	if err = chain.AddPath(dcli, vars["conn-id"], dcha, dpor, dord); err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}

	offset, limit, err := helpers.ParsePaginationParams(r)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}

	res, err := chain.QueryConnectionChannels(vars["conn-id"], offset, limit)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	helpers.SuccessProtoResponse(http.StatusOK, chain, res, w)
}

// QueryIBCDenomsHandler handles the route
func QueryIBCDenomsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["chain-id"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}

	h, err := chain.QueryLatestHeight()
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}

	res, err := chain.QueryDenomTraces(0, 1000, h)
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	helpers.SuccessProtoResponse(http.StatusOK, chain, res, w)
}

// PostRelayerListenHandler returns a handler for a listener that can listen on many IBC paths
func PostRelayerListenHandler(sm *ServicesManager) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		// TODO: check name to ensure that no other servies exist
		// TODO: make this handler accept a json post arguement
		pth, err := config.Paths.Get(vars["path"])
		if err != nil {
			helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
			return
		}
		c, src, dst, err := config.ChainsFromPath(vars["path"])
		if err != nil {
			helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
			return
		}
		pth.Strategy = &relayer.StrategyCfg{Type: vars["strategy"]}
		strat, err := pth.GetStrategy()
		if err != nil {
			helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
			return
		}
		done, err := relayer.RunStrategy(c[src], c[dst], strat)
		if err != nil {
			helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
			return
		}
		sm.Lock()
		sm.Services[vars["name"]] = NewService(vars["name"], vars["path"], c[src], c[dst], done)
		sm.Unlock()
	}
}

// GetChainsHandler returns the configured chains in json format
func GetChainsHandler(w http.ResponseWriter, r *http.Request) {
	helpers.SuccessJSONResponse(http.StatusOK, config.Chains, w)
}

// GetChainHandler returns the configured chains in json format
func GetChainHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["name"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}
	helpers.SuccessJSONResponse(http.StatusOK, chain, w)
}

// GetChainStatusHandler returns the configured chains in json format
func GetChainStatusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["name"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}
	helpers.SuccessJSONResponse(http.StatusOK, chainStatusResponse{}.Populate(chain), w)
}

type chainStatusResponse struct {
	Light   bool `json:"light"`
	Path    bool `json:"path"`
	Key     bool `json:"key"`
	Balance bool `json:"balance"`
}

func (cs chainStatusResponse) Populate(c *relayer.Chain) chainStatusResponse {
	_, err := c.GetAddress()
	if err == nil {
		cs.Key = true
	}

	coins, err := c.QueryBalance(c.Key)
	if err == nil && !coins.Empty() {
		cs.Balance = true
	}

	_, err = c.GetLatestLightHeader()
	if err == nil {
		cs.Light = true
	}

	for _, pth := range config.Paths {
		if pth.Src.ChainID == c.ChainID || pth.Dst.ChainID == c.ChainID {
			cs.Path = true
		}
	}
	return cs
}

// GetPathsHandler returns the configured chains in json format
func GetPathsHandler(w http.ResponseWriter, r *http.Request) {
	helpers.SuccessJSONResponse(http.StatusOK, config.Paths, w)
}

// GetPathHandler returns the configured chains in json format
func GetPathHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pth, err := config.Paths.Get(vars["name"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}
	helpers.SuccessJSONResponse(http.StatusOK, pth, w)
}

// GetPathStatusHandler returns the configured chains in json format
func GetPathStatusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pth, err := config.Paths.Get(vars["name"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusBadRequest, err, w)
		return
	}
	c, src, dst, err := config.ChainsFromPath(vars["name"])
	if err != nil {
		helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	ps := pth.QueryPathStatus(c[src], c[dst])
	helpers.SuccessJSONResponse(http.StatusOK, ps, w)
}

// VersionHandler returns the version info in json format
func VersionHandler(w http.ResponseWriter, r *http.Request) {
	version := versionInfo{
		Version:   Version,
		Commit:    Commit,
		CosmosSDK: SDKCommit,
		Go:        fmt.Sprintf("%s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH),
	}
	helpers.SuccessJSONResponse(http.StatusOK, version, w)
}
