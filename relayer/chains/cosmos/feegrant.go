package cosmos

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/x/feegrant"
)

// Searches for valid, existing BasicAllowance grants for the ChainClient's configured Feegranter.
// Expired grants are ignored. Other grant types are ignored.
func (cc *CosmosProvider) GetValidBasicGrants() ([]*feegrant.Grant, error) {
	validGrants := []*feegrant.Grant{}

	if cc.PCfg.FeeGrants == nil {
		return nil, errors.New("no feegrant configuration for chainclient")
	}

	keyNameOrAddress := cc.PCfg.FeeGrants.GranterKey
	address, err := cc.AccountFromKeyOrAddress(keyNameOrAddress)
	if err != nil {
		return nil, err
	}

	encodedAddr := cc.MustEncodeAccAddr(address)
	grants, err := cc.QueryFeegrantsByGranter(encodedAddr, nil)
	if err != nil {
		return nil, err
	}

	for _, grant := range grants {
		switch grant.Allowance.TypeUrl {
		case "/cosmos.feegrant.v1beta1.BasicAllowance":
			//var feegrantAllowance feegrant.BasicAllowance
			var feegrantAllowance feegrant.FeeAllowanceI
			e := cc.Cdc.InterfaceRegistry.UnpackAny(grant.Allowance, &feegrantAllowance)
			if e != nil {
				return nil, e
			}
			//feegrantAllowance := grant.Allowance.GetCachedValue().(*feegrant.BasicAllowance)
			if isValidGrant(feegrantAllowance.(*feegrant.BasicAllowance)) {
				validGrants = append(validGrants, grant)
			}
		default:
			fmt.Printf("Ignoring grant type %s for granter %s and grantee %s\n", grant.Allowance.TypeUrl, grant.Granter, grant.Grantee)
		}
	}

	return validGrants, nil
}

// Searches for valid, existing BasicAllowance grants for the given grantee & ChainClient's configured granter.
// Expired grants are ignored. Other grant types are ignored.
func (cc *CosmosProvider) GetGranteeValidBasicGrants(granteeKey string) ([]*feegrant.Grant, error) {
	validGrants := []*feegrant.Grant{}

	if cc.PCfg.FeeGrants == nil {
		return nil, errors.New("no feegrant configuration for chainclient")
	}

	granterAddr, err := cc.AccountFromKeyOrAddress(cc.PCfg.FeeGrants.GranterKey)
	if err != nil {
		return nil, err
	}
	granterEncodedAddr := cc.MustEncodeAccAddr(granterAddr)

	address, err := cc.AccountFromKeyOrAddress(granteeKey)
	if err != nil {
		return nil, err
	}

	encodedAddr := cc.MustEncodeAccAddr(address)
	grants, err := cc.QueryFeegrantsByGrantee(encodedAddr, nil)
	if err != nil {
		return nil, err
	}

	for _, grant := range grants {
		if grant.Granter == granterEncodedAddr {
			switch grant.Allowance.TypeUrl {
			case "/cosmos.feegrant.v1beta1.BasicAllowance":
				var feegrantAllowance feegrant.FeeAllowanceI
				e := cc.Cdc.InterfaceRegistry.UnpackAny(grant.Allowance, &feegrantAllowance)
				if e != nil {
					return nil, e
				}
				if isValidGrant(feegrantAllowance.(*feegrant.BasicAllowance)) {
					validGrants = append(validGrants, grant)
				}
			default:
				fmt.Printf("Ignoring grant type %s for granter %s and grantee %s\n", grant.Allowance.TypeUrl, grant.Granter, grant.Grantee)
			}
		}
	}

	return validGrants, nil
}

// True if the grant has not expired and all coins have positive balances, false otherwise
// Note: technically, any single coin with a positive balance makes the grant usable
func isValidGrant(a *feegrant.BasicAllowance) bool {
	//grant expired due to time limit
	if a.Expiration != nil && time.Now().After(*a.Expiration) {
		return false
	}

	//feegrant without a spending limit specified allows unlimited fees to be spent
	valid := true

	//spending limit is specified, check if there are funds remaining on every coin
	if a.SpendLimit != nil {
		for _, coin := range a.SpendLimit {
			if coin.Amount.LTE(types.ZeroInt()) {
				valid = false
			}
		}
	}

	return valid
}

func (cc *CosmosProvider) ConfigureFeegrants(numGrantees int, granterKey string) error {
	cc.PCfg.FeeGrants = &FeeGrantConfiguration{
		GranteesWanted:  numGrantees,
		GranterKey:      granterKey,
		ManagedGrantees: []string{},
	}

	return cc.PCfg.FeeGrants.AddGranteeKeys(cc)
}

func (cc *CosmosProvider) ConfigureWithGrantees(grantees []string, granterKey string) error {
	if len(grantees) == 0 {
		return errors.New("list of grantee names cannot be empty")
	}

	cc.PCfg.FeeGrants = &FeeGrantConfiguration{
		GranteesWanted:  len(grantees),
		GranterKey:      granterKey,
		ManagedGrantees: grantees,
	}

	for _, newGrantee := range grantees {
		if !cc.KeyExists(newGrantee) {
			//Add another key to the chain client for the grantee
			_, err := cc.AddKey(newGrantee, sdk.CoinType, string(hd.Secp256k1Type))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (fg *FeeGrantConfiguration) AddGranteeKeys(cc *CosmosProvider) error {
	for i := len(fg.ManagedGrantees); i < fg.GranteesWanted; i++ {
		newGranteeIdx := strconv.Itoa(len(fg.ManagedGrantees) + 1)
		newGrantee := "grantee" + newGranteeIdx

		//Add another key to the chain client for the grantee
		_, err := cc.AddKey(newGrantee, sdk.CoinType, string(hd.Secp256k1Type))
		if err != nil {
			return err
		}

		fg.ManagedGrantees = append(fg.ManagedGrantees, newGrantee)
	}

	return nil
}

// Get the feegrant params to use for the next TX. If feegrants are not configured for the chain client, the default key will be used for TX signing.
// Otherwise, a configured feegrantee will be chosen for TX signing in round-robin fashion.
func (cc *CosmosProvider) GetTxFeeGrant() (txSignerKey string, feeGranterKey string) {
	//By default, we should sign TXs with the ChainClient's default key
	txSignerKey = cc.PCfg.Key

	if cc.PCfg.FeeGrants == nil {
		fmt.Printf("cc.Config.FeeGrants == nil\n")
		return
	}

	// Use the ChainClient's configured Feegranter key for the next TX.
	feeGranterKey = cc.PCfg.FeeGrants.GranterKey

	// The ChainClient Feegrant configuration has never been verified on chain.
	// Don't use Feegrants as it could cause the TX to fail on chain.
	if feeGranterKey == "" || cc.PCfg.FeeGrants.BlockHeightVerified <= 0 {
		fmt.Printf("cc.Config.FeeGrants.BlockHeightVerified <= 0\n")
		feeGranterKey = ""
		return
	}

	//Pick the next managed grantee in the list as the TX signer
	lastGranteeIdx := cc.PCfg.FeeGrants.GranteeLastSignerIndex

	if lastGranteeIdx >= 0 && lastGranteeIdx <= len(cc.PCfg.FeeGrants.ManagedGrantees)-1 {
		txSignerKey = cc.PCfg.FeeGrants.ManagedGrantees[lastGranteeIdx]
		cc.PCfg.FeeGrants.GranteeLastSignerIndex = cc.PCfg.FeeGrants.GranteeLastSignerIndex + 1

		//Restart the round robin at 0 if we reached the end of the list of grantees
		if cc.PCfg.FeeGrants.GranteeLastSignerIndex == len(cc.PCfg.FeeGrants.ManagedGrantees) {
			cc.PCfg.FeeGrants.GranteeLastSignerIndex = 0
		}
	}

	return
}

// Ensure all Basic Allowance grants are in place for the given ChainClient.
// This will query (RPC) for existing grants and create new grants if they don't exist.
func (cc *CosmosProvider) EnsureBasicGrants(ctx context.Context, memo string) (*sdk.TxResponse, error) {
	if cc.PCfg.FeeGrants == nil {
		return nil, errors.New("ChainClient must be a FeeGranter to establish grants")
	} else if len(cc.PCfg.FeeGrants.ManagedGrantees) == 0 {
		return nil, errors.New("ChainClient is a FeeGranter, but is not managing any Grantees")
	}

	granterKey := cc.PCfg.FeeGrants.GranterKey
	if granterKey == "" {
		granterKey = cc.PCfg.Key
	}

	granterAcc, err := cc.GetKeyAddressForKey(granterKey)
	if err != nil {
		fmt.Printf("Retrieving key '%s': ChainClient FeeGranter misconfiguration: %s", granterKey, err.Error())
		return nil, err
	}

	granterAddr, granterAddrErr := cc.EncodeBech32AccAddr(granterAcc)
	if granterAddrErr != nil {
		return nil, granterAddrErr
	}

	validGrants, err := cc.GetValidBasicGrants()
	failedLookupGrantsByGranter := err != nil

	msgs := []sdk.Msg{}
	numGrantees := len(cc.PCfg.FeeGrants.ManagedGrantees)
	grantsNeeded := 0

	for _, grantee := range cc.PCfg.FeeGrants.ManagedGrantees {

		//Searching for all grants with the given granter failed, so we will search by the grantee.
		//Reason this lookup sometimes fails is because the 'Search by granter' request is in SDK v0.46+
		if failedLookupGrantsByGranter {
			validGrants, err = cc.GetGranteeValidBasicGrants(grantee)
			if err != nil {
				return nil, err
			}
		}

		granteeAcc, err := cc.GetKeyAddressForKey(grantee)
		if err != nil {
			fmt.Printf("Misconfiguration for grantee key %s. Error: %s\n", grantee, err.Error())
			return nil, err
		}

		granteeAddr, granteeAddrErr := cc.EncodeBech32AccAddr(granteeAcc)
		if granteeAddrErr != nil {
			return nil, granteeAddrErr
		}

		hasGrant := false
		for _, basicGrant := range validGrants {
			if basicGrant.Grantee == granteeAddr {
				fmt.Printf("Valid grant found for granter %s, grantee %s\n", basicGrant.Granter, basicGrant.Grantee)
				hasGrant = true
			}
		}

		if !hasGrant {
			grantsNeeded += 1
			fmt.Printf("Grant will be created on chain for granter %s and grantee %s\n", granterAddr, granteeAddr)
			grantMsg, err := cc.getMsgGrantBasicAllowance(granterAcc, granteeAcc)
			if err != nil {
				return nil, err
			}
			msgs = append(msgs, grantMsg)
		}
	}

	if len(msgs) > 0 {
		cliCtx := client.Context{}.WithClient(cc.RPCClient).
			WithInterfaceRegistry(cc.Cdc.InterfaceRegistry).
			WithChainID(cc.PCfg.ChainID).
			WithCodec(cc.Cdc.Marshaler).
			WithFromAddress(granterAcc)

		granterExists := cc.EnsureExists(cliCtx, granterAcc) == nil

		//Feegranter exists on chain
		if granterExists {
			txResp, err := cc.SubmitTxAwaitResponse(ctx, msgs, memo, 0, granterKey)
			if err != nil {
				fmt.Printf("Error: SubmitTxAwaitResponse: %s", err.Error())
				return nil, err
			} else if txResp != nil && txResp.TxResponse != nil && txResp.TxResponse.Code != 0 {
				fmt.Printf("Submitting grants for granter %s failed. Code: %d, TX hash: %s\n", granterKey, txResp.TxResponse.Code, txResp.TxResponse.TxHash)
				return nil, fmt.Errorf("could not configure feegrant for granter %s", granterKey)
			}

			fmt.Printf("TX succeeded, %d new grants configured, %d grants already in place. TX hash: %s\n", grantsNeeded, numGrantees-grantsNeeded, txResp.TxResponse.TxHash)
			return txResp.TxResponse, err
		} else {
			return nil, fmt.Errorf("granter %s does not exist on chain", granterKey)
		}
	} else {
		fmt.Printf("All grantees (%d total) already had valid feegrants. Feegrant configuration verified.\n", numGrantees)
	}

	return nil, nil
}

func getGasTokenDenom(gasPrices string) (string, error) {
	r := regexp.MustCompile(`(?P<digits>[0-9.]*)(?P<denom>.*)`)
	submatches := r.FindStringSubmatch(gasPrices)
	if len(submatches) != 3 {
		return "", errors.New("could not find fee denom")
	}

	return submatches[2], nil
}

// GrantBasicAllowance Send a feegrant with the basic allowance type.
// This function does not check for existing feegrant authorizations.
// TODO: check for existing authorizations prior to attempting new one.
func (cc *CosmosProvider) GrantAllGranteesBasicAllowance(ctx context.Context, gas uint64) error {
	if cc.PCfg.FeeGrants == nil {
		return errors.New("ChainClient must be a FeeGranter to establish grants")
	} else if len(cc.PCfg.FeeGrants.ManagedGrantees) == 0 {
		return errors.New("ChainClient is a FeeGranter, but is not managing any Grantees")
	}

	granterKey := cc.PCfg.FeeGrants.GranterKey
	if granterKey == "" {
		granterKey = cc.PCfg.Key
	}
	granterAddr, err := cc.GetKeyAddressForKey(granterKey)
	if err != nil {
		fmt.Printf("ChainClient FeeGranter misconfiguration: %s", err.Error())
		return err
	}

	for _, grantee := range cc.PCfg.FeeGrants.ManagedGrantees {
		granteeAddr, err := cc.GetKeyAddressForKey(grantee)

		if err != nil {
			fmt.Printf("Misconfiguration for grantee %s. Error: %s\n", grantee, err.Error())
			return err
		}

		grantResp, err := cc.GrantBasicAllowance(ctx, granterAddr, granterKey, granteeAddr, gas)
		if err != nil {
			return err
		} else if grantResp != nil && grantResp.TxResponse != nil && grantResp.TxResponse.Code != 0 {
			fmt.Printf("grantee %s and granter %s. Code: %d\n", granterAddr.String(), granteeAddr.String(), grantResp.TxResponse.Code)
			return fmt.Errorf("could not configure feegrant for granter %s and grantee %s", granterAddr.String(), granteeAddr.String())
		}
	}
	return nil
}

// GrantBasicAllowance Send a feegrant with the basic allowance type.
// This function does not check for existing feegrant authorizations.
// TODO: check for existing authorizations prior to attempting new one.
func (cc *CosmosProvider) GrantAllGranteesBasicAllowanceWithExpiration(ctx context.Context, gas uint64, expiration time.Time) error {
	if cc.PCfg.FeeGrants == nil {
		return errors.New("ChainClient must be a FeeGranter to establish grants")
	} else if len(cc.PCfg.FeeGrants.ManagedGrantees) == 0 {
		return errors.New("ChainClient is a FeeGranter, but is not managing any Grantees")
	}

	granterKey := cc.PCfg.FeeGrants.GranterKey
	if granterKey == "" {
		granterKey = cc.PCfg.Key
	}

	granterAddr, err := cc.GetKeyAddressForKey(granterKey)
	if err != nil {
		fmt.Printf("ChainClient FeeGranter misconfiguration: %s", err.Error())
		return err
	}

	for _, grantee := range cc.PCfg.FeeGrants.ManagedGrantees {
		granteeAddr, err := cc.GetKeyAddressForKey(grantee)

		if err != nil {
			fmt.Printf("Misconfiguration for grantee %s. Error: %s\n", grantee, err.Error())
			return err
		}

		grantResp, err := cc.GrantBasicAllowanceWithExpiration(ctx, granterAddr, granterKey, granteeAddr, gas, expiration)
		if err != nil {
			return err
		} else if grantResp != nil && grantResp.TxResponse != nil && grantResp.TxResponse.Code != 0 {
			fmt.Printf("grantee %s and granter %s. Code: %d\n", granterAddr.String(), granteeAddr.String(), grantResp.TxResponse.Code)
			return fmt.Errorf("could not configure feegrant for granter %s and grantee %s", granterAddr.String(), granteeAddr.String())
		}
	}
	return nil
}

func (cc *CosmosProvider) getMsgGrantBasicAllowanceWithExpiration(granter sdk.AccAddress, grantee sdk.AccAddress, expiration time.Time) (sdk.Msg, error) {
	//thirtyMin := time.Now().Add(30 * time.Minute)
	feeGrantBasic := &feegrant.BasicAllowance{
		Expiration: &expiration,
	}
	msgGrantAllowance, err := feegrant.NewMsgGrantAllowance(feeGrantBasic, granter, grantee)
	if err != nil {
		fmt.Printf("Error: GrantBasicAllowance.NewMsgGrantAllowance: %s", err.Error())
		return nil, err
	}

	//Due to the way Lens configures the SDK, addresses will have the 'cosmos' prefix which
	//doesn't necessarily match the chain prefix of the ChainClient config. So calling the internal
	//'NewMsgGrantAllowance' function will return the *incorrect* 'cosmos' prefixed bech32 address.

	//Update the Grant to ensure the correct chain-specific granter is set
	granterAddr, granterAddrErr := cc.EncodeBech32AccAddr(granter)
	if granterAddrErr != nil {
		fmt.Printf("EncodeBech32AccAddr: %s", granterAddrErr.Error())
		return nil, granterAddrErr
	}

	//Update the Grant to ensure the correct chain-specific grantee is set
	granteeAddr, granteeAddrErr := cc.EncodeBech32AccAddr(grantee)
	if granteeAddrErr != nil {
		fmt.Printf("EncodeBech32AccAddr: %s", granteeAddrErr.Error())
		return nil, granteeAddrErr
	}

	//override the 'cosmos' prefixed bech32 addresses with the correct chain prefix
	msgGrantAllowance.Grantee = granteeAddr
	msgGrantAllowance.Granter = granterAddr

	return msgGrantAllowance, nil
}

func (cc *CosmosProvider) getMsgGrantBasicAllowance(granter sdk.AccAddress, grantee sdk.AccAddress) (sdk.Msg, error) {
	//thirtyMin := time.Now().Add(30 * time.Minute)
	feeGrantBasic := &feegrant.BasicAllowance{
		//Expiration: &thirtyMin,
	}
	msgGrantAllowance, err := feegrant.NewMsgGrantAllowance(feeGrantBasic, granter, grantee)
	if err != nil {
		fmt.Printf("Error: GrantBasicAllowance.NewMsgGrantAllowance: %s", err.Error())
		return nil, err
	}

	//Due to the way Lens configures the SDK, addresses will have the 'cosmos' prefix which
	//doesn't necessarily match the chain prefix of the ChainClient config. So calling the internal
	//'NewMsgGrantAllowance' function will return the *incorrect* 'cosmos' prefixed bech32 address.

	//Update the Grant to ensure the correct chain-specific granter is set
	granterAddr, granterAddrErr := cc.EncodeBech32AccAddr(granter)
	if granterAddrErr != nil {
		fmt.Printf("EncodeBech32AccAddr: %s", granterAddrErr.Error())
		return nil, granterAddrErr
	}

	//Update the Grant to ensure the correct chain-specific grantee is set
	granteeAddr, granteeAddrErr := cc.EncodeBech32AccAddr(grantee)
	if granteeAddrErr != nil {
		fmt.Printf("EncodeBech32AccAddr: %s", granteeAddrErr.Error())
		return nil, granteeAddrErr
	}

	//override the 'cosmos' prefixed bech32 addresses with the correct chain prefix
	msgGrantAllowance.Grantee = granteeAddr
	msgGrantAllowance.Granter = granterAddr

	return msgGrantAllowance, nil
}

func (cc *CosmosProvider) GrantBasicAllowance(ctx context.Context, granter sdk.AccAddress, granterKeyName string, grantee sdk.AccAddress, gas uint64) (*txtypes.GetTxResponse, error) {
	msgGrantAllowance, err := cc.getMsgGrantBasicAllowance(granter, grantee)
	if err != nil {
		return nil, err
	}

	msgs := []sdk.Msg{msgGrantAllowance}
	txResp, err := cc.SubmitTxAwaitResponse(ctx, msgs, "", gas, granterKeyName)
	if err != nil {
		fmt.Printf("Error: GrantBasicAllowance.SubmitTxAwaitResponse: %s", err.Error())
		return nil, err
	}

	return txResp, nil
}

func (cc *CosmosProvider) GrantBasicAllowanceWithExpiration(ctx context.Context, granter sdk.AccAddress, granterKeyName string, grantee sdk.AccAddress, gas uint64, expiration time.Time) (*txtypes.GetTxResponse, error) {
	msgGrantAllowance, err := cc.getMsgGrantBasicAllowanceWithExpiration(granter, grantee, expiration)
	if err != nil {
		return nil, err
	}

	msgs := []sdk.Msg{msgGrantAllowance}
	txResp, err := cc.SubmitTxAwaitResponse(ctx, msgs, "", gas, granterKeyName)
	if err != nil {
		fmt.Printf("Error: GrantBasicAllowance.SubmitTxAwaitResponse: %s", err.Error())
		return nil, err
	}

	return txResp, nil
}
