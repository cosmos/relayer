package relayer

import (
	"fmt"

	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"

	chantypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
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
