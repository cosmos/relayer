package relayer

import (
	"fmt"
	"strconv"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	conntypes "github.com/cosmos/cosmos-sdk/x/ibc/core/03-connection/types"
	chantypes "github.com/cosmos/cosmos-sdk/x/ibc/core/04-channel/types"
)

// LogFailedTx takes the transaction and the messages to create it and logs the appropriate data
func (c *Chain) LogFailedTx(res *sdk.TxResponse, err error, msgs []sdk.Msg) {
	if c.debug {
		c.Log(fmt.Sprintf("- [%s] -> sending transaction:", c.ChainID))
		for _, msg := range msgs {
			c.Print(msg, false, false)
		}
	}

	if err != nil {
		c.logger.Error(fmt.Errorf("- [%s] -> err(%v)", c.ChainID, err).Error())
		if res == nil {
			return
		}
	}

	if res.Code != 0 && res.Codespace != "" {
		c.logger.Info(fmt.Sprintf("✘ [%s]@{%d} - msg(%s) err(%s:%d:%s)",
			c.ChainID, res.Height, getMsgAction(msgs), res.Codespace, res.Code, res.RawLog))
	}

	if c.debug && !res.Empty() {
		c.Log("- transaction response:")
		c.Print(res, false, false)
	}
}

// LogSuccessTx take the transaction and the messages to create it and logs the appropriate data
func (c *Chain) LogSuccessTx(res *sdk.TxResponse, msgs []sdk.Msg) {
	c.logger.Info(fmt.Sprintf("✔ [%s]@{%d} - msg(%s) hash(%s)", c.ChainID, res.Height, getMsgAction(msgs), res.TxHash))
}

func (c *Chain) logPacketsRelayed(dst *Chain, num int) {
	dst.Log(fmt.Sprintf("★ Relayed %d packets: [%s]port{%s}->[%s]port{%s}",
		num, dst.ChainID, dst.PathEnd.PortID, c.ChainID, c.PathEnd.PortID))
}

func logChannelStates(src, dst *Chain, srcChan, dstChan *chantypes.QueryChannelResponse) {
	src.Log(fmt.Sprintf("- [%s]@{%d}chan(%s)-{%s} : [%s]@{%d}chan(%s)-{%s}",
		src.ChainID,
		MustGetHeight(srcChan.ProofHeight),
		src.PathEnd.ChannelID,
		srcChan.Channel.State,
		dst.ChainID,
		MustGetHeight(dstChan.ProofHeight),
		dst.PathEnd.ChannelID,
		dstChan.Channel.State,
	))
}

func logConnectionStates(src, dst *Chain, srcConn, dstConn *conntypes.QueryConnectionResponse) {
	src.Log(fmt.Sprintf("- [%s]@{%d}conn(%s)-{%s} : [%s]@{%d}conn(%s)-{%s}",
		src.ChainID,
		MustGetHeight(srcConn.ProofHeight),
		src.PathEnd.ConnectionID,
		srcConn.Connection.State,
		dst.ChainID,
		MustGetHeight(dstConn.ProofHeight),
		dst.PathEnd.ConnectionID,
		dstConn.Connection.State,
	))
}

func (c *Chain) logCreateClient(dst *Chain, dstH int64) {
	c.Log(fmt.Sprintf("- [%s] -> creating client on %s for %s header-height{%d} trust-period(%s)",
		c.ChainID, c.ChainID, dst.ChainID, dstH, dst.GetTrustingPeriod()))
}

func (c *Chain) logTx(events map[string][]string) {
	hash := ""
	if len(events["tx.hash"]) > 0 {
		hash = events["tx.hash"][0]
	}
	c.Log(fmt.Sprintf("• [%s]@{%d} - actions(%s) hash(%s)",
		c.ChainID,
		getTxEventHeight(events),
		getTxActions(events["message.action"]),
		hash),
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

func logRetryUpdateHeaders(src, dst *Chain, n uint, err error) {
	if src.debug && dst.debug {
		src.Log(fmt.Sprintf("- [%s]&[%s] - try(%d/%d) update headers: %s", src.ChainID, dst.ChainID, n+1, rtyAttNum, err))
	}
}

func (c *Chain) logRetryQueryPacketAcknowledgements(height uint64, n uint, err error) {
	if c.debug {
		c.Log(fmt.Sprintf("- [%s]@{%d} - try(%d/%d) query packet acknowledgements: %s", c.ChainID, height, n+1, rtyAttNum, err))
	}
}

func (c *Chain) errQueryUnrelayedPacketAcks() error {
	return fmt.Errorf("No error on QueryPacketUnrelayedAcknowledgements for %s, however response is nil", c.ChainID)
}
