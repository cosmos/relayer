package relayer

import (
	"fmt"

	host "github.com/cosmos/cosmos-sdk/x/ibc/24-host"
)

type Paths []Path

// Path represents a pair of chains and the identifiers needed to
// relay over them
type Path struct {
	Src PathEnd `yaml:"src" json:"src"`
	Dst PathEnd `yaml:"dst" json:"dst"`
}

func (p Path) String() string {
	return fmt.Sprintf("%s ->\n %s", p.Src.String(), p.Dst.String())
}

// PathEnd represents the local connection identifers for a relay path
// The path is set on the chain before performing operations
type PathEnd struct {
	ChainID      string `yaml:"chain-id,omitempty" json:"chain-id,omitempty"`
	ClientID     string `yaml:"client-id,omitempty" json:"client-id,omitempty"`
	ConnectionID string `yaml:"connection-id,omitempty" json:"connection-id,omitempty"`
	ChannelID    string `yaml:"channel-id,omitempty" json:"channel-id,omitempty"`
	PortID       string `yaml:"port-id,omitempty" json:"port-id,omitempty"`
}

func (p *PathEnd) validateClient() error {
	return host.DefaultClientIdentifierValidator(p.ClientID)
}

func (p *PathEnd) validateConnection() error {
	return host.DefaultConnectionIdentifierValidator(p.ConnectionID)
}

func (p *PathEnd) validateChannel() error {
	return host.DefaultChannelIdentifierValidator(p.ChannelID)
}

func (p *PathEnd) validatePort() error {
	return host.DefaultPortIdentifierValidator(p.PortID)
}

func (p PathEnd) String() string {
	return fmt.Sprintf("client{%s}-conn{%s}-chan{%s}@chain{%s}:port{%s}", p.ClientID, p.ConnectionID, p.ChannelID, p.ChainID, p.PortID)
}

// pathType helps define what validations need to be run
type pathType byte

const (
	CLNTPATH pathType = iota
	CONNPATH
	CHANPATH
	CLNTCHANPATH
	FULLPATH
)

func (p pathType) String() string {
	switch p {
	case CLNTPATH:
		return "CLNTPATH"
	case CONNPATH:
		return "CONNPATH"
	case CHANPATH:
		return "CHANPATH"
	case CLNTCHANPATH:
		return "CLNTCHANPATH"
	case FULLPATH:
		return "FULLPATH"
	default:
		return "shouldn't be here"
	}
}

// PathClient used to set the path for client creation commands
func (c *Chain) PathClient(clientID string) error {
	return c.setPath(&PathEnd{
		ChainID:  c.ChainID,
		ClientID: clientID,
	}, CLNTPATH)
}

// PathConnection used to set the path for the connection creation commands
func (c *Chain) PathConnection(clientID, connectionID string) error {
	return c.setPath(&PathEnd{
		ChainID:      c.ChainID,
		ClientID:     clientID,
		ConnectionID: connectionID,
	}, CONNPATH)
}

// PathChannel used to set the path for the channel creation commands
func (c *Chain) PathChannel(channelID, portID string) error {
	return c.setPath(&PathEnd{
		ChannelID: channelID,
		PortID:    portID,
	}, CHANPATH)
}

// PathChannelClient used to set the path for channel and client commands
func (c *Chain) PathChannelClient(clientID, channelID, portID string) error {
	return c.setPath(&PathEnd{
		ClientID:  clientID,
		ChannelID: channelID,
		PortID:    portID,
	}, CLNTCHANPATH)
}

// SetFULLPATh sets all of the properties on the path
func (c *Chain) FullPath(clientID, connectionID, channelID, portID string) error {
	return c.setPath(&PathEnd{
		ChainID:      c.ChainID,
		ClientID:     clientID,
		ConnectionID: connectionID,
		ChannelID:    channelID,
		PortID:       portID,
	}, FULLPATH)
}

// PathSet check if the chain has a path set
func (c *Chain) PathSet() bool {
	if c.PathEnd == nil {
		return false
	}
	return true
}

// PathsSet checks if the chains have their paths set
func PathsSet(chains ...*Chain) bool {
	for _, c := range chains {
		if !c.PathSet() {
			return false
		}
	}
	return true
}

func (c *Chain) setPath(p *PathEnd, t pathType) error {
	err := p.Validate(t)
	if err != nil {
		return err
	}
	c.PathEnd = p
	return nil
}

func (p *PathEnd) Validate(t pathType) error {
	// TODO: validate chain ID here too?
	switch t {
	case CLNTPATH:
		if err := p.validateClient(); err != nil {
			return err
		}
		return nil
	case CONNPATH:
		if err := p.validateClient(); err != nil {
			return err
		}
		if err := p.validateConnection(); err != nil {
			return err
		}
		return nil
	case CHANPATH:
		if err := p.validateChannel(); err != nil {
			return err
		}
		if err := p.validatePort(); err != nil {
			return err
		}
		return nil
	case CLNTCHANPATH:
		if err := p.validateClient(); err != nil {
			return err
		}
		if err := p.validateChannel(); err != nil {
			return err
		}
		if err := p.validatePort(); err != nil {
			return err
		}
		return nil
	case FULLPATH:
		if err := p.validateClient(); err != nil {
			return err
		}
		if err := p.validateConnection(); err != nil {
			return err
		}
		if err := p.validateChannel(); err != nil {
			return err
		}
		if err := p.validatePort(); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("invalid path type: %s", t.String())
	}
}

// ErrorPathNotSet returns information what identifiers are needed to relay
func (c *Chain) ErrPathNotSet(t pathType, err error) error {
	return fmt.Errorf("Path of type %s on chain %s not set: %w", t.String(), c.ChainID, err)
}

func (c *Chain) ErrCantSetPath(t pathType, err error) error {
	return fmt.Errorf("Path of type %s on chain %s failed to set: %w", t.String(), c.ChainID, err)
}
