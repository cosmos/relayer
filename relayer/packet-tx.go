package relayer

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	commitmentypes "github.com/cosmos/cosmos-sdk/x/ibc/23-commitment/types"
)

var (
	defaultChainPrefix     = commitmentypes.NewMerklePrefix([]byte("ibc"))
	defaultPacketSendQuery = "send_packet.packet_src_channel=%s&send_packet.packet_sequence=%d"
	// defaultPacketAckQuery  = "recv_packet.packet_src_channel=%s&recv_packet.packet_sequence=%d"
)

// SendTransferMsg initiates an ibs20 transfer from src to dst with the specified args
func (c *Chain) SendTransferMsg(dst *Chain, amount sdk.Coin, dstAddr fmt.Stringer) error {
	h, err := dst.UpdateLightWithHeader()
	if err != nil {
		return err
	}

	// Properly render the address string
	dst.UseSDKContext()
	dstAddrString := dstAddr.String()

	// MsgTransfer will call SendPacket on src chain
	// TODO: Add ability to specify timeout time or height via command line flags
	txs := RelayMsgs{
		Src: []sdk.Msg{c.PathEnd.MsgTransfer(
			dst.PathEnd, amount, dstAddrString, c.MustGetAddress(), uint64(h.Header.Height+1000), 0,
		)},
		Dst: []sdk.Msg{},
	}

	if txs.Send(c, dst); !txs.success {
		return fmt.Errorf("failed to send transfer message")
	}
	return nil
}
