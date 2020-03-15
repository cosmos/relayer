package relayer

import (
	"fmt"

	"gopkg.in/yaml.v2"
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

// Add adds a path by its name
func (p Paths) Add(name string, path *Path) (Paths, error) {
	if err := path.Validate(); err != nil {
		return Paths{}, err
	}
	if _, found := p[name]; found {
		return Paths{}, fmt.Errorf("path with name %s already exists", name)
	}
	p[name] = path
	return p, nil
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
	Index    int          `yaml:"index,omitempty" json:"index,omitempty"`
}

// Validate checks that a path is valid
func (p *Path) Validate() (err error) {
	if err = p.Src.Validate(); err != nil {
		return err
	}
	if err = p.Dst.Validate(); err != nil {
		return err
	}
	if _, err = p.GetStrategy(); err != nil {
		return err
	}
	return nil
}

// Equal returns true if the path ends are equivelent, false otherwise
func (p *Path) Equal(path *Path) bool {
	if (p.Src.Equal(path.Src) || p.Src.Equal(path.Dst)) && (p.Dst.Equal(path.Src) || p.Dst.Equal(path.Dst)) {
		return true
	}
	return false
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
	return fmt.Sprintf("[%d] %s ->\n %s", p.Index, p.Src.String(), p.Dst.String())
}

// TODO: add Order chanTypes.Order as a property and wire it up in validation
// as well as in the transaction commands

// PathEnd represents the local connection identifers for a relay path
// The path is set on the chain before performing operations
type PathEnd struct {
	ChainID      string `yaml:"chain-id,omitempty" json:"chain-id,omitempty"`
	ClientID     string `yaml:"client-id,omitempty" json:"client-id,omitempty"`
	ConnectionID string `yaml:"connection-id,omitempty" json:"connection-id,omitempty"`
	ChannelID    string `yaml:"channel-id,omitempty" json:"channel-id,omitempty"`
	PortID       string `yaml:"port-id,omitempty" json:"port-id,omitempty"`
}

// Equal returns true if both path ends are equivelent, false otherwise
func (p *PathEnd) Equal(path *PathEnd) bool {
	if p.ChainID == path.ChainID && p.ClientID == path.ClientID && p.ConnectionID == path.ConnectionID && p.PortID == path.PortID && p.ChannelID == path.ChannelID {
		return true
	}
	return false
}
