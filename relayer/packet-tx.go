package relayer

import (
	"context"
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

//nolint:lll
// SendTransferMsg initiates an ics20 transfer from src to dst with the specified args
func (c *Chain) SendTransferMsg(ctx context.Context, log *zap.Logger, dst *Chain, amount sdk.Coin, dstAddr string, toHeightOffset uint64, toTimeOffset time.Duration, srcChannel *chantypes.IdentifiedChannel) error {
	var (
		timeoutHeight    uint64
		timeoutTimestamp uint64
	)

	// get header representing dst to check timeouts
	dsth, err := dst.ChainProvider.QueryLatestHeight(ctx)
	if err != nil {
		return err
	}
	h, err := dst.ChainProvider.GetIBCUpdateHeader(ctx, dsth, c.ChainProvider, c.PathEnd.ClientID)
	if err != nil {
		return err
	}

	// if the timestamp offset is set we need to query the dst chains consensus state to get the current time
	var consensusState ibcexported.ConsensusState
	if toTimeOffset > 0 {
		clientStateRes, err := dst.ChainProvider.QueryClientStateResponse(ctx, dsth, dst.ClientID())
		if err != nil {
			return fmt.Errorf("failed to query the client state response: %w", err)
		}
		clientState, err := clienttypes.UnpackClientState(clientStateRes.ClientState)
		if err != nil {
			return fmt.Errorf("failed to unpack client state: %w", err)
		}
		consensusStateRes, err := dst.ChainProvider.QueryClientConsensusState(ctx, dsth, dst.ClientID(), clientState.GetLatestHeight())
		if err != nil {
			return fmt.Errorf("failed to query client consensus state: %w", err)
		}
		consensusState, err = clienttypes.UnpackConsensusState(consensusStateRes.ConsensusState)
		if err != nil {
			return fmt.Errorf("failed to unpack consensus state: %w", err)
		}

		// use local clock time as reference time if it is later than the
		// consensus state timestamp of the counter party chain, otherwise
		// still use consensus state timestamp as reference.
		// see https://github.com/cosmos/ibc-go/blob/ccc4cb804843f1a80acfb0d4dbf106d1ff2178bb/modules/apps/transfer/client/cli/tx.go#L94-L110
		tmpNow := time.Now().UnixNano()
		consensusTimestamp := consensusState.GetTimestamp()
		now := uint64(tmpNow)
		if now > consensusTimestamp {
			timeoutTimestamp = now + uint64(toTimeOffset)
		} else {
			timeoutTimestamp = consensusTimestamp + uint64(toTimeOffset)
		}
	}

	switch {
	case toHeightOffset > 0 && toTimeOffset > 0:
		timeoutHeight = h.GetHeight().GetRevisionHeight() + toHeightOffset
	case toHeightOffset > 0:
		timeoutHeight = h.GetHeight().GetRevisionHeight() + toHeightOffset
		timeoutTimestamp = 0
	case toTimeOffset > 0:
		timeoutHeight = 0
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
	}

	result := txs.Send(ctx, log, AsRelayMsgSender(c), AsRelayMsgSender(dst), "")
	if err := result.Error(); err != nil {
		if result.PartiallySent() {
			c.log.Info(
				"Partial success when sending transfer",
				zap.String("src_chain_id", c.ChainID()),
				zap.String("dst_chain_id", dst.ChainID()),
				zap.Object("send_result", result),
			)
		}
		return err
	}

	return nil
}
