package cosmos

import (
	"errors"
	"fmt"
	sdkCtx "github.com/cosmos/cosmos-sdk/client"
	sdkTx "github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	relayer "github.com/cosmos/relayer/relayer/provider"
)

// TxFactory returns an instance of tx.Factory derived from
func (cp *CosmosProvider) TxFactory(height int64) (sdkTx.Factory, sdkCtx.TxConfig) {
	ctx := cp.CLIContext(height)
	return sdkTx.Factory{}.
		WithAccountRetriever(ctx.AccountRetriever).
		WithChainID(cp.Config.ChainID).
		WithTxConfig(ctx.TxConfig).
		WithGasAdjustment(cp.Config.GasAdjustment).
		WithGasPrices(cp.Config.GasPrices).
		WithKeybase(cp.Keybase).
		WithSignMode(signing.SignMode_SIGN_MODE_DIRECT), ctx.TxConfig
}

func prepareFactory(clientCtx sdkCtx.Context, txf sdkTx.Factory) (sdkTx.Factory, error) {
	from := clientCtx.GetFromAddress()

	if err := txf.AccountRetriever().EnsureExists(clientCtx, from); err != nil {
		return txf, err
	}

	initNum, initSeq := txf.AccountNumber(), txf.Sequence()
	if initNum == 0 || initSeq == 0 {
		num, seq, err := txf.AccountRetriever().GetAccountNumberSequence(clientCtx, from)
		if err != nil {
			return txf, err
		}

		if initNum == 0 {
			txf = txf.WithAccountNumber(num)
		}

		if initSeq == 0 {
			txf = txf.WithSequence(seq)
		}
	}

	return txf, nil
}

// protoTxProvider is a type which can provide a proto transaction. It is a
// workaround to get access to the wrapper TxBuilder's method GetProtoTx().
type protoTxProvider interface {
	GetProtoTx() *txtypes.Tx
}

// BuildUnsignedTx builds a transaction to be signed given a set of messages. The
// transaction is initially created via the provided factory's generator. Once
// created, the fee, memo, and messages are set.
func BuildUnsignedTx(txf sdkTx.Factory, txConfig sdkCtx.TxConfig, msgs ...relayer.RelayerMessage) (sdkCtx.TxBuilder, error) {
	if txf.ChainID() == "" {
		return nil, fmt.Errorf("chain ID required but not specified")
	}

	fees := txf.Fees()

	if !txf.GasPrices().IsZero() {
		if !fees.IsZero() {
			return nil, errors.New("cannot provide both fees and gas prices")
		}

		glDec := sdk.NewDec(int64(txf.Gas()))

		// Derive the fees based on the provided gas prices, where
		// fee = ceil(gasPrice * gasLimit).
		fees = make(sdk.Coins, len(txf.GasPrices()))

		for i, gp := range txf.GasPrices() {
			fee := gp.Amount.Mul(glDec)
			fees[i] = sdk.NewCoin(gp.Denom, fee.Ceil().RoundInt())
		}
	}

	tx := txConfig.NewTxBuilder()

	if err := tx.SetMsgs(CosmosMsgs(msgs...)...); err != nil {
		return nil, err
	}

	tx.SetMemo(txf.Memo())
	tx.SetFeeAmount(fees)
	tx.SetGasLimit(txf.Gas())
	tx.SetTimeoutHeight(txf.TimeoutHeight())

	return tx, nil
}

// BuildSimTx creates an unsigned tx with an empty single signature and returns
// the encoded transaction or an error if the unsigned transaction cannot be built.
func BuildSimTx(txf sdkTx.Factory, txConfig sdkCtx.TxConfig, msgs ...relayer.RelayerMessage) ([]byte, error) {
	txb, err := BuildUnsignedTx(txf, txConfig, msgs...)
	if err != nil {
		return nil, err
	}

	// Create an empty signature literal as the ante handler will populate with a
	// sentinel pubkey.
	sig := signing.SignatureV2{
		PubKey: &secp256k1.PubKey{},
		Data: &signing.SingleSignatureData{
			SignMode: txf.SignMode(),
		},
		Sequence: txf.Sequence(),
	}
	if err := txb.SetSignatures(sig); err != nil {
		return nil, err
	}

	protoProvider, ok := txb.(protoTxProvider)
	if !ok {
		return nil, fmt.Errorf("cannot simulate amino tx")
	}
	simReq := txtypes.SimulateRequest{Tx: protoProvider.GetProtoTx()}

	return simReq.Marshal()
}

// CalculateGas simulates the execution of a transaction and returns the
// simulation response obtained by the query and the adjusted gas amount.
func CalculateGas(queryFunc func(string, []byte) ([]byte, int64, error), txf sdkTx.Factory, txConfig sdkCtx.TxConfig, msgs ...relayer.RelayerMessage) (txtypes.SimulateResponse, uint64, error) {
	txBytes, err := BuildSimTx(txf, txConfig, msgs...)
	if err != nil {
		return txtypes.SimulateResponse{}, 0, err
	}

	bz, _, err := queryFunc("/cosmos.tx.v1beta1.Service/Simulate", txBytes)
	if err != nil {
		return txtypes.SimulateResponse{}, 0, err
	}

	var simRes txtypes.SimulateResponse

	if err := simRes.Unmarshal(bz); err != nil {
		return txtypes.SimulateResponse{}, 0, err
	}

	return simRes, uint64(txf.GasAdjustment() * float64(simRes.GasInfo.GasUsed)), nil
}

//func (cp *CosmosProvider) NewUpgradeProp(clientid string) error {
//	var plan *upgradetypes.Plan
//	height, err := cp.QueryLatestHeight()
//	if err != nil {
//		return err
//	}
//
//	clientState, err := cp.QueryClientState(height, clientid)
//	if err != nil {
//		return err
//	}
//
//	upgradedClientState := clientState.ZeroCustomFields().(*ibctmtypes.ClientState)
//	upgradedClientState.LatestHeight.RevisionHeight = uint64(plan.Height + 1)
//	upgradedClientState.UnbondingPeriod = unbondingPeriod
//
//	// TODO: make cli args for title and description
//	upgradeProposal, err := clienttypes.NewUpgradeProposal("upgrade",
//		"upgrade the chain's software and unbonding period", *plan, upgradedClientState)
//	if err != nil {
//		return err
//	}
//
//	addr, err := c.ChainProvider.ShowAddress(c.ChainProvider.Key())
//	if err != nil {
//		return err
//	}
//
//	msg, err := govtypes.NewMsgSubmitProposal(upgradeProposal, sdk.NewCoins(deposit), addr)
//	if err != nil {
//		return err
//	}
//}
