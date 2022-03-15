package relayer

import (
	"fmt"

	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"

	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v3/modules/core/03-connection/types"
)

const (
	check = "✔"
	xIcon = "✘"
)

// Paths represent connection paths between chains
type Paths map[string]*Path

// MustYAML returns the yaml string representation of the Paths
func (p Paths) MustYAML() string {
	out, err := yaml.Marshal(p)
	if err != nil {
		panic(err)
	}
	return string(out)
}

// Get returns the configuration for a given path
func (p Paths) Get(name string) (path *Path, err error) {
	if pth, ok := p[name]; ok {
		path = pth
	} else {
		err = fmt.Errorf("path with name %s does not exist", name)
	}
	return
}

// MustGet panics if path is not found
func (p Paths) MustGet(name string) *Path {
	pth, err := p.Get(name)
	if err != nil {
		panic(err)
	}
	return pth
}

// Add adds a path by its name
func (p Paths) Add(name string, path *Path) error {
	if _, found := p[name]; found {
		return fmt.Errorf("path with name %s already exists", name)
	}
	p[name] = path
	return nil
}

// MustYAML returns the yaml string representation of the Path
func (p *Path) MustYAML() string {
	out, err := yaml.Marshal(p)
	if err != nil {
		panic(err)
	}
	return string(out)
}

// PathsFromChains returns a path from the config between two chains
func (p Paths) PathsFromChains(src, dst string) (Paths, error) {
	out := Paths{}
	for name, path := range p {
		if (path.Dst.ChainID == src || path.Src.ChainID == src) &&
			(path.Dst.ChainID == dst || path.Src.ChainID == dst) {
			out[name] = path
		}
	}
	if len(out) == 0 {
		return Paths{}, fmt.Errorf("failed to find path in config between chains %s and %s", src, dst)
	}
	return out, nil
}

// PathAction is struct
type PathAction struct {
	*Path
	Type string `json:"type"`
}

// Path represents a pair of chains and the identifiers needed to
// relay over them
type Path struct {
	Src *PathEnd `yaml:"src" json:"src"`
	Dst *PathEnd `yaml:"dst" json:"dst"`
}

// End returns the proper end given a chainID
func (p *Path) End(chainID string) *PathEnd {
	if p.Dst.ChainID == chainID {
		return p.Dst
	}
	if p.Src.ChainID == chainID {
		return p.Src
	}
	return &PathEnd{}
}

func (p *Path) String() string {
	return fmt.Sprintf("[ ] %s ->\n %s", p.Src.String(), p.Dst.String())
}

// GenPath generates a path with unspecified client, connection and channel identifiers
// given chainIDs and portIDs.
func GenPath(srcChainID, dstChainID string) *Path {
	return &Path{
		Src: &PathEnd{
			ChainID:      srcChainID,
			ClientID:     "",
			ConnectionID: "",
		},
		Dst: &PathEnd{
			ChainID:      dstChainID,
			ClientID:     "",
			ConnectionID: "",
		},
	}
}

// PathStatus holds the status of the primitives in the path
type PathStatus struct {
	Chains     bool `yaml:"chains" json:"chains"`
	Clients    bool `yaml:"clients" json:"clients"`
	Connection bool `yaml:"connection" json:"connection"`
}

// PathWithStatus is used for showing the status of the path
type PathWithStatus struct {
	Path   *Path      `yaml:"path" json:"chains"`
	Status PathStatus `yaml:"status" json:"status"`
}

// QueryPathStatus returns an instance of the path struct with some attached data about
// the current status of the path
func (p *Path) QueryPathStatus(src, dst *Chain) *PathWithStatus {
	var (
		err              error
		eg               errgroup.Group
		srch, dsth       int64
		srcCs, dstCs     *clienttypes.QueryClientStateResponse
		srcConn, dstConn *conntypes.QueryConnectionResponse

		out = &PathWithStatus{Path: p, Status: PathStatus{false, false, false}}
	)
	eg.Go(func() error {
		srch, err = src.ChainProvider.QueryLatestHeight()
		return err
	})
	eg.Go(func() error {
		dsth, err = dst.ChainProvider.QueryLatestHeight()
		return err
	})
	if err = eg.Wait(); err != nil {
		return out
	}
	out.Status.Chains = true
	if err = src.SetPath(p.Src); err != nil {
		return out
	}
	if err = dst.SetPath(p.Dst); err != nil {
		return out
	}

	eg.Go(func() error {
		srcCs, err = src.ChainProvider.QueryClientStateResponse(srch, src.ClientID())
		return err
	})
	eg.Go(func() error {
		dstCs, err = dst.ChainProvider.QueryClientStateResponse(dsth, dst.ClientID())
		return err
	})
	if err = eg.Wait(); err != nil || srcCs == nil || dstCs == nil {
		return out
	}
	out.Status.Clients = true

	eg.Go(func() error {
		srcConn, err = src.ChainProvider.QueryConnection(srch, src.ConnectionID())
		return err
	})
	eg.Go(func() error {
		dstConn, err = dst.ChainProvider.QueryConnection(dsth, dst.ConnectionID())
		return err
	})
	if err = eg.Wait(); err != nil || srcConn.Connection.State != conntypes.OPEN ||
		dstConn.Connection.State != conntypes.OPEN {
		return out
	}
	out.Status.Connection = true
	return out
}

// PrintString prints a string representations of the path status
func (ps *PathWithStatus) PrintString(name string) string {
	pth := ps.Path
	return fmt.Sprintf(`Path "%s":
  SRC(%s)
    ClientID:     %s
    ConnectionID: %s
  DST(%s)
    ClientID:     %s
    ConnectionID: %s
  STATUS:
    Chains:       %s
    Clients:      %s
    Connection:   %s`, name, pth.Src.ChainID, pth.Src.ClientID, pth.Src.ConnectionID, pth.Dst.ChainID, pth.Dst.ClientID,
		pth.Dst.ConnectionID, checkmark(ps.Status.Chains), checkmark(ps.Status.Clients), checkmark(ps.Status.Connection))
}

func checkmark(status bool) string {
	if status {
		return check
	}
	return xIcon
}
