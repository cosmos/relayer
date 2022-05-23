package processor

import (
	"context"
	"sync"

	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"

	"github.com/cosmos/relayer/v2/relayer/ibc"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

// ChainProcessor interface is reponsible for polling blocks and emitting ibc message events to the PathProcessors
// it is also responsible for tracking open channels and not sending messages to the PathProcessors for closed channels
type ChainProcessor interface {
	// starts the query loop for the chain which will gather applicable ibc messages and push events out to the relevant PathProcessors
	Start(context.Context, chan<- error)

	// makes sure packet is valid to be relayed
	// should return ibc.TimeoutError or ibc.TimeoutOnCloseError if packet is timed out so that Timeout can be written to other chain
	ValidatePacket(provider.RelayerMessage) error

	// MsgTransfer provided, expect MsgRecvPacket for counterparty chain to be returned
	GetMsgRecvPacket(signer string, msgTransfer provider.RelayerMessage) (provider.RelayerMessage, error)

	// MsgRecvPacket provided, expect MsgAcknowledgement for counterparty chain to be returned
	GetMsgAcknowledgement(signer string, msgRecvPacket provider.RelayerMessage) (provider.RelayerMessage, error)

	// MsgTransfer provided, expect MsgTimeout for source chain to be returned
	GetMsgTimeout(signer string, msgTransfer provider.RelayerMessage) (provider.RelayerMessage, error)

	// MsgTransfer provided, expect MsgTimeoutOnClose for source chain to be returned
	GetMsgTimeoutOnClose(signer string, msgTransfer provider.RelayerMessage) (provider.RelayerMessage, error)

	// Counterparty latest signed header provided, expect MsgUpdateClient to be returned
	GetMsgUpdateClient(clientID string, counterpartyChainLatestHeader ibcexported.Header) (provider.RelayerMessage, error)

	// get current height of client
	ClientHeight(clientID string) (clienttypes.Height, error)

	// latest block header with trusted vals for client height + 1
	LatestHeaderWithTrustedVals(height uint64) (ibcexported.Header, error)

	// latest block height, timestamp
	Latest() ibc.LatestBlock

	// are queries in sync with latest height of the chain?
	// path processors use this as a signal for determining if packets should be relayed or not
	InSync() bool

	// returns chain provider
	Provider() provider.ChainProvider
}

type ChainProcessors []ChainProcessor

// blocking call that launches all chain processors in parallel (main process)
func (cp ChainProcessors) Start(ctx context.Context, errCh chan<- error) {
	infiniteWait := sync.WaitGroup{}
	// will never be finished
	infiniteWait.Add(1)
	for _, chainProcessor := range cp {
		go chainProcessor.Start(ctx, errCh)
	}
	infiniteWait.Wait()
}
