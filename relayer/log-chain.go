package relayer

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cosmos/relayer/relayer/provider"
	"github.com/cosmos/relayer/relayer/provider/cosmos"

	sdk "github.com/cosmos/cosmos-sdk/types"
	conntypes "github.com/cosmos/ibc-go/v3/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
)

// LogFailedTx takes the transaction and the messages to create it and logs the appropriate data
func (c *Chain) LogFailedTx(res *provider.RelayerTxResponse, err error, msgs []provider.RelayerMessage) {
	if c.debug {
		c.Log(fmt.Sprintf("- [%s] -> failed sending transaction:", c.ChainID()))
		for _, msg := range msgs {
			_ = c.Print(cosmos.CosmosMsg(msg), false, false)
		}
	}

	if err != nil {
		c.logger.Error(fmt.Errorf("- [%s] -> err(%v)", c.ChainID(), err).Error())
		if res == nil {
			return
		}
	}

	if res != nil {
		if res.Code != 0 && res.Data != "" {
			c.logger.Info(fmt.Sprintf("✘ [%s]@{%d} - msg(%s) err(%d:%s)",
				c.ChainID(), res.Height, getMsgTypes(msgs), res.Code, res.Data))
		}
	}

	if c.debug && res != nil {
		c.Log("- transaction response:")
		_ = c.PrintRelayerTxResponse(res, false, false)
	}
}

func (c *Chain) PrintRelayerTxResponse(res *provider.RelayerTxResponse, text, indent bool) error {
	var (
		out []byte
		err error
	)

	switch {
	case indent && text:
		return fmt.Errorf("must pass either indent or text, not both")
	case text:
		// TODO: This isn't really a good option,
		out = []byte(fmt.Sprintf("%v", res))
	default:
		out, err = json.Marshal(res)
	}

	if err != nil {
		return err
	}

	fmt.Println(string(out))
	return nil
}

// LogSuccessTx take the transaction and the messages to create it and logs the appropriate data
func (c *Chain) LogSuccessTx(res *sdk.TxResponse, msgs []provider.RelayerMessage) {
	c.logger.Info(fmt.Sprintf("✔ [%s]@{%d} - msg(%s) hash(%s)", c.ChainID(), res.Height, getMsgTypes(msgs), res.TxHash))
}

func (c *Chain) logPacketsRelayed(dst *Chain, num int) {
	dst.Log(fmt.Sprintf("★ Relayed %d packets: [%s]port{%s}->[%s]port{%s}",
		num, dst.ChainID(), dst.PathEnd.PortID, c.ChainID(), c.PathEnd.PortID))
}

func logChannelStates(src, dst *Chain, srcChan, dstChan *chantypes.QueryChannelResponse) {
	src.Log(fmt.Sprintf("- [%s]@{%d}chan(%s)-{%s} : [%s]@{%d}chan(%s)-{%s}",
		src.ChainID(),
		MustGetHeight(srcChan.ProofHeight),
		src.PathEnd.ChannelID,
		srcChan.Channel.State,
		dst.ChainID(),
		MustGetHeight(dstChan.ProofHeight),
		dst.PathEnd.ChannelID,
		dstChan.Channel.State,
	))
}

func logConnectionStates(src, dst *Chain, srcConn, dstConn *conntypes.QueryConnectionResponse) {
	src.Log(fmt.Sprintf("- [%s]@{%d}conn(%s)-{%s} : [%s]@{%d}conn(%s)-{%s}",
		src.ChainID(),
		MustGetHeight(srcConn.ProofHeight),
		src.PathEnd.ConnectionID,
		srcConn.Connection.State,
		dst.ChainID(),
		MustGetHeight(dstConn.ProofHeight),
		dst.PathEnd.ConnectionID,
		dstConn.Connection.State,
	))
}

func (c *Chain) logCreateClient(dst *Chain, dstH uint64, tp *time.Duration) {
	c.Log(fmt.Sprintf("- [%s] -> creating client on %s for %s header-height{%d} trust-period(%s)",
		c.ChainID(), c.ChainID(), dst.ChainID(), dstH, tp))
}

func (c *Chain) logOpenInit(dst *Chain, connOrChan string) {
	c.Log(fmt.Sprintf("- attempting to create new %s ends from chain[%s] with chain[%s]",
		connOrChan, c.ChainID(), dst.ChainID()))
}

func (c *Chain) logOpenTry(dst *Chain, connOrChan string) {
	c.Log(fmt.Sprintf("- chain[%s] trying to open %s end on chain[%s]",
		c.ChainID(), connOrChan, dst.ChainID()))
}

func (c *Chain) logIdentifierExists(dst *Chain, identifierType string, id string) {
	c.Log(fmt.Sprintf("- identical %s(%s) on %s with %s already exists",
		identifierType, id, c.ChainID(), dst.ChainID()))
}

func (c *Chain) logTx(events map[string][]string) {
	hash := ""
	if len(events["tx.hash"]) > 0 {
		hash = events["tx.hash"][0]
	}
	c.Log(fmt.Sprintf("• [%s]@{%d} - actions(%s) hash(%s)",
		c.ChainID(),
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

func (c *Chain) logRetryQueryPacketAcknowledgements(height uint64, n uint, err error) {
	if c.debug {
		c.Log(fmt.Sprintf("- [%s]@{%d} - try(%d/%d) query packet acknowledgements: %s",
			c.ChainID(), height, n+1, RtyAttNum, err))
	}
}

func (c *Chain) logUnreceivedPackets(dst *Chain, packetType string, log string) {
	c.Log(fmt.Sprintf("- unrelayed packet %s sent by %s to %s: %s", packetType, c.ChainID(), dst.ChainID(), log))
}

func (c *Chain) errQueryUnrelayedPacketAcks() error {
	return fmt.Errorf("no error on QueryPacketUnrelayedAcknowledgements for %s, however response is nil", c.ChainID())
}

func (c *Chain) LogRetryGetIBCUpdateHeader(n uint, err error) {
	if c.debug {
		c.Log(fmt.Sprintf("failed to get IBC update headers, try(%d/%d). Err: %v", n+1, RtyAttNum, err))
	}
}

func (c *Chain) LogRetryGetLightSignedHeader(n uint, err error) {
	if c.debug {
		c.Log(fmt.Sprintf("failed to get light signed header, try(%d/%d). Err: %v", n+1, RtyAttNum, err))
	}
}
