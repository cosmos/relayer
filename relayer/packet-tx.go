package relayer

import (
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/cosmos/relayer/relayer/provider"
)

//nolint:lll
// SendTransferMsg initiates an ics20 transfer from src to dst with the specified args
func (c *Chain) SendTransferMsg(dst *Chain, amount sdk.Coin, dstAddr string, toHeightOffset uint64, toTimeOffset time.Duration, srcChannel *chantypes.IdentifiedChannel) error {
	var (
		timeoutHeight    uint64
		timeoutTimestamp uint64
	)

	// get header representing dst to check timeouts
	dsth, err := dst.ChainProvider.QueryLatestHeight()
	if err != nil {
		return err
	}
	h, err := dst.ChainProvider.GetIBCUpdateHeader(dsth, c.ChainProvider, c.PathEnd.ClientID)
	if err != nil {
		return err
	}

	switch {
	case toHeightOffset > 0 && toTimeOffset > 0:
		timeoutHeight = h.GetHeight().GetRevisionHeight() + toHeightOffset
		timeoutTimestamp = uint64(time.Now().Add(toTimeOffset).UnixNano())
	case toHeightOffset > 0:
		timeoutHeight = h.GetHeight().GetRevisionHeight() + toHeightOffset
		timeoutTimestamp = 0
	case toTimeOffset > 0:
		timeoutHeight = 0
		timeoutTimestamp = uint64(time.Now().Add(toTimeOffset).UnixNano())
	case toHeightOffset == 0 && toTimeOffset == 0:
		timeoutHeight = h.GetHeight().GetRevisionHeight() + 1000
		timeoutTimestamp = 0
	}

	// MsgTransfer will call SendPacket on src chain
	msg, err := c.ChainProvider.MsgTransfer(amount, dst.PathEnd.ChainID, dstAddr, srcChannel.PortId, srcChannel.ChannelId, timeoutHeight, timeoutTimestamp)
	if err != nil {
		return err
	}

	txs := RelayMsgs{
		Src: []provider.RelayerMessage{msg},
		Dst: []provider.RelayerMessage{},
	}

	if txs.Send(c, dst); !txs.Success() {
		return fmt.Errorf("failed to send transfer message")
	}
	return nil
}
