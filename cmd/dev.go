package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/DataDog/datadog-go/statsd"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint/types"
	"github.com/iqlusioninc/relayer/relayer"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"
)

func devCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "development",
		Aliases: []string{"dev"},
		Short:   "commands for developers either deploying or hacking on the relayer",
	}
	cmd.AddCommand(
		gaiaServiceCmd(),
		faucetService(),
		rlyService(),
		listenCmd(),
		genesisCmd(),
		gozDataCmd(),
		gozCSVCmd(),
		gozStatsDCmd(),
		scorephase1b(),
	)
	return cmd
}

// ByAge implements sort.Interface for []Person based on
// the Age field.
type ByTrustPeriod []*clientData

func (a ByTrustPeriod) Len() int           { return len(a) }
func (a ByTrustPeriod) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByTrustPeriod) Less(i, j int) bool { return a[i].TrustPeriod < a[j].TrustPeriod }

func scorephase1b() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "phase1b [chain-id] [start-block] [end-block]",
		Aliases: []string{"csv"},
		Short:   "Find all the clients that live at the end of phase 1b",
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {

			cd, err := fetchClientData(args[0])
			if err != nil {
				return err
			}

			// Filter by clients that were alive in the starting block.

			c, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			startHeight, err := strconv.ParseInt(args[1], 10, 64)

			if err != nil {
				return err
			}
			startClients, err := liveClientsAtBlock(c, startHeight, cd)

			if err != nil {
				return err
			}

			endHeight, err := strconv.ParseInt(args[2], 10, 64)

			endClients, err := liveClientsAtBlock(c, endHeight, startClients)
			sort.Sort(ByTrustPeriod(endClients))

			w := csv.NewWriter(os.Stdout)

			for _, c := range endClients {
				w.Write([]string{c.ChainID, c.TrustPeriod.String(), c.ClientID})
			}

			w.Flush()

			return nil
		},
	}
	return cmd
}

func gozCSVCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "goz-csv [chain-id] [file]",
		Aliases: []string{"csv"},
		Short:   "read in source of truth csv, and enrich on chain w/ team data",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			to, err := readGoZCsv(args[1])
			if err != nil {
				return err
			}

			cd, err := fetchClientData(args[0])
			if err != nil {
				return err
			}
			w := csv.NewWriter(os.Stdout)

			for _, c := range cd {
				info := to[c.ChainID]
				if info != nil {
					c.TeamInfo = info
					w.Write([]string{c.TeamInfo.Name, c.TeamInfo.Address, c.TeamInfo.RPCAddr, c.ClientID})
				}
			}
			w.Flush()

			return nil
		},
	}
	return cmd
}

func gozStatsDCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "goz-statsd [chain-id] [file] [statsd-host] [statd-port]",
		Aliases: []string{"statsd"},
		Short:   "read in source of truth csv",
		Args:    cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {

			to, err := readGoZCsv(args[1])
			if err != nil {
				return err
			}
			client, err := statsd.New(args[2])
			if err != nil {
				return err
			}

			cd, err := fetchClientData(args[0])
			if err != nil {
				return err
			}
			for _, c := range cd {
				info := to[c.ChainID]
				c.TeamInfo = info
				c.StatsD(client, args[3])
			}
			return nil
		},
	}
	return cmd
}

func gozDataCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "goz-dump [chain-id]",
		Aliases: []string{"dump", "goz"},
		Short:   "fetch the list of chains connected as a CSV dump",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cd, err := fetchClientData(args[0])
			if err != nil {
				return err
			}
			out, _ := json.Marshal(cd)
			fmt.Println(string(out))
			return nil
		},
	}
	return cmd
}

func genesisCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "genesis [chain-id]",
		Aliases: []string{"gen"},
		Short:   "fetch the genesis file for a configured chain",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			gen, err := c.Client.Genesis()
			if err != nil {
				return err
			}

			return c.Print(gen, false, false)
		},
	}
	return cmd
}

// listenCmd represents the listen command
func listenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "listen [chain-id]",
		Aliases: []string{"l"},
		Short:   "listen to all transaction and block events from a given chain and output them to stdout",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := config.Chains.Get(args[0])
			if err != nil {
				return err
			}

			tx, err := cmd.Flags().GetBool(flagTx)
			if err != nil {
				return err
			}
			block, err := cmd.Flags().GetBool(flagBlock)
			if err != nil {
				return err
			}
			data, err := cmd.Flags().GetBool(flagData)
			if err != nil {
				return err
			}

			if block && tx {
				return fmt.Errorf("Must output block and/or tx")
			}

			done := c.ListenRPCEmitJSON(tx, block, data)

			trapSignal(done)

			return nil
		},
	}
	return listenFlags(cmd)
}

func gaiaServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gaia [user] [home]",
		Short: "gaia returns a sample gaiad service file",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf(`[Unit]
Description=gaiad
After=network.target
[Service]
Type=simple
User=%s
WorkingDirectory=%s
ExecStart=%s/go/bin/gaiad start --pruning=nothing
Restart=on-failure
RestartSec=3
LimitNOFILE=4096
[Install]
WantedBy=multi-user.target
`, args[0], args[1], args[1])
		},
	}
	return cmd
}

func faucetService() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "faucet [user] [home] [chain-id] [key-name] [amount]",
		Short: "faucet returns a sample faucet service file",
		Args:  cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.Chains.Get(args[2])
			if err != nil {
				return err
			}
			_, err = chain.Keybase.Key(args[3])
			if err != nil {
				return err
			}
			_, err = sdk.ParseCoin(args[4])
			if err != nil {
				return err
			}
			fmt.Printf(`[Unit]
Description=faucet
After=network.target
[Service]
Type=simple
User=%s
WorkingDirectory=%s
ExecStart=%s/go/bin/rly testnets faucet %s %s %s
Restart=on-failure
RestartSec=3
LimitNOFILE=4096
[Install]
WantedBy=multi-user.target
`, args[0], args[1], args[1], args[2], args[3], args[4])
			return nil
		},
	}
	return cmd
}

func rlyService() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "relayer [path-name]",
		Aliases: []string{"rly"},
		Short:   "relayer returns a service file for the relayer to relay over an individual path",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			user, home := os.Getenv("USER"), os.Getenv("HOME")
			if user == "" || home == "" {
				return fmt.Errorf("$USER(%s) or $HOME(%s) not set", user, home)
			}

			// ensure that path is configured
			path, err := config.Paths.Get(args[0])
			if err != nil {
				return err
			}

			// ensure that chains are configured
			src, dst := path.Src.ChainID, path.Dst.ChainID
			chains, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			// set paths on chains
			if err = chains[src].SetPath(path.Src); err != nil {
				return err
			}
			if err = chains[dst].SetPath(path.Dst); err != nil {
				return err
			}

			// ensure that keys exist
			if _, err = chains[src].GetAddress(); err != nil {
				return err
			}
			if _, err = chains[src].GetAddress(); err != nil {
				return err
			}

			// ensure that balances aren't == nil
			var srcBal, dstBal sdk.Coins
			if srcBal, err = chains[src].QueryBalance(chains[src].Key); err != nil {
				return err
			} else if srcBal.AmountOf(chains[src].DefaultDenom).IsZero() {
				return fmt.Errorf("no balance on %s, ensure %s has a balance before continuing setup", src, chains[src].MustGetAddress())
			}
			if dstBal, err = chains[dst].QueryBalance(chains[dst].Key); err != nil {
				return err
			} else if dstBal.AmountOf(chains[dst].DefaultDenom).IsZero() {
				return fmt.Errorf("no balance on %s, ensure %s has a balance before continuing setup", dst, chains[dst].MustGetAddress())
			}

			// ensure lite clients are initialized
			if _, err = chains[src].GetLatestLiteHeight(); err != nil {
				return fmt.Errorf("no lite client on %s, ensure it is initalized before continuing: %w", src, err)
			}
			if _, err = chains[dst].GetLatestLiteHeight(); err != nil {
				return fmt.Errorf("no lite client on %s, ensure it is initalized before continuing: %w", dst, err)
			}

			fmt.Printf(`[Unit]
Description=%s
After=network.target
[Service]
Type=simple
User=%s
WorkingDirectory=%s
ExecStart=%s/go/bin/rly start %s -d
Restart=on-failure
RestartSec=3
LimitNOFILE=4096
[Install]
WantedBy=multi-user.target
`, args[0], user, home, home, args[0])
			return nil
		},
	}
	return cmd
}

func readGoZCsv(path string) (map[string]*teamInfo, error) {
	// open the CSV file
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// create the csv reader
	cs := csv.NewReader(f)

	// ignore the header line
	if _, err := cs.Read(); err != nil {
		return nil, err
	}

	// read all the records into memory
	records, err := cs.ReadAll()
	if err != nil {
		return nil, err
	}

	// format the map[chain-id]Info
	var out = map[string]*teamInfo{}
	for _, r := range records {
		out[r[2]] = &teamInfo{r[0], r[1], r[3]}
	}

	return out, nil
}

type teamInfo struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	RPCAddr string `json:"rpc-addr"`
}

func fetchClientData(chainID string) ([]*clientData, error) {
	c, err := config.Chains.Get(chainID)
	if err != nil {
		return nil, err
	}

	clients, err := c.QueryClients(1, 1000)
	if err != nil {
		return nil, err
	}

	height, err := c.QueryLatestHeight()
	if err != nil {
		return nil, err
	}

	chans, err := c.QueryChannels(1, 10000)
	if err != nil {
		return nil, err
	}

	var clientDatas = []*clientData{}
	for _, cl := range clients {
		tmdata, ok := cl.(tmclient.ClientState)
		if !ok {
			continue
		}
		cd := &clientData{
			ClientID:         cl.GetID(),
			ChainID:          cl.GetChainID(),
			TimeOfLastUpdate: tmdata.LastHeader.Time,
			TrustPeriod:      tmdata.TrustingPeriod,
			ChannelIDs:       []string{},
		}

		if err := c.AddPath(cd.ClientID, dcon, dcha, dpor, dord); err != nil {
			return nil, err
		}

		conns, err := c.QueryConnectionsUsingClient(height)
		if err != nil {
			return nil, err
		}

		cd.ConnectionIDs = conns.ConnectionPaths
		for _, conn := range cd.ConnectionIDs {
			for _, ch := range chans {
				for _, co := range ch.ConnectionHops {
					if co == conn {
						cd.ChannelIDs = append(cd.ChannelIDs, ch.ID)
					}
				}
			}
		}

		// todo deal with channels
		clientDatas = append(clientDatas, cd)

	}
	return clientDatas, nil
}

type clientData struct {
	ClientID         string        `json:"client-id"`
	ConnectionIDs    []string      `json:"connection-ids"`
	ChannelIDs       []string      `json:"channel-ids"`
	ChainID          string        `json:"chain-id"`
	TimeOfLastUpdate time.Time     `json:"time-last-update"`
	TeamInfo         *teamInfo     `json:"team-info"`
	TrustPeriod      time.Duration `json:"trust-period"`
}

func (cd *clientData) StatsD(cl *statsd.Client, prefix string) {
	switch {
	case len(cd.ConnectionIDs) != 1:
		byt, _ := json.Marshal(cd)
		fmt.Fprintf(os.Stderr, "%s", string(byt))
	case len(cd.ChannelIDs) != 1:
		byt, _ := json.Marshal(cd)
		fmt.Fprintf(os.Stderr, "%s", string(byt))
		// TODO: add more cases here
	}
	cl.TimeInMilliseconds(fmt.Sprintf("relayer.%s.client", prefix), float64(time.Since(cd.TimeOfLastUpdate).Milliseconds()), []string{"teamname", cd.TeamInfo.Name, "chain-id", cd.ChainID, "client-id", cd.ClientID, "connection-id", cd.ConnectionIDs[0], "channelid", cd.ChannelIDs[0]}, 1)
}

func liveClientsAtBlock(c *relayer.Chain, height int64, cd []*clientData) ([]*clientData, error) {
	var vcs []*clientData
	stat, err := c.Client.Block(&height)
	if err != nil {
		return nil, err
	}

	for _, client := range cd {
		cs, err := c.QueryClientStateHeight(client.ClientID, height)
		if err != nil {
			return nil, err
		}
		if cs != nil {
			cl := cs.ClientState.(tmclient.ClientState)
			if stat.Block.Time.Sub(cl.LastHeader.Time) < cl.TrustingPeriod {
				vcs = append(vcs, client)
			}
		}
	}

	return vcs, nil
}
