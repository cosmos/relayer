package cmd

import (
	"errors"
	"fmt"

	sdkflags "github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/spf13/cobra"
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
	var delete bool
	var updateGrantees bool
	var grantees []string

	cmd := &cobra.Command{
		Use:   "basicallowance [chain-name] [granter] --num-grantees [int] --overwrite-granter --overwrite-grantees",
		Short: "feegrants for the given chain and granter (if granter is unspecified, use the default key)",
		Long:  "feegrants for the given chain. 10 grantees by default, all with an unrestricted BasicAllowance.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain := args[0]
			cosmosChain, ok := a.config.Chains[chain]
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
				granterKeyOrAddr = prov.PCfg.FeeGrants.GranterKeyOrAddr
			} else {
				granterKeyOrAddr = prov.PCfg.Key
			}

			externalGranter := false

			granterKey, err := prov.KeyFromKeyOrAddress(granterKeyOrAddr)
			if err != nil {
				externalGranter = true
			}

			if externalGranter {
				_, err := prov.DecodeBech32AccAddr(granterKeyOrAddr)
				if err != nil {
					return fmt.Errorf("an unknown granter was specified: '%s' is not a valid bech32 address", granterKeyOrAddr)
				}
			}

			if delete {
				a.log.Info("Deleting feegrant configuration", zap.String("chain", chain))

				cfgErr := a.performConfigLockingOperation(cmd.Context(), func() error {
					chain := a.config.Chains[chain]
					oldProv := chain.ChainProvider.(*cosmos.CosmosProvider)
					oldProv.PCfg.FeeGrants = nil
					return nil
				})
				cobra.CheckErr(cfgErr)
				return nil
			}

			if prov.PCfg.FeeGrants != nil && granterKey != prov.PCfg.FeeGrants.GranterKeyOrAddr && !update {
				return fmt.Errorf("you specified granter '%s' which is different than configured feegranter '%s', but you did not specify the --overwrite-granter flag", granterKeyOrAddr, prov.PCfg.FeeGrants.GranterKeyOrAddr)
			} else if prov.PCfg.FeeGrants != nil && granterKey != prov.PCfg.FeeGrants.GranterKeyOrAddr && update {
				cfgErr := a.performConfigLockingOperation(cmd.Context(), func() error {
					prov.PCfg.FeeGrants.GranterKeyOrAddr = granterKey
					prov.PCfg.FeeGrants.IsExternalGranter = externalGranter
					return nil
				})
				cobra.CheckErr(cfgErr)
			}

			if prov.PCfg.FeeGrants == nil || updateGrantees || len(grantees) > 0 {
				var feegrantErr error

				// No list of grantees was provided, so we will use the default naming convention "grantee1, ... granteeN"
				if grantees == nil {
					if externalGranter {
						return fmt.Errorf("external granter %s was specified, pre-authorized grantees must also be specified", granterKeyOrAddr)
					}
					feegrantErr = prov.ConfigureFeegrants(numGrantees, granterKey)
				} else if !externalGranter {
					feegrantErr = prov.ConfigureWithGrantees(grantees, granterKey)
				} else {
					feegrantErr = prov.ConfigureWithExternalGranter(grantees, granterKeyOrAddr)
				}

				if feegrantErr != nil {
					return feegrantErr
				}

				cfgErr := a.performConfigLockingOperation(cmd.Context(), func() error {
					chain := a.config.Chains[chain]
					oldProv := chain.ChainProvider.(*cosmos.CosmosProvider)
					prov.PCfg.FeeGrants.IsExternalGranter = externalGranter
					oldProv.PCfg.FeeGrants = prov.PCfg.FeeGrants
					return nil
				})
				cobra.CheckErr(cfgErr)
			}

			memo, err := cmd.Flags().GetString(flagMemo)
			if err != nil {
				return err
			}

			gas := uint64(0)
			gasStr, _ := cmd.Flags().GetString(sdkflags.FlagGas)
			if gasStr != "" {
				gasSetting, _ := sdkflags.ParseGasSetting(gasStr)
				gas = gasSetting.Gas
			}

			ctx := cmd.Context()
			_, err = prov.EnsureBasicGrants(ctx, memo, gas)
			if err != nil {
				return fmt.Errorf("error writing grants on chain: '%s'", err.Error())
			}

			// Get latest height from the chain, mark feegrant configuration as verified up to that height.
			// This means we've verified feegranting is enabled on-chain and TXs can be sent with a feegranter.
			if prov.PCfg.FeeGrants != nil {
				h, err := prov.QueryLatestHeight(ctx)
				cobra.CheckErr(err)

				cfgErr := a.performConfigLockingOperation(cmd.Context(), func() error {
					chain := a.config.Chains[chain]
					oldProv := chain.ChainProvider.(*cosmos.CosmosProvider)
					prov.PCfg.FeeGrants.IsExternalGranter = externalGranter
					oldProv.PCfg.FeeGrants = prov.PCfg.FeeGrants
					oldProv.PCfg.FeeGrants.BlockHeightVerified = h
					a.log.Info("feegrant configured", zap.Int64("height", h))
					return nil
				})
				cobra.CheckErr(cfgErr)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&delete, "delete", false, "delete the feegrant configuration")
	cmd.Flags().BoolVar(&update, "overwrite-granter", false, "allow overwriting the existing granter")
	cmd.Flags().BoolVar(&updateGrantees, "overwrite-grantees", false, "allow overwriting existing grantees")
	cmd.Flags().IntVar(&numGrantees, "num-grantees", 10, "number of grantees that will be feegranted with basic allowances")
	cmd.Flags().StringSliceVar(&grantees, "grantees", nil, "comma separated list of grantee key names (keys are created if they do not exist)")
	cmd.MarkFlagsMutuallyExclusive("num-grantees", "grantees", "delete")
	cmd.Flags().String(sdkflags.FlagGas, "", fmt.Sprintf("gas limit to set per-transaction; set to %q to calculate sufficient gas automatically (default %d)", sdkflags.GasFlagAuto, sdkflags.DefaultGasLimit))

	memoFlag(a.viper, cmd)
	return cmd
}

func feegrantBasicGrantsCmd(a *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "basic chain-name [granter]",
		Short: "query the grants for an account (if none is specified, the default account is returned)",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain := args[0]
			cosmosChain, ok := a.config.Chains[chain]
			if !ok {
				return errChainNotFound(args[0])
			}

			prov, ok := cosmosChain.ChainProvider.(*cosmos.CosmosProvider)
			if !ok {
				return errors.New("only CosmosProvider can be feegranted")
			}

			keyNameOrAddress := ""
			if len(args) == 0 {
				keyNameOrAddress = prov.PCfg.Key
			} else {
				keyNameOrAddress = args[0]
			}

			granterAcc, err := prov.AccountFromKeyOrAddress(keyNameOrAddress)
			if err != nil {
				a.log.Error("Unknown account", zap.String("key_or_address", keyNameOrAddress), zap.Error(err))
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
				a.log.Info("feegrant", zap.String("granter", grant.Granter), zap.String("grantee", grant.Grantee), zap.String("allowance", allowance))
			}

			return nil
		},
	}
	return paginationFlags(a.viper, cmd, "feegrant")
}
