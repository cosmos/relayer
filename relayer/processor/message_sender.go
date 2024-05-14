package processor

import (
	"context"

	"github.com/cosmos/relayer/v2/relayer/provider"
)

type MessageSender interface {
	trackAndSendMessages(
		ctx context.Context,
		src, dst *pathEndRuntime,
		msgUpdateClient provider.RelayerMessage,
		trackers []messageToTrack,
		needsClientUpdate bool,
	) error
}

type MessageSenderNop struct{}

func (MessageSenderNop) trackAndSendMessages(
	ctx context.Context,
	src, dst *pathEndRuntime,
	msgUpdateClient provider.RelayerMessage,
	trackers []messageToTrack,
	needsClientUpdate bool,
) error {
	return nil
}
