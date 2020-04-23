package relayer

import (
	"fmt"

	chanState "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/exported"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint/types"
	"gopkg.in/yaml.v2"
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
	return p.Src.getOrder() == chanState.ORDERED
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
	if p.Src.Order != p.Dst.Order {
		return fmt.Errorf("Both sides must have same order ('ORDERED' or 'UNORDERED'), got src(%s) and dst(%s)", p.Src.Order, p.Dst.Order)
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
func GenPath(srcChainID, dstChainID, srcPortID, dstPortID, order string) *Path {
	return &Path{
		Src: &PathEnd{
			ChainID:      srcChainID,
			ClientID:     RandLowerCaseLetterString(10),
			ConnectionID: RandLowerCaseLetterString(10),
			ChannelID:    RandLowerCaseLetterString(10),
			PortID:       srcPortID,
			Order:        order,
		},
		Dst: &PathEnd{
			ChainID:      dstChainID,
			ClientID:     RandLowerCaseLetterString(10),
			ConnectionID: RandLowerCaseLetterString(10),
			ChannelID:    RandLowerCaseLetterString(10),
			PortID:       dstPortID,
			Order:        order,
		},
		Strategy: &StrategyCfg{
			Type: "naive",
		},
	}
}

// FindPaths returns all the open paths that exist between chains
func FindPaths(chains Chains) (*Paths, error) {
	var out = &Paths{}
	hs, err := QueryLatestHeights(chains...)
	if err != nil {
		return nil, err
	}
	for _, src := range chains {
		clients, err := src.QueryClients(1, 1000)
		if err != nil {
			return nil, err
		}
		for _, client := range clients {
			clnt, ok := client.(tmclient.ClientState)
			if !ok || clnt.LastHeader.Commit == nil || clnt.LastHeader.Header == nil {
				continue
			}
			dst, err := chains.Get(client.GetChainID())
			if err != nil {
				continue
			}

			if err = src.AddPath(client.GetID(), dcon, dcha, dpor, "ORDERED"); err != nil {
				return nil, err
			}

			conns, err := src.QueryConnectionsUsingClient(hs[src.ChainID])
			if err != nil {
				return nil, err
			}

			for _, connid := range conns.ConnectionPaths {
				if err = src.AddPath(client.GetID(), connid, dcha, dpor, "ORDERED"); err != nil {
					return nil, err
				}
				conn, err := src.QueryConnection(hs[src.ChainID])
				if err != nil {
					return nil, err
				}
				if conn.Connection.Connection.GetState().String() == "OPEN" {
					chans, err := src.QueryConnectionChannels(connid, 1, 1000)
					if err != nil {
						return nil, err
					}
					for _, chn := range chans {
						if chn.Channel.State.String() == "OPEN" {
							p := &Path{
								Src: &PathEnd{
									ChainID:      src.ChainID,
									ClientID:     client.GetID(),
									ConnectionID: conn.Connection.Identifier,
									ChannelID:    chn.ChannelIdentifier,
									PortID:       chn.PortIdentifier,
								},
								Dst: &PathEnd{
									ChainID:      dst.ChainID,
									ClientID:     conn.Connection.Connection.GetCounterparty().GetClientID(),
									ConnectionID: conn.Connection.Connection.GetCounterparty().GetConnectionID(),
									ChannelID:    chn.Channel.GetCounterparty().GetChannelID(),
									PortID:       chn.Channel.GetCounterparty().GetPortID(),
								},
								Strategy: &StrategyCfg{
									Type: "naive",
								},
							}
							if err = out.Add(fmt.Sprintf("%s-%s", src.ChainID, dst.ChainID), p); err != nil {
								return nil, err
							}
						}
					}
				}
			}
		}
	}
	return out, nil
}
