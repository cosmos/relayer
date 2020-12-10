package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	clienttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/types"
	conntypes "github.com/cosmos/cosmos-sdk/x/ibc/core/03-connection/types"
	chantypes "github.com/cosmos/cosmos-sdk/x/ibc/core/04-channel/types"
	"github.com/cosmos/cosmos-sdk/x/ibc/core/exported"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v2"
)

func pathsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "paths",
		Aliases: []string{"pth"},
		Short:   "manage path configurations",
		Long: `
A path represents the "full path" or "link" for communication between two chains. This includes the client, 
connection, and channel ids from both the source and destination chains as well as the strategy to use when relaying`,
	}

	cmd.AddCommand(
		pathsListCmd(),
		pathsShowCmd(),
		pathsAddCmd(),
		pathsGenCmd(),
		pathsDeleteCmd(),
	)

	return cmd
}

func pathsGenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "generate [src-chain-id] [dst-chain-id] [name]",
		Aliases: []string{"gen"},
		Short:   "generate identifiers for a new path between src and dst, reusing any that exist",
		Args:    cobra.ExactArgs(3),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s paths generate ibc-0 ibc-1 demo-path --force
$ %s pth gen ibc-0 ibc-1 demo-path --unordered false --version ics20-2`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			var (
				src, dst, pth          = args[0], args[1], args[2]
				c                      map[string]*relayer.Chain
				eg                     errgroup.Group
				srcClients, dstClients *clienttypes.QueryClientStatesResponse
				srcConns, dstConns     *conntypes.QueryConnectionsResponse
				srcCon, dstCon         *conntypes.IdentifiedConnection
				srcChans, dstChans     *chantypes.QueryChannelsResponse
				srcChan, dstChan       *chantypes.IdentifiedChannel
			)
			if c, err = config.Chains.Gets(src, dst); err != nil {
				return fmt.Errorf("chains need to be configured before paths to them can be added: %w", err)
			}
			version, _ := cmd.Flags().GetString(flagVersion)
			strategy, _ := cmd.Flags().GetString(flagStrategy)
			port, _ := cmd.Flags().GetString(flagPort)
			path := &relayer.Path{
				Src:      &relayer.PathEnd{ChainID: src, PortID: port, Version: version},
				Dst:      &relayer.PathEnd{ChainID: dst, PortID: port, Version: version},
				Strategy: &relayer.StrategyCfg{Type: strategy},
			}

			// get desired order of the channel
			if unordered, _ := cmd.Flags().GetBool(flagOrder); unordered {
				path.Src.Order = UNORDERED
				path.Dst.Order = UNORDERED
			} else {
				path.Src.Order = ORDERED
				path.Dst.Order = ORDERED
			}

			// if -f is passed, generate a random path between the two chains
			if force, _ := cmd.Flags().GetBool(flagForce); force {
				path.GenSrcClientID()
				path.GenDstClientID()
				path.GenSrcConnID()
				path.GenDstConnID()
				path.GenSrcChanID()
				path.GenDstChanID()
				// validate it...
				if err = config.Paths.AddForce(pth, path); err != nil {
					return err
				}
				logPathGen(pth)
				// ...then add it to the config file
				return overWriteConfig(cmd, config)
			}

			// see if there are existing clients that can be reused
			eg.Go(func() error {
				srcClients, err = c[src].QueryClients(0, 1000)
				return err
			})
			eg.Go(func() error {
				dstClients, err = c[dst].QueryClients(0, 1000)
				return err
			})
			if eg.Wait(); err != nil {
				return err
			}

			for _, idCs := range srcClients.ClientStates {
				var clnt exported.ClientState
				if err = idCs.UnpackInterfaces(c[src].Encoding.Marshaler); err != nil {
					return err
				}
				if clnt, err = clienttypes.UnpackClientState(idCs.ClientState); err != nil {
					return err
				}
				switch cs := clnt.(type) {
				case *tmclient.ClientState:
					// if the client is an active tendermint client for the counterparty chain then we reuse it
					if cs.ChainId == c[dst].ChainID && !cs.IsFrozen() {
						path.Src.ClientID = idCs.ClientId
					}
				default:
				}
			}

			for _, idCs := range dstClients.ClientStates {
				var clnt exported.ClientState
				if err = idCs.UnpackInterfaces(c[dst].Encoding.Marshaler); err != nil {
					return err
				}
				if clnt, err = clienttypes.UnpackClientState(idCs.ClientState); err != nil {
					return err
				}
				switch cs := clnt.(type) {
				case *tmclient.ClientState:
					// if the client is an active tendermint client for the counterparty chain then we reuse it
					if cs.ChainId == c[src].ChainID && !cs.IsFrozen() {
						path.Dst.ClientID = idCs.ClientId
					}
				default:
				}
			}

			switch {
			// If there aren't any matching clients between chains, generate
			case path.Src.ClientID == "" && path.Dst.ClientID == "":
				path.GenSrcClientID()
				path.GenDstClientID()
				path.GenSrcConnID()
				path.GenSrcConnID()
				path.GenSrcChanID()
				path.GenDstChanID()
				if err = config.Paths.Add(pth, path); err != nil {
					return err
				}
				logPathGen(pth)
				return overWriteConfig(cmd, config)
			case path.Src.ClientID == "" && path.Dst.ClientID != "":
				path.GenSrcClientID()
				path.GenSrcConnID()
				path.GenSrcConnID()
				path.GenSrcChanID()
				path.GenDstChanID()
				if err = config.Paths.Add(pth, path); err != nil {
					return err
				}
				logPathGen(pth)
				return overWriteConfig(cmd, config)
			case path.Dst.ClientID == "" && path.Src.ClientID != "":
				path.GenDstClientID()
				path.GenSrcConnID()
				path.GenSrcConnID()
				path.GenSrcChanID()
				path.GenDstChanID()
				if err = config.Paths.Add(pth, path); err != nil {
					return err
				}
				logPathGen(pth)
				return overWriteConfig(cmd, config)
			}

			// see if there are existing connections that can be reused
			eg.Go(func() error {
				srcConns, err = c[src].QueryConnections(0, 1000)
				return err
			})
			eg.Go(func() error {
				dstConns, err = c[dst].QueryConnections(0, 1000)
				return err
			})
			if err = eg.Wait(); err != nil {
				return err
			}

			// find connections with the appropriate client id
			for _, c := range srcConns.Connections {
				if c.ClientId == path.Src.ClientID {
					srcCon = c
					path.Src.ConnectionID = c.Id
				}
			}
			for _, c := range dstConns.Connections {
				if c.ClientId == path.Dst.ClientID {
					dstCon = c
					path.Dst.ConnectionID = c.Id
				}
			}

			switch {
			case path.Src.ConnectionID != "" && path.Dst.ConnectionID != "":
				// If we have identified a connection, make sure that each end is the
				// other's counterparty and that the connection is open. In the failure case
				// we should generate a new connection identifier
				dstCpForSrc := srcCon.Counterparty.ConnectionId == dstCon.Id
				srcCpForDst := dstCon.Counterparty.ConnectionId == srcCon.Id
				srcOpen := srcCon.State == conntypes.OPEN
				dstOpen := dstCon.State == conntypes.OPEN
				if !(dstCpForSrc && srcCpForDst && srcOpen && dstOpen) {
					path.GenSrcConnID()
					path.GenDstConnID()
					path.GenSrcChanID()
					path.GenDstChanID()
					if err = config.Paths.Add(pth, path); err != nil {
						return err
					}
					logPathGen(pth)
					return overWriteConfig(cmd, config)
				}
			default:
				path.GenSrcConnID()
				path.GenDstConnID()
				path.GenSrcChanID()
				path.GenDstChanID()
				if err = config.Paths.Add(pth, path); err != nil {
					return err
				}
				logPathGen(pth)
				return overWriteConfig(cmd, config)
			}

			eg.Go(func() error {
				srcChans, err = c[src].QueryChannels(0, 1000)
				return err
			})
			eg.Go(func() error {
				dstChans, err = c[dst].QueryChannels(0, 1000)
				return err
			})
			if err = eg.Wait(); err != nil {
				return err
			}

			for _, c := range srcChans.Channels {
				if c.ConnectionHops[0] == path.Src.ConnectionID {
					srcChan = c
					path.Src.ChannelID = c.ChannelId
				}
			}

			for _, c := range dstChans.Channels {
				if c.ConnectionHops[0] == path.Dst.ConnectionID {
					dstChan = c
					path.Dst.ChannelID = c.ChannelId
				}
			}

			switch {
			case path.Src.ChannelID != "" && path.Dst.ChannelID != "":
				// If we have identified a channel, make sure that each end is the
				// other's counterparty and that the channel is open. In the failure case
				// we should generate a new channel identifier
				dstCpForSrc := srcChan.Counterparty.ChannelId == dstChan.ChannelId
				srcCpForDst := dstChan.Counterparty.ChannelId == srcChan.ChannelId
				srcOpen := srcChan.State == chantypes.OPEN
				dstOpen := dstChan.State == chantypes.OPEN
				srcPort := srcChan.PortId == path.Src.PortID
				dstPort := dstChan.PortId == path.Dst.PortID
				srcOrder := srcChan.Ordering == path.Src.GetOrder()
				dstOrder := dstChan.Ordering == path.Dst.GetOrder()
				srcVersion := srcChan.Version == path.Src.Version
				dstVersion := dstChan.Version == path.Dst.Version
				if !(dstCpForSrc && srcCpForDst && srcOpen && dstOpen && srcPort && dstPort && srcOrder && dstOrder && srcVersion && dstVersion) {
					path.GenSrcChanID()
					path.GenDstChanID()
				}
				if err = config.Paths.Add(pth, path); err != nil {
					return err
				}
				logPathGen(pth)
				return overWriteConfig(cmd, config)
			default:
				path.GenSrcChanID()
				path.GenDstChanID()
				if err = config.Paths.Add(pth, path); err != nil {
					return err
				}
				logPathGen(pth)
				return overWriteConfig(cmd, config)
			}
		},
	}
	return forceFlag(orderFlag(versionFlag(pathStrategy(portFlag(cmd)))))
}

func logPathGen(pth string) {
	fmt.Printf("Generated path(%s), run 'rly paths show %s --yaml' to see details\n", pth, pth)
}

func pathsDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete [index]",
		Aliases: []string{"d"},
		Short:   "delete a path with a given index",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s paths delete demo-path
$ %s pth d path-name`, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := config.Paths.Get(args[0]); err != nil {
				return err
			}
			cfg := config
			delete(cfg.Paths, args[0])
			return overWriteConfig(cmd, cfg)
		},
	}
	return cmd
}

func pathsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"l"},
		Short:   "print out configured paths",
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s paths list --yaml
$ %s paths list --json
$ %s pth l`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			jsn, _ := cmd.Flags().GetBool(flagJSON)
			yml, _ := cmd.Flags().GetBool(flagYAML)
			switch {
			case yml && jsn:
				return fmt.Errorf("can't pass both --json and --yaml, must pick one")
			case yml:
				out, err := yaml.Marshal(config.Paths)
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			case jsn:
				out, err := json.Marshal(config.Paths)
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			default:
				i := 0
				for k, pth := range config.Paths {
					chains, err := config.Chains.Gets(pth.Src.ChainID, pth.Dst.ChainID)
					if err != nil {
						return err
					}
					stat := pth.QueryPathStatus(chains[pth.Src.ChainID], chains[pth.Dst.ChainID]).Status
					printPath(i, k, pth, checkmark(stat.Chains), checkmark(stat.Clients), checkmark(stat.Connection), checkmark(stat.Channel))
					i++
				}
				return nil
			}
		},
	}
	return yamlFlag(jsonFlag(cmd))
}

func printPath(i int, k string, pth *relayer.Path, chains, clients, connection, channel string) {
	fmt.Printf("%2d: %-20s -> chns(%s) clnts(%s) conn(%s) chan(%s) (%s:%s<>%s:%s)\n",
		i, k, chains, clients, connection, channel, pth.Src.ChainID, pth.Src.PortID, pth.Dst.ChainID, pth.Dst.PortID)
}

func checkmark(status bool) string {
	if status {
		return check
	}
	return xIcon
}

func pathsShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show [path-name]",
		Aliases: []string{"s"},
		Short:   "show a path given its name",
		Args:    cobra.ExactArgs(1),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s paths show demo-path --yaml
$ %s paths show demo-path --json
$ %s pth s path-name`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.Paths.Get(args[0])
			if err != nil {
				return err
			}
			chains, err := config.Chains.Gets(path.Src.ChainID, path.Dst.ChainID)
			if err != nil {
				return err
			}
			jsn, _ := cmd.Flags().GetBool(flagJSON)
			yml, _ := cmd.Flags().GetBool(flagYAML)
			pathWithStatus := path.QueryPathStatus(chains[path.Src.ChainID], chains[path.Dst.ChainID])
			switch {
			case yml && jsn:
				return fmt.Errorf("can't pass both --json and --yaml, must pick one")
			case yml:
				out, err := yaml.Marshal(pathWithStatus)
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			case jsn:
				out, err := json.Marshal(pathWithStatus)
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			default:
				fmt.Println(pathWithStatus.PrintString(args[0]))
			}

			return nil
		},
	}
	return yamlFlag(jsonFlag(cmd))
}

func pathsAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add [src-chain-id] [dst-chain-id] [path-name]",
		Aliases: []string{"a"},
		Short:   "add a path to the list of paths",
		Args:    cobra.ExactArgs(3),
		Example: strings.TrimSpace(fmt.Sprintf(`
$ %s paths add ibc-0 ibc-1 demo-path
$ %s paths add ibc-0 ibc-1 demo-path --file paths/demo.json
$ %s pth a ibc-0 ibc-1 demo-path`, appName, appName, appName)),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			_, err := config.Chains.Gets(src, dst)
			if err != nil {
				return fmt.Errorf("chains need to be configured before paths to them can be added: %w", err)
			}

			var out *Config
			file, err := cmd.Flags().GetString(flagFile)
			if err != nil {
				return err
			}

			if file != "" {
				if out, err = fileInputPathAdd(file, args[2]); err != nil {
					return err
				}
			} else {
				if out, err = userInputPathAdd(src, dst, args[2]); err != nil {
					return err
				}
			}

			return overWriteConfig(cmd, out)
		},
	}
	return fileFlag(cmd)
}

func fileInputPathAdd(file, name string) (cfg *Config, err error) {
	// If the user passes in a file, attempt to read the chain config from that file
	p := &relayer.Path{}
	if _, err := os.Stat(file); err != nil {
		return nil, err
	}

	byt, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(byt, &p); err != nil {
		return nil, err
	}

	if err = config.Paths.Add(name, p); err != nil {
		return nil, err
	}

	return config, nil
}

func userInputPathAdd(src, dst, name string) (*Config, error) {
	var (
		value string
		err   error
		path  = &relayer.Path{
			Strategy: relayer.NewNaiveStrategy(),
			Src: &relayer.PathEnd{
				ChainID: src,
				Order:   "ORDERED",
			},
			Dst: &relayer.PathEnd{
				ChainID: dst,
				Order:   "ORDERED",
			},
		}
	)

	fmt.Printf("enter src(%s) client-id...\n", src)
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	path.Src.ClientID = value

	if err = path.Src.Vclient(); err != nil {
		return nil, err
	}

	fmt.Printf("enter src(%s) connection-id...\n", src)
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	path.Src.ConnectionID = value

	if err = path.Src.Vconn(); err != nil {
		return nil, err
	}

	fmt.Printf("enter src(%s) channel-id...\n", src)
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	path.Src.ChannelID = value

	if err = path.Src.Vchan(); err != nil {
		return nil, err
	}

	fmt.Printf("enter src(%s) port-id...\n", src)
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	path.Src.PortID = value

	if err = path.Src.Vport(); err != nil {
		return nil, err
	}

	fmt.Printf("enter src(%s) version...\n", src)
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	path.Src.Version = value

	if err = path.Src.Vversion(); err != nil {
		return nil, err
	}

	fmt.Printf("enter dst(%s) client-id...\n", dst)
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	path.Dst.ClientID = value

	if err = path.Dst.Vclient(); err != nil {
		return nil, err
	}

	fmt.Printf("enter dst(%s) connection-id...\n", dst)
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	path.Dst.ConnectionID = value

	if err = path.Dst.Vconn(); err != nil {
		return nil, err
	}

	fmt.Printf("enter dst(%s) channel-id...\n", dst)
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	path.Dst.ChannelID = value

	if err = path.Dst.Vchan(); err != nil {
		return nil, err
	}

	fmt.Printf("enter dst(%s) port-id...\n", dst)
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	path.Dst.PortID = value

	if err = path.Dst.Vport(); err != nil {
		return nil, err
	}

	fmt.Printf("enter dst(%s) version...\n", dst)
	if value, err = readStdin(); err != nil {
		return nil, err
	}

	path.Dst.Version = value

	if err = path.Dst.Vversion(); err != nil {
		return nil, err
	}

	if err = config.Paths.Add(name, path); err != nil {
		return nil, err
	}

	return config, nil
}
