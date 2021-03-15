package relayer

import (
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

//nolint:lll
// SendTransferMsg initiates an ibs20 transfer from src to dst with the specified args
func (c *Chain) SendTransferMsg(dst *Chain, amount sdk.Coin, dstAddr string, toHeightOffset uint64, toTimeOffset time.Duration) error {
	var (
		timeoutHeight    uint64
		timeoutTimestamp uint64
	)

	// get header representing dst to check timeouts
	h, err := dst.GetIBCUpdateHeader(c)
	if err != nil {
		return err
	}

	switch {
	case toHeightOffset > 0 && toTimeOffset > 0:
		timeoutHeight = uint64(h.Header.Height) + toHeightOffset
		timeoutTimestamp = uint64(time.Now().Add(toTimeOffset).UnixNano())
	case toHeightOffset > 0:
		timeoutHeight = uint64(h.Header.Height) + toHeightOffset
		timeoutTimestamp = 0
	case toTimeOffset > 0:
		timeoutHeight = 0
		timeoutTimestamp = uint64(time.Now().Add(toTimeOffset).UnixNano())
	case toHeightOffset == 0 && toTimeOffset == 0:
		timeoutHeight = uint64(h.Header.Height + 1000)
		timeoutTimestamp = 0
	}

	// MsgTransfer will call SendPacket on src chain
	txs := RelayMsgs{
		Src: []sdk.Msg{c.MsgTransfer(
			dst.PathEnd, amount, dstAddr, timeoutHeight, timeoutTimestamp,
		)},
		Dst: []sdk.Msg{},
	}

	if txs.Send(c, dst); !txs.Success() {
		return fmt.Errorf("failed to send transfer message")
	}
	return nil
}
