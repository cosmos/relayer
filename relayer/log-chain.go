package relayer

import (
	"fmt"

	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	"go.uber.org/zap"

	conntypes "github.com/cosmos/ibc-go/v3/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
)

func logFailedTx(log *zap.Logger, chainID string, res *provider.RelayerTxResponse, err error, msgs []provider.RelayerMessage) {
	fields := make([]zap.Field, 1+len(msgs), 2+len(msgs))
	fields[0] = zap.String("chain_id", chainID)
	for i, msg := range msgs {
		// TODO add case here for other implementations of provider.RelayerMessage
		switch m := msg.(type) {
		case cosmos.CosmosMessage:
			fields[i+1] = zap.Object(
				fmt.Sprintf("msg-%d", i),
				m,
			)
		default:
			fields[i+1] = zap.Skip()
		}
	}
	if err != nil {
		fields = append(fields, zap.Error(err))
	}
	log.Info("Failed sending transaction", fields...)

	if res != nil && res.Code != 0 && res.Data != "" {
		msgTypes := make([]string, len(msgs))
		for i, msg := range msgs {
			msgTypes[i] = msg.Type()
		}

		log.Info(
			"Sent transaction that resulted in error",
			zap.String("chain_id", chainID),
			zap.Int64("height", res.Height),
			zap.Strings("msg_types", msgTypes),
			zap.Uint32("error_code", res.Code),
			zap.String("error_data", res.Data),
		)
	}

	if res != nil {
		log.Debug("Transaction response", zap.Object("resp", res))
	}
}

// LogFailedTx takes the transaction and the messages to create it and logs the appropriate data
func (c *Chain) LogFailedTx(res *provider.RelayerTxResponse, err error, msgs []provider.RelayerMessage) {
	logFailedTx(c.log, c.ChainID(), res, err, msgs)
}

func (c *Chain) logPacketsRelayed(dst *Chain, num int, srcChannel *chantypes.IdentifiedChannel) {
	c.log.Info(
		"Relayed packets",
		zap.Int("count", num),
		zap.String("from_chain_id", dst.ChainID()),
		zap.String("from_port_id", srcChannel.Counterparty.PortId),
		zap.String("to_chain_id", c.ChainID()),
		zap.String("to_port_id", srcChannel.PortId),
	)
}

func (c *Chain) logInterqueryRelayed(dst *Chain, num int) {
	c.log.Info(
		"Relayed interqueries",
		zap.Int("count", num),
		zap.String("querying_chain_id", dst.ChainID()),
		zap.String("queried_chain_id", c.ChainID()),
	)
}

func logChannelStates(src, dst *Chain, srcChan, dstChan *chantypes.QueryChannelResponse) {
	src.log.Debug(
		"Channel states",
		zap.String("src_chain_id", src.ChainID()),
		zap.Stringer("src_channel_proof_height", MustGetHeight(srcChan.ProofHeight)),
		zap.String("src_channel_id", dstChan.Channel.Counterparty.ChannelId),
		zap.Stringer("src_channel_state", srcChan.Channel.State),

		zap.String("dst_chain_id", dst.ChainID()),
		zap.Stringer("dst_channel_proof_height", MustGetHeight(dstChan.ProofHeight)),
		zap.String("dst_channel_id", srcChan.Channel.Counterparty.ChannelId),
		zap.Stringer("dst_channel_state", dstChan.Channel.State),
	)
}

func logConnectionStates(src, dst *Chain, srcConn, dstConn *conntypes.QueryConnectionResponse) {
	src.log.Debug(
		"Connection states",
		zap.String("src_chain_id", src.ChainID()),
		zap.Stringer("src_conn_proof_height", MustGetHeight(srcConn.ProofHeight)),
		zap.String("src_conn_id", src.ConnectionID()),
		zap.Stringer("src_conn_state", srcConn.Connection.State),

		zap.String("dst_chain_id", dst.ChainID()),
		zap.Stringer("dst_conn_proof_height", MustGetHeight(dstConn.ProofHeight)),
		zap.String("dst_conn_id", dst.ConnectionID()),
		zap.Stringer("dst_conn_state", dstConn.Connection.State),
	)
}

func (c *Chain) errQueryUnrelayedPacketAcks() error {
	return fmt.Errorf("no error on QueryPacketUnrelayedAcknowledgements for %s, however response is nil", c.ChainID())
}

func (c *Chain) LogRetryGetIBCUpdateHeader(n uint, err error) {
	c.log.Info(
		"Failed to get IBC update headers",
		zap.String("chain_id", c.ChainID()),
		zap.Uint("attempt", n+1),
		zap.Uint("max_attempts", RtyAttNum),
		zap.Error(err),
	)
}
