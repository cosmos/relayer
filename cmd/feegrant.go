package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// feegrantConfigureCmd returns the fee grant configuration commands for this module
func feegrantConfigureBaseCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feegrant",
		Short: "Configure the client to use round-robin feegranted accounts when sending TXs",
		Long:  "Use round-robin feegranted accounts when sending TXs. Useful for relayers and applications where sequencing is important",
	}

	cmd.AddCommand(
		feegrantConfigureBasicCmd(a),
	)

	return cmd
}

func feegrantConfigureBasicCmd(a *appState) *cobra.Command {
	var numGrantees int
	var update bool
	var updateGrantees bool
	var grantees []string

	cmd := &cobra.Command{
		Use:   "basicallowance [chain-name] [granter] --num-grantees [int] --overwrite-granter --overwrite-grantees",
		Short: "feegrants for the given chain and granter (if granter is unspecified, use the default key)",
		Long:  "feegrants for the given chain. 10 grantees by default, all with an unrestricted BasicAllowance.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain := args[0]
			cosmosChain, ok := a.Config.Chains[chain]
			if !ok {
				return errChainNotFound(args[0])
			}

			prov, ok := cosmosChain.ChainProvider.(*cosmos.CosmosProvider)
			if !ok {
				return errors.New("only CosmosProvider can be feegranted")
			}

			granterKeyOrAddr := ""

			if len(args) > 1 {
				granterKeyOrAddr = args[1]
			} else if prov.PCfg.FeeGrants != nil {
				granterKeyOrAddr = prov.PCfg.FeeGrants.GranterKey
			} else {
				granterKeyOrAddr = prov.PCfg.Key
			}

			granterKey, err := prov.KeyFromKeyOrAddress(granterKeyOrAddr)
			if err != nil {
				return fmt.Errorf("could not get granter key from '%s'", granterKeyOrAddr)
			}

			if prov.PCfg.FeeGrants != nil && granterKey != prov.PCfg.FeeGrants.GranterKey && !update {
				return fmt.Errorf("you specified granter '%s' which is different than configured feegranter '%s', but you did not specify the --overwrite-granter flag", granterKeyOrAddr, prov.PCfg.FeeGrants.GranterKey)
			} else if prov.PCfg.FeeGrants != nil && granterKey != prov.PCfg.FeeGrants.GranterKey && update {
				prov.PCfg.FeeGrants.GranterKey = granterKey
				cfgErr := a.OverwriteConfig(a.Config)
				cobra.CheckErr(cfgErr)
			}

			if prov.PCfg.FeeGrants == nil || updateGrantees || len(grantees) > 0 {
				var feegrantErr error

				//No list of grantees was provided, so we will use the default naming convention "grantee1, ... granteeN"
				if grantees == nil {
					feegrantErr = prov.ConfigureFeegrants(numGrantees, granterKey)
				} else {
					feegrantErr = prov.ConfigureWithGrantees(grantees, granterKey)
				}

				if feegrantErr != nil {
					return feegrantErr
				}

				if feegrantErr != nil {
					return feegrantErr
				}

				cfgErr := a.OverwriteConfig(a.Config)
				cobra.CheckErr(cfgErr)
			}

			memo, err := cmd.Flags().GetString(flagMemo)
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			_, err = prov.EnsureBasicGrants(ctx, memo)
			if err != nil {
				return fmt.Errorf("error writing grants on chain: '%s'", err.Error())
			}

			//Get latest height from the chain, mark feegrant configuration as verified up to that height.
			//This means we've verified feegranting is enabled on-chain and TXs can be sent with a feegranter.
			if prov.PCfg.FeeGrants != nil {
				fmt.Printf("Querying latest chain height to mark FeeGrant height... \n")
				h, err := prov.QueryLatestHeight(ctx)
				cobra.CheckErr(err)
				prov.PCfg.FeeGrants.BlockHeightVerified = h
				fmt.Printf("Feegrant chain height marked: %d\n", h)
				cfgErr := a.OverwriteConfig(a.Config)
				cobra.CheckErr(cfgErr)
			}

			return nil
		},
	}
	cmd.Flags().BoolVar(&update, "overwrite-granter", false, "allow overwriting the existing granter")
	cmd.Flags().BoolVar(&updateGrantees, "overwrite-grantees", false, "allow overwriting existing grantees")
	cmd.Flags().IntVar(&numGrantees, "num-grantees", 10, "number of grantees that will be feegranted with basic allowances")
	cmd.Flags().StringSliceVar(&grantees, "grantees", []string{}, "comma separated list of grantee key names (keys are created if they do not exist)")
	cmd.MarkFlagsMutuallyExclusive("num-grantees", "grantees")

	memoFlag(a.Viper, cmd)
	return cmd
}

func feegrantBasicGrantsCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "basic chain-name [granter]",
		Short: "query the grants for an account (if none is specified, the default account is returned)",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain := args[0]
			cosmosChain, ok := a.Config.Chains[chain]
			if !ok {
				return errChainNotFound(args[0])
			}

			prov, ok := cosmosChain.ChainProvider.(*cosmos.CosmosProvider)
			if !ok {
				return errors.New("only CosmosProvider can be feegranted")
			}

			// TODO fix pagination
			// pageReq, err := client.ReadPageRequest(cmd.Flags())
			// if err != nil {
			// 	return err
			// }

			//TODO fix height
			// height, err := lensCmd.ReadHeight(cmd.Flags())
			// if err != nil {
			// 	return err
			// }

			keyNameOrAddress := ""
			if len(args) == 0 {
				keyNameOrAddress = prov.PCfg.Key
			} else {
				keyNameOrAddress = args[0]
			}

			granterAcc, err := prov.AccountFromKeyOrAddress(keyNameOrAddress)
			if err != nil {
				fmt.Printf("Error retrieving account from key '%s'\n", keyNameOrAddress)
				return err
			}
			granterAddr := prov.MustEncodeAccAddr(granterAcc)

			res, err := prov.QueryFeegrantsByGranter(granterAddr, nil)
			if err != nil {
				return err
			}

			for _, grant := range res {
				allowance, e := prov.Sprint(grant.Allowance)
				cobra.CheckErr(e)
				fmt.Printf("Granter: %s, Grantee: %s, Allowance: %s\n", grant.Granter, grant.Grantee, allowance)
			}

			return nil
		},
	}
	return paginationFlags(a.Viper, cmd, "feegrant")
}

func TestFeegrant(log *zap.Logger, home string, chain string, granterKeyOrAddr string, update bool, updateGrantees bool, numGrantees int, grantees []string) error {

	var rootCmd = &cobra.Command{
		Use:   appName,
		Short: "This application makes data relay between IBC enabled chains easy!",
		Long: strings.TrimSpace(`rly has:
   1. Configuration management for Chains and Paths
   2. Key management for managing multiple keys for multiple chains
   3. Query and transaction functionality for IBC

   NOTE: Most of the commands have aliases that make typing them much quicker 
         (i.e. 'rly tx', 'rly q', etc...)`),
	}

	a := &appState{
		Viper:    viper.New(),
		HomePath: home,
		Log:      log,
	}

	err := initConfig(rootCmd, a)
	if err != nil {
		return err
	}

	cosmosChain, ok := a.Config.Chains[chain]
	if !ok {
		return errChainNotFound(chain)
	}

	prov, ok := cosmosChain.ChainProvider.(*cosmos.CosmosProvider)
	if !ok {
		return errors.New("only CosmosProvider can be feegranted")
	}

	if prov.PCfg.FeeGrants != nil {
		granterKeyOrAddr = prov.PCfg.FeeGrants.GranterKey
	} else if granterKeyOrAddr == "" {
		granterKeyOrAddr = prov.PCfg.Key
	}

	granterKey, err := prov.KeyFromKeyOrAddress(granterKeyOrAddr)
	if err != nil {
		return fmt.Errorf("could not get granter key from '%s'", granterKeyOrAddr)
	}

	if prov.PCfg.FeeGrants != nil && granterKey != prov.PCfg.FeeGrants.GranterKey && !update {
		return fmt.Errorf("you specified granter '%s' which is different than configured feegranter '%s', but you did not specify the --overwrite-granter flag", granterKeyOrAddr, prov.PCfg.FeeGrants.GranterKey)
	} else if prov.PCfg.FeeGrants != nil && granterKey != prov.PCfg.FeeGrants.GranterKey && update {
		prov.PCfg.FeeGrants.GranterKey = granterKey
		cfgErr := a.OverwriteConfig(a.Config)
		cobra.CheckErr(cfgErr)
	}

	if prov.PCfg.FeeGrants == nil || updateGrantees || len(grantees) > 0 {
		var feegrantErr error

		//No list of grantees was provided, so we will use the default naming convention "grantee1, ... granteeN"
		if grantees == nil {
			feegrantErr = prov.ConfigureFeegrants(numGrantees, granterKey)
		} else {
			feegrantErr = prov.ConfigureWithGrantees(grantees, granterKey)
		}

		if feegrantErr != nil {
			return feegrantErr
		}

		if feegrantErr != nil {
			return feegrantErr
		}

		cfgErr := a.OverwriteConfig(a.Config)
		cobra.CheckErr(cfgErr)
	}

	ctx := context.Background()
	_, err = prov.EnsureBasicGrants(ctx, "")
	if err != nil {
		return fmt.Errorf("error writing grants on chain: '%s'", err.Error())
	}

	//Get latest height from the chain, mark feegrant configuration as verified up to that height.
	//This means we've verified feegranting is enabled on-chain and TXs can be sent with a feegranter.
	if prov.PCfg.FeeGrants != nil {
		fmt.Printf("Querying latest chain height to mark FeeGrant height... \n")
		h, err := prov.QueryLatestHeight(ctx)
		cobra.CheckErr(err)
		prov.PCfg.FeeGrants.BlockHeightVerified = h
		fmt.Printf("Feegrant chain height marked: %d\n", h)
		cfgErr := a.OverwriteConfig(a.Config)
		cobra.CheckErr(cfgErr)
	}

	return nil
}
