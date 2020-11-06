package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync"

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

func getAPICmd() *cobra.Command {
	apiCmd := &cobra.Command{
		Use: "api",
		// Aliases: []string{""},
		Short: "Start the relayer API",
		RunE: func(cmd *cobra.Command, args []string) error {
			r := mux.NewRouter()
			sm := NewServicesManager()
			r.HandleFunc("/version", VersionHandler).Methods("GET")
			r.HandleFunc("/chains", GetChainsHandler).Methods("GET")
			r.HandleFunc("/chains/{name}", GetChainHandler).Methods("GET")
			r.HandleFunc("/chains/{name}/status", GetChainStatusHandler).Methods("GET")
			r.HandleFunc("/paths", GetPathsHandler).Methods("GET")
			r.HandleFunc("/paths/{name}", GetPathHandler).Methods("GET")
			r.HandleFunc("/paths/{name}/status", GetPathStatusHandler).Methods("GET")
			r.HandleFunc("/listen/{path}/{strategy}/{name}", PostRelayerListenHandler(sm)).Methods("POST")

			// TODO: listen validation in config
			fmt.Println("listening on", config.Global.APIListenPort)

			return http.ListenAndServe(config.Global.APIListenPort, r)
		},
	}
	return apiCmd
}

// PostRelayerListenHandler returns a handler for a listener that can listen on many IBC paths
func PostRelayerListenHandler(sm *ServicesManager) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		// TODO: check name to ensure that no other servies exist
		// TODO: make this handler accept a json post arguement
		pth, err := config.Paths.Get(vars["path"])
		if err != nil {
			errJSONBytes(err, w)
			return
		}
		c, src, dst, err := config.ChainsFromPath(vars["path"])
		if err != nil {
			errJSONBytes(err, w)
			return
		}
		pth.Strategy = &relayer.StrategyCfg{Type: vars["strategy"]}
		strat, err := pth.GetStrategy()
		if err != nil {
			errJSONBytes(err, w)
			return
		}
		done, err := relayer.RunStrategy(c[src], c[dst], strat)
		if err != nil {
			errJSONBytes(err, w)
			return
		}
		sm.Lock()
		sm.Services[vars["name"]] = NewService(vars["name"], vars["path"], c[src], c[dst], done)
		sm.Unlock()
	}
}

// GetChainsHandler returns the configured chains in json format
func GetChainsHandler(w http.ResponseWriter, r *http.Request) {
	out, _ := json.Marshal(config.Chains)
	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
}

// GetChainHandler returns the configured chains in json format
func GetChainHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["name"])
	if err != nil {
		errJSONBytes(err, w)
		return
	}
	out, _ := json.Marshal(chain)
	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
}

// GetChainStatusHandler returns the configured chains in json format
func GetChainStatusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chain, err := config.Chains.Get(vars["name"])
	if err != nil {
		errJSONBytes(err, w)
		return
	}
	out, _ := json.Marshal(chainStatusResponse{}.Populate(chain))
	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
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
	out, _ := json.Marshal(config.Paths)
	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
}

// GetPathHandler returns the configured chains in json format
func GetPathHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pth, err := config.Paths.Get(vars["name"])
	if err != nil {
		errJSONBytes(err, w)
		return
	}
	out, _ := json.Marshal(pth)
	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
}

// GetPathStatusHandler returns the configured chains in json format
func GetPathStatusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pth, err := config.Paths.Get(vars["name"])
	if err != nil {
		errJSONBytes(err, w)
		return
	}
	c, src, dst, err := config.ChainsFromPath(vars["name"])
	if err != nil {
		errJSONBytes(err, w)
		return
	}
	ps := pth.QueryPathStatus(c[src], c[dst])
	out, _ := json.Marshal(ps)
	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
}

// VersionHandler returns the version info in json format
func VersionHandler(w http.ResponseWriter, r *http.Request) {
	version := versionInfo{
		Version:   Version,
		Commit:    Commit,
		CosmosSDK: SDKCommit,
		Go:        fmt.Sprintf("%s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH),
	}
	out, _ := json.Marshal(version)
	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
}

func errJSONBytes(err error, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(fmt.Sprintf("{\"err\": \"%s\"}", err)))
}
