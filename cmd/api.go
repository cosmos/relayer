package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"

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

// Middleware calls initConfig for every request
func Middleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = initConfig(NewRootCmd())
		h.ServeHTTP(w, r)
	})
}

func getAPICmd() *cobra.Command {
	apiCmd := &cobra.Command{
		Use: "api",
		// Aliases: []string{""},
		Short: "Start the relayer API",
		RunE: func(cmd *cobra.Command, args []string) error {
			r := mux.NewRouter()

			r.Use(Middleware)

			sm := NewServicesManager()
			// NOTE: there is no hardening of this API. It is meant to be run in a secure environment and
			// accessed via private networking

			// VERSION
			// version get
			r.HandleFunc("/version", VersionHandler).Methods("GET")
			// CONFIG
			// config get
			r.HandleFunc("/config", ConfigHandler).Methods("GET")
			// CHAINS
			// chains get
			r.HandleFunc("/chains", GetChainsHandler).Methods("GET")
			// chain get
			r.HandleFunc("/chains/{name}", GetChainHandler).Methods("GET")
			// chain status get
			r.HandleFunc("/chains/{name}/status", GetChainStatusHandler).Methods("GET")
			// chain add
			// NOTE: when updating the config, we need to update the global config object ()
			// as well as the file on disk updating the file on disk may cause contention, just
			// retry the `file.Open` until the file can be opened, rewrite it with the new config
			// and close it
			r.HandleFunc("/chains/{name}", PostChainHandler).Methods("POST")
			// chain update
			r.HandleFunc("/chains/{name}", PutChainHandler).Methods("PUT")
			// chain delete
			r.HandleFunc("/chains/{name}", DeleteChainHandler).Methods("DELETE")
			// PATHS
			// paths get
			r.HandleFunc("/paths", GetPathsHandler).Methods("GET")
			// path get
			r.HandleFunc("/paths/{name}", GetPathHandler).Methods("GET")
			// path status get
			r.HandleFunc("/paths/{name}/status", GetPathStatusHandler).Methods("GET")
			// path add
			r.HandleFunc("/paths/{name}", PostPathHandler).Methods("POST")
			// path delete
			r.HandleFunc("/paths/{name}", DeletePathHandler).Methods("DELETE")
			// KEYS
			// keys get
			r.HandleFunc("/keys/{chain-id}", GetKeysHandler).Methods("GET")
			// key get
			r.HandleFunc("/keys/{chain-id}/{name}", GetKeyHandler).Methods("GET")
			// key add
			r.HandleFunc("/keys/{chain-id}/{name}", PostKeyHandler).Methods("POST")
			// key delete
			r.HandleFunc("/keys/{chain-id}/{name}", DeleteKeyHandler).Methods("DELETE")
			// key restore
			r.HandleFunc("/keys/{chain-id}/{name}/restore", RestoreKeyHandler).Methods("POST")
			// LIGHT
			// light header, if no ?height={height} is passed, latest
			r.HandleFunc("/light/{chain-id}/header", GetLightHeader).Methods("GET")
			// light height
			r.HandleFunc("/light/{chain-id}/height", GetLightHeight).Methods("GET")
			// light create
			r.HandleFunc("/light/{chain-id}", PostLight).Methods("POST")
			// light update
			r.HandleFunc("/light/{chain-id}", PutLight).Methods("PUT")
			// light delete
			r.HandleFunc("/light/{chain-id}", DeleteLight).Methods("DELETE")
			// QUERY
			// query account info for an address
			r.HandleFunc("/query/{chain-id}/account/{address}", QueryAccountHandler).Methods("GET")
			// query balance info for an address, if ibc-denoms=true?, then display ibc denominations
			r.HandleFunc("/query/{chain-id}/balance/{address}", QueryBalanceHandler).Methods("GET")
			// query header at height, if no ?height={height} is passed, latest
			r.HandleFunc("/query/{chain-id}/header", QueryHeaderHandler).Methods("GET")
			// query node-state
			r.HandleFunc("/query/{chain-id}/node-state", QueryNodeStateHandler).Methods("GET")
			// query valset at height, if no ?height={height} is passed, latest
			r.HandleFunc("/query/{chain-id}/valset", QueryValSetHandler).Methods("GET")
			// query txs passing in a query via POST request
			r.HandleFunc("/query/{chain-id}/txs", QueryTxsHandler).Methods("POST")
			// query tx by hash
			r.HandleFunc("/query/{chain-id}/tx/{hash}", QueryTxHandler).Methods("GET")
			// query client by chain-id and client-id, if no ?height={height} is passed, latest
			r.HandleFunc("/query/{chain-id}/clients/{client-id}", QueryClientHandler).Methods("GET")
			// query clients by chain-id with pagination (offset and limit query params)
			r.HandleFunc("/query/{chain-id}/clients", QueryClientsHandler).Methods("GET")
			// query connection by chain-id and conn-id, if no ?height={height} is passed, latest
			r.HandleFunc("/query/{chain-id}/connections/{conn-id}", QueryConnectionHandler).Methods("GET")
			// query connections by chain-id with pagination (offset and limit query params)
			r.HandleFunc("/query/{chain-id}/connections", QueryConnectionsHandler).Methods("GET")
			// query connections by chain-id and client-id, if no ?height={height} is passed, latest
			r.HandleFunc("/query/{chain-id}/connections/client/{client-id}",
				QueryClientConnectionsHandler).Methods("GET")
			// query channel by chain-id chan-id and port-id, if no ?height={height} is passed, latest
			r.HandleFunc("/query/{chain-id}/channels/{chan-id}/{port-id}",
				QueryChannelHandler).Methods("GET")
			// query channels by chain-id with pagination (offset and limit query params)
			r.HandleFunc("/query/{chain-id}/channels", QueryChannelsHandler).Methods("GET")
			// query channels by chain-id and conn-id with pagination (offset and limit query params)
			r.HandleFunc("/query/{chain-id}/channels/connection/{conn-id}", QueryConnectionChannelsHandler).Methods("GET")
			// query a chain's ibc denoms
			r.HandleFunc("/query/{chain-id}/ibc-denoms", QueryIBCDenomsHandler).Methods("GET")

			// TODO: this particular function needs some work, we need to listen on chains in configuration and
			// route all the events (both block and tx) though and event bus to allow for multiple subscribers
			// on update of config we need to handle that case
			// Data for this should be stored in the ServicesManager struct
			r.HandleFunc("/listen/{path}/{strategy}/{name}", PostRelayerListenHandler(sm)).Methods("POST")

			fs := http.FileServer(http.Dir("./docs/swagger-ui"))
			r.PathPrefix("/").Handler(fs)

			fmt.Println("listening on", config.Global.APIListenPort)

			if err := http.ListenAndServe(config.Global.APIListenPort, r); err != nil {
				return err
			}

			return nil
		},
	}
	return apiCmd
}

// ConfigHandler handles the route
func ConfigHandler(w http.ResponseWriter, r *http.Request) {
	helpers.SuccessJSONResponse(http.StatusOK, config, w)
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
		&authtypes.QueryAccountRequest{
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

	ibcDenoms := strings.TrimSpace(r.URL.Query().Get("ibc-denoms"))

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
	height := strings.TrimSpace(r.URL.Query().Get("height"))

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

type txsRequest struct {
	Events []string `json:"events"`
}

// QueryTxsHandler handles the route
func QueryTxsHandler(w http.ResponseWriter, r *http.Request) {
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
	if len(strings.TrimSpace(r.URL.Query().Get("offset"))) == 0 {
		offset = 1
	}

	if len(strings.TrimSpace(r.URL.Query().Get("limit"))) == 0 {
		limit = 100
	}

	var request txsRequest
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
		// TODO: check name to ensure that no other services exist
		// TODO: make this handler accept a json post argument
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
		strategyType, err := pth.GetStrategy()
		if err != nil {
			helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
			return
		}
		done, err := relayer.RunStrategy(c[src], c[dst], strategyType)
		if err != nil {
			helpers.WriteErrorResponse(http.StatusInternalServerError, err, w)
			return
		}
		sm.Lock()
		sm.Services[vars["name"]] = NewService(vars["name"], vars["path"], c[src], c[dst], done)
		sm.Unlock()
	}
}
