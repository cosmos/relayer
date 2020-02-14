package relayer

import (
	"time"

	"github.com/cosmos/cosmos-sdk/client/keys"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"
	clientExported "github.com/cosmos/cosmos-sdk/x/ibc/02-client/exported"
	clientTypes "github.com/cosmos/cosmos-sdk/x/ibc/02-client/types"
	tmClient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint"
	commitment "github.com/cosmos/cosmos-sdk/x/ibc/23-commitment"
)

// CreateConnection creates a connection between two chains given src and dst client IDs
func (src *Chain) CreateConnection(dst *Chain, srcClientID, dstClientID, srcConnectionID, dstConnectionID string, timeout time.Duration) error {
	srcAddr, dstAddr, srcHeight, dstHeight, err := addrsHeights(src, dst)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(timeout)
	for ; true; <-ticker.C {
		msgs, err := src.CreateConnectionStep(dst, srcHeight, dstHeight, srcAddr, dstAddr, srcClientID, dstClientID, srcConnectionID, dstConnectionID)
		if err != nil {
			return err
		}

		if len(msgs.Dst) == 0 && len(msgs.Src) == 0 {
			break
		}

		// Submit the transactions to src chain
		srcRes, err := src.SendMsgs(msgs.Src)
		if err != nil {
			return err
		}
		src.logger.Info(srcRes.String())

		// Submit the transactions to dst chain
		dstRes, err := dst.SendMsgs(msgs.Dst)
		if err != nil {
			return err
		}
		src.logger.Info(dstRes.String())
	}

	return nil
}

// CreateConnectionStep returns the next set of messags for relaying between a src and dst chain
func (src *Chain) CreateConnectionStep(dst *Chain,
	srcHeight, dstHeight int64,
	srcAddr, dstAddr sdk.Address,
	srcClientID, dstClientID,
	srcConnectionID, dstConnectionID string) (*RelayMsgs, error) {
	return &RelayMsgs{}, nil
}

// CreateChannel creates a connection between two chains given src and dst client IDs
func (src *Chain) CreateChannel(dst *Chain,
	srcConnectionID, dstConnectionID,
	srcChannelID, dstChannelID,
	srcPortID, dstPortID string,
	timeout time.Duration) error {
	srcAddr, dstAddr, srcHeight, dstHeight, err := addrsHeights(src, dst)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(timeout)
	for ; true; <-ticker.C {
		msgs, err := src.CreateChannelStep(dst, srcHeight, dstHeight, srcAddr, dstAddr, srcConnectionID, dstConnectionID, srcChannelID, dstChannelID, srcPortID, dstPortID)
		if err != nil {
			return err
		}

		if len(msgs.Dst) == 0 && len(msgs.Src) == 0 {
			break
		}

		// Submit the transactions to src chain
		srcRes, err := src.SendMsgs(msgs.Src)
		if err != nil {
			return err
		}
		src.logger.Info(srcRes.String())

		// Submit the transactions to dst chain
		dstRes, err := dst.SendMsgs(msgs.Dst)
		if err != nil {
			return err
		}
		src.logger.Info(dstRes.String())
	}

	return nil
}

// CreateChannelStep returns the next set of messages for relaying between a src and dst chain
func (src *Chain) CreateChannelStep(dst *Chain,
	srcHeight, dstHeight int64,
	srcAddr, dstAddr sdk.Address,
	srcConnectionID, dstConnectionID,
	srcChannelID, dstChannelID,
	srcPortID, dstPortID string) (*RelayMsgs, error) {
	return &RelayMsgs{}, nil
}

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

// SendMsg wraps the msg in a stdtx, signs and sends it
func (c *Chain) SendMsg(datagram sdk.Msg) (*sdk.TxResponse, error) {
	return c.SendMsgs([]sdk.Msg{datagram})
}

// SendMsgs wraps the msgs in a stdtx, signs and sends it
func (c *Chain) SendMsgs(datagram []sdk.Msg) (*sdk.TxResponse, error) {
	// Fetch key address
	info, err := c.Keybase.Get(c.Key)
	if err != nil {
		return nil, err
	}

	// Fetch account and sequence numbers for the account
	acc, err := auth.NewAccountRetriever(c).GetAccount(info.GetAddress())
	if err != nil {
		return nil, err
	}

	txbytes, err := c.NewTxBldr(
		acc.GetAccountNumber(), acc.GetSequence(),
		c.Gas, c.GasAdjustment, sdk.NewCoins(), c.GasPrices).
		BuildAndSign(c.Key, keys.DefaultKeyPass, datagram)
	if err != nil {
		return nil, err
	}

	res, err := c.NewCliContext().BroadcastTxCommit(txbytes)

	return &res, err
}
