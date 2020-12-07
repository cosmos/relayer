package relayer

import (
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// SendTransferMsg initiates an ibs20 transfer from src to dst with the specified args
func (c *Chain) SendTransferMsg(dst *Chain, amount sdk.Coin, dstAddr fmt.Stringer, toHeightOffset uint64, toTimeOffset time.Duration) error {
	var (
		timeoutHeight    uint64
		timeoutTimestamp uint64
	)

	h, err := dst.UpdateLightWithHeader()
	if err != nil {
		return err
	}

	// Properly render the address string
	dstAddrString := dstAddr.String()

	switch {
	case toHeightOffset > 0 && toTimeOffset > 0:
		return fmt.Errorf("cant set both timeout height and time offset")
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
		Src: []sdk.Msg{c.PathEnd.MsgTransfer(
			dst.PathEnd, amount, dstAddrString, c.MustGetAddress(), timeoutHeight, timeoutTimestamp,
		)},
		Dst: []sdk.Msg{},
	}

	if txs.Send(c, dst); !txs.Success() {
		return fmt.Errorf("failed to send transfer message")
	}
	return nil
}
