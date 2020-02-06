package relayer

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"
	clientExported "github.com/cosmos/cosmos-sdk/x/ibc/02-client/exported"
	clientTypes "github.com/cosmos/cosmos-sdk/x/ibc/02-client/types"
	tmClient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint"
	commitment "github.com/cosmos/cosmos-sdk/x/ibc/23-commitment"
)

// UpdateClient creates an sdk.Msg to update the client on c with data pulled from cp
func (src *Chain) UpdateClient(dst *Chain, dstHeight int64, srcAddr sdk.AccAddress) (clientTypes.MsgUpdateClient, error) {
	// Fetch counterparty clientIDs
	counterIDs, err := src.GetCounterparty(dst.ChainID)
	if err != nil {
		return clientTypes.MsgUpdateClient{}, err
	}

	// Pull header for the dst chain from the liteDB
	dstHeader, err := dst.GetLiteSignedHeaderAtHeight(dstHeight)
	if err != nil {
		return clientTypes.MsgUpdateClient{}, err
	}

	return clientTypes.NewMsgUpdateClient(counterIDs.ClientID, dstHeader, srcAddr), nil
}

// CreateClient creates an sdk.Msg to update the client on c with data pulled from cp
func (src *Chain) CreateClient(dst *Chain, dstHeight int64, srcAddr sdk.AccAddress) (clientTypes.MsgCreateClient, error) {
	// Fetch counterparty clientIDs
	counterIDs, err := src.GetCounterparty(dst.ChainID)
	if err != nil {
		return clientTypes.MsgCreateClient{}, err
	}

	// Pull header for the dst chain from the liteDB
	dstHeader, err := dst.GetLiteSignedHeaderAtHeight(dstHeight)
	if err != nil {
		return clientTypes.MsgCreateClient{}, err
	}

	consState := tmClient.ConsensusState{
		Root:             commitment.NewRoot(dstHeader.AppHash),
		ValidatorSetHash: dstHeader.ValidatorSet.Hash(),
	}

	return clientTypes.NewMsgCreateClient(counterIDs.ClientID, clientExported.ClientTypeTendermint, consState, srcAddr), nil
}

// SendMsgs sends the standard transactions to the individual chain
func (c *Chain) SendMsgs(datagram []sdk.Msg) error {
	// Fetch key address
	info, err := c.Keybase.Get(c.Key)
	if err != nil {
		return err
	}

	// Fetch account and sequence numbers for the account
	acc, err := auth.NewAccountRetriever(c).GetAccount(info.GetAddress())
	if err != nil {
		return err
	}

	// Calculate fess from the gas and gas prices
	// TODO: Incorporate c.GasAdjustment here?
	fees := make(sdk.Coins, len(c.GasPrices))
	for i, gp := range c.GasPrices {
		fee := gp.Amount.Mul(sdk.NewDec(int64(c.Gas)))
		fees[i] = sdk.NewCoin(gp.Denom, fee.Ceil().RoundInt())
	}

	// Build the StdSignMsg
	sign := auth.StdSignMsg{
		ChainID:       c.ChainID,
		AccountNumber: acc.GetSequence(),
		Sequence:      acc.GetSequence(),
		Memo:          c.Memo,
		Msgs:          datagram,
		Fee:           auth.NewStdFee(c.Gas, fees),
	}

	// Create signature for transaction
	stdSignature, err := auth.MakeSignature(c.Keybase, c.Key, "", sign)

	// Create the StdTx for broadcast
	stdTx := auth.NewStdTx(datagram, sign.Fee, []auth.StdSignature{stdSignature}, c.Memo)

	// Marshal amino
	out, err := c.Cdc.MarshalBinaryLengthPrefixed(stdTx)
	if err != nil {
		return err
	}

	// Broadcast transaction
	res, err := c.Client.BroadcastTxCommit(out)
	if err != nil {
		return err
	}

	// TODO: Figure out what to do with the response
	fmt.Println(res)
	return nil
}
