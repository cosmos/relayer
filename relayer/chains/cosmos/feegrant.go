package cosmos

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/x/feegrant"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	"go.uber.org/zap"
)

// Searches for valid, existing BasicAllowance grants for the ChainClient's configured Feegranter.
// Expired grants are ignored. Other grant types are ignored.
func (cc *CosmosProvider) GetValidBasicGrants() ([]*feegrant.Grant, error) {
	validGrants := []*feegrant.Grant{}

	if cc.PCfg.FeeGrants == nil {
		return nil, errors.New("no feegrant configuration for chainclient")
	}

	keyNameOrAddress := cc.PCfg.FeeGrants.GranterKeyOrAddr
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
			var feegrantAllowance feegrant.FeeAllowanceI
			e := cc.Cdc.InterfaceRegistry.UnpackAny(grant.Allowance, &feegrantAllowance)
			if e != nil {
				return nil, e
			}
			if isValidGrant(feegrantAllowance.(*feegrant.BasicAllowance)) {
				validGrants = append(validGrants, grant)
			}
		default:
			cc.log.Debug("Ignoring grant",
				zap.String("type", grant.Allowance.TypeUrl),
				zap.String("granter", grant.Granter),
				zap.String("grantee", grant.Grantee))
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

	granterAddr, err := cc.AccountFromKeyOrAddress(cc.PCfg.FeeGrants.GranterKeyOrAddr)
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
				cc.log.Debug("Ignoring grant",
					zap.String("type", grant.Allowance.TypeUrl),
					zap.String("granter", grant.Granter),
					zap.String("grantee", grant.Grantee))
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
			if coin.Amount.LTE(sdkmath.ZeroInt()) {
				valid = false
			}
		}
	}

	return valid
}

func (cc *CosmosProvider) ConfigureFeegrants(numGrantees int, granterKey string) error {
	cc.PCfg.FeeGrants = &FeeGrantConfiguration{
		GranteesWanted:   numGrantees,
		GranterKeyOrAddr: granterKey,
		ManagedGrantees:  []string{},
	}

	return cc.PCfg.FeeGrants.AddGranteeKeys(cc)
}

func (cc *CosmosProvider) ConfigureWithGrantees(grantees []string, granterKey string) error {
	if len(grantees) == 0 {
		return errors.New("list of grantee names cannot be empty")
	}

	cc.PCfg.FeeGrants = &FeeGrantConfiguration{
		GranteesWanted:   len(grantees),
		GranterKeyOrAddr: granterKey,
		ManagedGrantees:  grantees,
	}

	for _, newGrantee := range grantees {
		if !cc.KeyExists(newGrantee) {
			// Add another key to the chain client for the grantee
			_, err := cc.AddKey(newGrantee, sdk.CoinType, string(hd.Secp256k1Type))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (cc *CosmosProvider) ConfigureWithExternalGranter(grantees []string, granterAddr string) error {
	if len(grantees) == 0 {
		return errors.New("list of grantee names cannot be empty")
	}

	cc.PCfg.FeeGrants = &FeeGrantConfiguration{
		GranteesWanted:    len(grantees),
		GranterKeyOrAddr:  granterAddr,
		ManagedGrantees:   grantees,
		IsExternalGranter: true,
	}

	for _, grantee := range grantees {
		k, err := cc.KeyFromKeyOrAddress(grantee)
		if k == "" {
			return fmt.Errorf("invalid empty grantee name")
		} else if err != nil {
			return err
		}
	}

	return nil
}

func (fg *FeeGrantConfiguration) AddGranteeKeys(cc *CosmosProvider) error {
	for i := len(fg.ManagedGrantees); i < fg.GranteesWanted; i++ {
		newGranteeIdx := strconv.Itoa(len(fg.ManagedGrantees) + 1)
		newGrantee := "grantee" + newGranteeIdx

		// Add another key to the chain client for the grantee
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
func (cc *CosmosProvider) GetTxFeeGrant() (txSignerKey string, feeGranterKeyOrAddr string) {
	// By default, we should sign TXs with the ChainClient's default key
	txSignerKey = cc.PCfg.Key

	if cc.PCfg.FeeGrants == nil {
		return
	}

	// Use the ChainClient's configured Feegranter key for the next TX.
	feeGranterKeyOrAddr = cc.PCfg.FeeGrants.GranterKeyOrAddr

	// The ChainClient Feegrant configuration has never been verified on chain.
	// Don't use Feegrants as it could cause the TX to fail on chain.
	if feeGranterKeyOrAddr == "" || cc.PCfg.FeeGrants.BlockHeightVerified <= 0 {
		feeGranterKeyOrAddr = ""
		return
	}

	// Pick the next managed grantee in the list as the TX signer
	lastGranteeIdx := cc.PCfg.FeeGrants.GranteeLastSignerIndex

	if lastGranteeIdx >= 0 && lastGranteeIdx <= len(cc.PCfg.FeeGrants.ManagedGrantees)-1 {
		txSignerKey = cc.PCfg.FeeGrants.ManagedGrantees[lastGranteeIdx]
		cc.PCfg.FeeGrants.GranteeLastSignerIndex = cc.PCfg.FeeGrants.GranteeLastSignerIndex + 1

		// Restart the round robin at 0 if we reached the end of the list of grantees
		if cc.PCfg.FeeGrants.GranteeLastSignerIndex == len(cc.PCfg.FeeGrants.ManagedGrantees) {
			cc.PCfg.FeeGrants.GranteeLastSignerIndex = 0
		}
	}

	return
}

// Ensure all Basic Allowance grants are in place for the given ChainClient.
// This will query (RPC) for existing grants and create new grants if they don't exist.
func (cc *CosmosProvider) EnsureBasicGrants(ctx context.Context, memo string, gas uint64) (*sdk.TxResponse, error) {
	if cc.PCfg.FeeGrants == nil {
		return nil, errors.New("chain client must be a FeeGranter to establish grants")
	} else if len(cc.PCfg.FeeGrants.ManagedGrantees) == 0 {
		return nil, errors.New("chain client is a FeeGranter, but is not managing any Grantees")
	}

	var granterAddr string
	var granterAcc types.AccAddress
	var err error

	granterKey := cc.PCfg.FeeGrants.GranterKeyOrAddr
	if granterKey == "" {
		granterKey = cc.PCfg.Key
	}

	if cc.PCfg.FeeGrants.IsExternalGranter {
		_, err := cc.DecodeBech32AccAddr(granterKey)
		if err != nil {
			return nil, fmt.Errorf("an unknown granter was specified: '%s' is not a valid bech32 address", granterKey)
		}

		granterAddr = granterKey
	} else {
		granterAcc, err = cc.GetKeyAddressForKey(granterKey)
		if err != nil {
			cc.log.Error("Unknown key", zap.String("name", granterKey))
			return nil, err
		}

		granterAddr, err = cc.EncodeBech32AccAddr(granterAcc)
		if err != nil {
			return nil, err
		}
	}

	validGrants, err := cc.GetValidBasicGrants()
	failedLookupGrantsByGranter := err != nil

	msgs := []sdk.Msg{}
	numGrantees := len(cc.PCfg.FeeGrants.ManagedGrantees)
	grantsNeeded := 0

	for _, grantee := range cc.PCfg.FeeGrants.ManagedGrantees {
		// Searching for all grants with the given granter failed, so we will search by the grantee.
		// Reason this lookup sometimes fails is because the 'Search by granter' request is in SDK v0.46+
		if failedLookupGrantsByGranter {
			validGrants, err = cc.GetGranteeValidBasicGrants(grantee)
			if err != nil {
				return nil, err
			}
		}

		granteeAcc, err := cc.GetKeyAddressForKey(grantee)
		if err != nil {
			cc.log.Error("Unknown grantee", zap.String("key_name", grantee))
			return nil, err
		}

		granteeAddr, granteeAddrErr := cc.EncodeBech32AccAddr(granteeAcc)
		if granteeAddrErr != nil {
			return nil, granteeAddrErr
		}

		hasGrant := false
		for _, basicGrant := range validGrants {
			if basicGrant.Grantee == granteeAddr {
				hasGrant = true
			}
		}

		if !hasGrant && !cc.PCfg.FeeGrants.IsExternalGranter {
			grantsNeeded++
			cc.log.Info("Creating feegrant", zap.String("granter", granterAddr), zap.String("grantee", granteeAddr))

			grantMsg, err := cc.getMsgGrantBasicAllowance(granterAcc, granteeAcc)
			if err != nil {
				return nil, err
			}
			msgs = append(msgs, grantMsg)
		} else if !hasGrant {
			cc.log.Warn("Missing feegrant", zap.String("external_granter", granterAddr), zap.String("grantee", granteeAddr))
		}
	}

	if len(msgs) > 0 {
		cliCtx := client.Context{}.WithClient(cc.RPCClient).
			WithInterfaceRegistry(cc.Cdc.InterfaceRegistry).
			WithChainID(cc.PCfg.ChainID).
			WithCodec(cc.Cdc.Marshaler).
			WithFromAddress(granterAcc)

		granterExists := cc.EnsureExists(cliCtx, granterAcc) == nil

		// Feegranter exists on chain
		if granterExists {
			txResp, err := cc.SubmitTxAwaitResponse(ctx, msgs, memo, gas, granterKey)
			if err != nil {
				return nil, err
			} else if txResp != nil && txResp.TxResponse != nil && txResp.TxResponse.Code != 0 {
				cc.log.Warn("Feegrant TX failed", zap.String("tx_hash", txResp.TxResponse.TxHash), zap.Uint32("code", txResp.TxResponse.Code))
				return nil, fmt.Errorf("could not configure feegrant for granter %s", granterKey)
			}

			cc.log.Info("Feegrant succeeded", zap.Int("new_grants", grantsNeeded), zap.Int("existing_grants", numGrantees-grantsNeeded), zap.String("tx_hash", txResp.TxResponse.TxHash))
			return txResp.TxResponse, err
		}

		return nil, fmt.Errorf("granter %s does not exist on chain", granterKey)
	}

	return nil, nil
}

// GrantBasicAllowance Send a feegrant with the basic allowance type.
// This function does not check for existing feegrant authorizations.
// TODO: check for existing authorizations prior to attempting new one.
func (cc *CosmosProvider) GrantAllGranteesBasicAllowance(ctx context.Context, gas uint64) error {
	if cc.PCfg.FeeGrants == nil {
		return errors.New("chain client must be a FeeGranter to establish grants")
	} else if len(cc.PCfg.FeeGrants.ManagedGrantees) == 0 {
		return errors.New("chain client is a FeeGranter, but is not managing any Grantees")
	}

	granterKey := cc.PCfg.FeeGrants.GranterKeyOrAddr
	if granterKey == "" {
		granterKey = cc.PCfg.Key
	}
	granterAddr, err := cc.GetKeyAddressForKey(granterKey)
	if err != nil {
		cc.log.Error("Unknown granter", zap.String("key_name", granterKey))
		return err
	}

	for _, grantee := range cc.PCfg.FeeGrants.ManagedGrantees {
		granteeAddr, err := cc.GetKeyAddressForKey(grantee)

		if err != nil {
			cc.log.Error("Unknown grantee", zap.String("key_name", grantee))
			return err
		}

		grantResp, err := cc.GrantBasicAllowance(ctx, granterAddr, granterKey, granteeAddr, gas)
		if err != nil {
			return err
		} else if grantResp != nil && grantResp.TxResponse != nil && grantResp.TxResponse.Code != 0 {
			return fmt.Errorf("could not configure feegrant for granter %s and grantee %s", granterAddr.String(), granteeAddr.String())
		}
	}
	return nil
}

// GrantBasicAllowance Send a feegrant with the basic allowance type.
// This function does not check for existing feegrant authorizations.
func (cc *CosmosProvider) GrantAllGranteesBasicAllowanceWithExpiration(ctx context.Context, gas uint64, expiration time.Time) error {
	if cc.PCfg.FeeGrants == nil {
		return errors.New("chain client must be a FeeGranter to establish grants")
	} else if len(cc.PCfg.FeeGrants.ManagedGrantees) == 0 {
		return errors.New("chain client is a FeeGranter, but is not managing any Grantees")
	}

	granterKey := cc.PCfg.FeeGrants.GranterKeyOrAddr
	if granterKey == "" {
		granterKey = cc.PCfg.Key
	}

	granterAddr, err := cc.GetKeyAddressForKey(granterKey)
	if err != nil {
		cc.log.Error("Unknown granter", zap.String("key_name", granterKey))
		return err
	}

	for _, grantee := range cc.PCfg.FeeGrants.ManagedGrantees {
		granteeAddr, err := cc.GetKeyAddressForKey(grantee)

		if err != nil {
			cc.log.Error("Unknown grantee", zap.String("key_name", grantee))
			return err
		}

		grantResp, err := cc.GrantBasicAllowanceWithExpiration(ctx, granterAddr, granterKey, granteeAddr, gas, expiration)
		if err != nil {
			return err
		} else if grantResp != nil && grantResp.TxResponse != nil && grantResp.TxResponse.Code != 0 {
			return fmt.Errorf("could not configure feegrant for granter %s and grantee %s", granterAddr.String(), granteeAddr.String())
		}
	}
	return nil
}

func (cc *CosmosProvider) getMsgGrantBasicAllowanceWithExpiration(granter sdk.AccAddress, grantee sdk.AccAddress, expiration time.Time) (sdk.Msg, error) {
	feeGrantBasic := &feegrant.BasicAllowance{
		Expiration: &expiration,
	}
	msgGrantAllowance, err := feegrant.NewMsgGrantAllowance(feeGrantBasic, granter, grantee)
	if err != nil {
		return nil, err
	}

	// Update the Grant to ensure the correct chain-specific granter is set
	granterAddr, granterAddrErr := cc.EncodeBech32AccAddr(granter)
	if granterAddrErr != nil {
		return nil, granterAddrErr
	}

	// Update the Grant to ensure the correct chain-specific grantee is set
	granteeAddr, granteeAddrErr := cc.EncodeBech32AccAddr(grantee)
	if granteeAddrErr != nil {
		return nil, granteeAddrErr
	}

	// Due to the way Lens configures the SDK, addresses will have the 'cosmos' prefix which
	// doesn't necessarily match the chain prefix of the ChainClient config. So calling the internal
	// 'NewMsgGrantAllowance' function will return the *incorrect* 'cosmos' prefixed bech32 address.
	// override the 'cosmos' prefixed bech32 addresses with the correct chain prefix
	msgGrantAllowance.Grantee = granteeAddr
	msgGrantAllowance.Granter = granterAddr

	return msgGrantAllowance, nil
}

func (cc *CosmosProvider) getMsgGrantBasicAllowance(granter sdk.AccAddress, grantee sdk.AccAddress) (sdk.Msg, error) {
	feeGrantBasic := &feegrant.BasicAllowance{}
	msgGrantAllowance, err := feegrant.NewMsgGrantAllowance(feeGrantBasic, granter, grantee)
	if err != nil {
		return nil, err
	}

	// Update the Grant to ensure the correct chain-specific granter is set
	granterAddr, granterAddrErr := cc.EncodeBech32AccAddr(granter)
	if granterAddrErr != nil {
		return nil, granterAddrErr
	}

	// Update the Grant to ensure the correct chain-specific grantee is set
	granteeAddr, granteeAddrErr := cc.EncodeBech32AccAddr(grantee)
	if granteeAddrErr != nil {
		return nil, granteeAddrErr
	}

	// Due to the way Lens configures the SDK, addresses will have the 'cosmos' prefix which
	// doesn't necessarily match the chain prefix of the ChainClient config. So calling the internal
	// 'NewMsgGrantAllowance' function will return the *incorrect* 'cosmos' prefixed bech32 address.
	// override the 'cosmos' prefixed bech32 addresses with the correct chain prefix
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
		return nil, err
	}

	return txResp, nil
}
