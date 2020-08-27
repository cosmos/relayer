package relayer

import (
	"fmt"

	"gopkg.in/yaml.v2"

	chantypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
)

var ( // Default identifiers for dummy usage
	dcon = "defaultconnectionid"
	dcha = "defaultchannelid"
	dpor = "defaultportid"
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
	if err := path.Validate(); err != nil {
		return err
	}
	if _, found := p[name]; found {
		return fmt.Errorf("path with name %s already exists", name)
	}
	p[name] = path
	return nil
}

// AddForce ignores existing paths and overwrites an existing path with that name
func (p Paths) AddForce(name string, path *Path) error {
	if err := path.Validate(); err != nil {
		return err
	}
	if _, found := p[name]; found {
		fmt.Printf("overwriting path %s with new path...\n", name)
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
		if (path.Dst.ChainID == src || path.Src.ChainID == src) && (path.Dst.ChainID == dst || path.Src.ChainID == dst) {
			out[name] = path
		}
	}
	if len(out) == 0 {
		return Paths{}, fmt.Errorf("failed to find path in config between chains %s and %s", src, dst)
	}
	return out, nil
}

// Path represents a pair of chains and the identifiers needed to
// relay over them
type Path struct {
	Src      *PathEnd     `yaml:"src" json:"src"`
	Dst      *PathEnd     `yaml:"dst" json:"dst"`
	Strategy *StrategyCfg `yaml:"strategy" json:"strategy"`
}

// Ordered returns true if the path is ordered and false if otherwise
func (p *Path) Ordered() bool {
	return p.Src.getOrder() == chantypes.ORDERED
}

// Validate checks that a path is valid
func (p *Path) Validate() (err error) {
	if err = p.Src.Validate(); err != nil {
		return err
	}
	if p.Src.Version == "" {
		return fmt.Errorf("source must specify a version")
	}
	if err = p.Dst.Validate(); err != nil {
		return err
	}
	if _, err = p.GetStrategy(); err != nil {
		return err
	}
	if p.Src.Order != p.Dst.Order {
		return fmt.Errorf("both sides must have same order ('ORDERED' or 'UNORDERED'), got src(%s) and dst(%s)",
			p.Src.Order, p.Dst.Order)
	}
	return nil
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

// GenPath generates a path with random client, connection and channel identifiers
// given chainIDs and portIDs
func GenPath(srcChainID, dstChainID, srcPortID, dstPortID, order string, version string) *Path {
	return &Path{
		Src: &PathEnd{
			ChainID:      srcChainID,
			ClientID:     RandLowerCaseLetterString(10),
			ConnectionID: RandLowerCaseLetterString(10),
			ChannelID:    RandLowerCaseLetterString(10),
			PortID:       srcPortID,
			Order:        order,
			Version:      version,
		},
		Dst: &PathEnd{
			ChainID:      dstChainID,
			ClientID:     RandLowerCaseLetterString(10),
			ConnectionID: RandLowerCaseLetterString(10),
			ChannelID:    RandLowerCaseLetterString(10),
			PortID:       dstPortID,
			Order:        order,
			Version:      version,
		},
		Strategy: &StrategyCfg{
			Type: "naive",
		},
	}
}
