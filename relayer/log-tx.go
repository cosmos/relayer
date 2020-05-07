package relayer

import (
	"fmt"
	"strconv"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	connTypes "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/types"
	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
)

// LogFailedTx takes the transaction and the messages to create it and logs the appropriate data
func (c *Chain) LogFailedTx(res sdk.TxResponse, err error, msgs []sdk.Msg) {
	if c.debug {
		c.Log(fmt.Sprintf("- [%s] -> sending transaction:", c.ChainID))
		c.Print(msgs, false, false)
	}

	if err != nil {
		c.logger.Error(fmt.Errorf("- [%s] -> err(%w)", c.ChainID, err).Error())
	}

	if res.Codespace != "" && res.Code != 0 {
		msg, err := GetCodespace(res.Codespace, int(res.Code))
		if err != nil {
			c.logger.Info(err.Error())
		}
		c.logger.Info(fmt.Sprintf("✘ [%s]@{%d} - msg(%s) err(%s: %s)", c.ChainID, res.Height, getMsgAction(msgs), res.Codespace, msg))
	}

	if c.debug && !res.Empty() {
		c.Print(res, false, false)
	}
}

// LogSuccessTx take the transaction and the messages to create it and logs the appropriate data
func (c *Chain) LogSuccessTx(res sdk.TxResponse, msgs []sdk.Msg) {
	c.logger.Info(fmt.Sprintf("✔ [%s]@{%d} - msg(%s) hash(%s)", c.ChainID, res.Height, getMsgAction(msgs), res.TxHash))
}

func (c *Chain) logPacketsRelayed(dst *Chain, num int) {
	dst.Log(fmt.Sprintf("★ Relayed %d packets: [%s]port{%s}->[%s]port{%s}", num, dst.ChainID, dst.PathEnd.PortID, c.ChainID, c.PathEnd.PortID))
}

func logChannelStates(src, dst *Chain, conn map[string]chanTypes.ChannelResponse) {
	// TODO: replace channelID with portID?
	src.Log(fmt.Sprintf("- [%s]@{%d}chan(%s)-{%s} : [%s]@{%d}chan(%s)-{%s}",
		src.ChainID,
		conn[src.ChainID].ProofHeight,
		src.PathEnd.ChannelID,
		conn[src.ChainID].Channel.State,
		dst.ChainID,
		conn[dst.ChainID].ProofHeight,
		dst.PathEnd.ChannelID,
		conn[dst.ChainID].Channel.State,
	))
}

func logConnectionStates(src, dst *Chain, conn map[string]connTypes.ConnectionResponse) {
	src.Log(fmt.Sprintf("- [%s]@{%d}conn(%s)-{%s} : [%s]@{%d}conn(%s)-{%s}",
		src.ChainID,
		conn[src.ChainID].ProofHeight,
		src.PathEnd.ConnectionID,
		conn[src.ChainID].Connection.State,
		dst.ChainID,
		conn[dst.ChainID].ProofHeight,
		dst.PathEnd.ConnectionID,
		conn[dst.ChainID].Connection.State,
	))
}

func (c *Chain) logCreateClient(dst *Chain, dstH uint64) {
	c.Log(fmt.Sprintf("- [%s] -> creating client for [%s]header-height{%d} trust-period(%s)", c.ChainID, dst.ChainID, dstH, dst.GetTrustingPeriod()))
}

func (c *Chain) logTx(events map[string][]string) {
	c.Log(fmt.Sprintf("• [%s]@{%d} - actions(%s) hash(%s)",
		c.ChainID,
		getTxEventHeight(events),
		getTxActions(events["message.action"]),
		events["tx.hash"][0]),
	)
}

func getTxEventHeight(events map[string][]string) int64 {
	if val, ok := events["tx.height"]; ok {
		out, _ := strconv.ParseInt(val[0], 10, 64)
		return out
	}
	return -1
}

func getTxActions(act []string) string {
	out := ""
	for i, a := range act {
		out += fmt.Sprintf("%d:%s,", i, a)
	}
	return strings.TrimSuffix(out, ",")
}
