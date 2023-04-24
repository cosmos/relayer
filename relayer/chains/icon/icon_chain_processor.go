package icon

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/avast/retry-go/v4"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/chains/icon/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/gorilla/websocket"
	"github.com/icon-project/goloop/common"
	"github.com/icon-project/goloop/common/codec"
)

const (
	queryTimeout                = 5 * time.Second
	blockResultsQueryTimeout    = 2 * time.Minute
	latestHeightQueryRetryDelay = 1 * time.Second
	latestHeightQueryRetries    = 5

	defaultMinQueryLoopDuration      = 1 * time.Second
	defaultBalanceUpdateWaitDuration = 60 * time.Second
	inSyncNumBlocksThreshold         = 2
	BTP_MESSAGE_CHAN_CAPACITY        = 1000
	INCOMING_BN_CAPACITY             = 1000
	ERROR_CAPACITY                   = 2
)

type IconChainProcessor struct {
	log           *zap.Logger
	chainProvider *IconProvider

	pathProcessors processor.PathProcessors

	inSync bool

	latestBlock   provider.LatestBlock
	latestBlockMu sync.Mutex

	latestClientState

	// holds open state for known connections
	connectionStateCache processor.ConnectionStateCache

	// holds open state for known channels
	channelStateCache processor.ChannelStateCache

	// map of connection ID to client ID
	connectionClients map[string]string

	// map of channel ID to connection ID
	channelConnections map[string]string

	// metrics to monitor lifetime of processor
	metrics *processor.PrometheusMetrics
}

func NewIconChainProcessor(log *zap.Logger, provider *IconProvider, metrics *processor.PrometheusMetrics) *IconChainProcessor {
	return &IconChainProcessor{
		log:                  log.With(zap.String("chain_name", "Icon")),
		chainProvider:        provider,
		latestClientState:    make(latestClientState),
		connectionStateCache: make(processor.ConnectionStateCache),
		channelStateCache:    make(processor.ChannelStateCache),
		connectionClients:    make(map[string]string),
		channelConnections:   make(map[string]string),
		metrics:              metrics,
	}
}

// Arrangement For the Latest height
type latestClientState map[string]provider.ClientState

func (l latestClientState) update(ctx context.Context, clientInfo clientInfo, icp *IconChainProcessor) {
	existingClientInfo, ok := l[clientInfo.clientID]
	var trustingPeriod time.Duration
	if ok {
		if clientInfo.consensusHeight.LT(existingClientInfo.ConsensusHeight) {
			// height is less than latest, so no-op
			return
		}
		trustingPeriod = existingClientInfo.TrustingPeriod
	}
	// if trustingPeriod.Milliseconds() == 0 {
	// 	cs, err := icp.chainProvider.QueryClientState(ctx, int64(icp.latestBlock.Height), clientInfo.clientID)
	// 	if err == nil {
	// 		trustingPeriod = cs.TrustingPeriod
	// 	}
	// }
	clientState := clientInfo.ClientState()
	clientState.TrustingPeriod = trustingPeriod

	// update latest if no existing state or provided consensus height is newer
	l[clientInfo.clientID] = clientState
}

// ********************************* Priority queue interface for BlockNotification *********************************
type BlockNotificationPriorityQueue []*types.BlockNotification

func (pq BlockNotificationPriorityQueue) Len() int { return len(pq) }

func (pq BlockNotificationPriorityQueue) Less(i, j int) bool {
	height_i, _ := pq[i].Height.BigInt()
	height_j, _ := pq[j].Height.BigInt()
	return height_i.Cmp(height_j) == -1
}

func (pq BlockNotificationPriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *BlockNotificationPriorityQueue) Push(x interface{}) {
	*pq = append(*pq, x.(*types.BlockNotification))
}

func (pq *BlockNotificationPriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}

// ************************************************** For persistence **************************************************
type queryCyclePersistence struct {
	latestHeight int64
	// latestHeightMu sync.Mutex

	lastQueriedHeight     int64
	latestQueriedHeightMu sync.Mutex

	minQueryLoopDuration time.Duration
}

func (icp *IconChainProcessor) Run(ctx context.Context, initialBlockHistory uint64) error {

	persistence := queryCyclePersistence{
		minQueryLoopDuration: time.Second,
	}

	height, err := icp.getLatestHeightWithRetry(ctx)
	if err != nil {
		icp.log.Error("Failed to query latest height",
			zap.Error(err),
		)
		return err
	}
	persistence.latestHeight = height

	lastQueriedBlock := persistence.latestHeight - int64(initialBlockHistory)
	if lastQueriedBlock < 0 {
		lastQueriedBlock = 1
	}
	persistence.lastQueriedHeight = lastQueriedBlock

	var eg errgroup.Group

	eg.Go(func() error {
		return icp.initializeConnectionState(ctx)
	})
	eg.Go(func() error {
		return icp.initializeChannelState(ctx)
	})
	if err := eg.Wait(); err != nil {
		return err
	}

	// start_query_cycle
	icp.log.Debug(" **************** Entering main query loop **************** ")
	err = icp.monitoring(ctx, &persistence)
	return err
}

func (icp *IconChainProcessor) initializeConnectionState(ctx context.Context) error {
	// TODO:
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	connections, err := icp.chainProvider.QueryConnections(ctx)
	if err != nil {
		return fmt.Errorf("error querying connections: %w", err)
	}

	for _, c := range connections {
		icp.connectionClients[c.Id] = c.ClientId
		icp.connectionStateCache[processor.ConnectionKey{
			ConnectionID:         c.Id,
			ClientID:             c.ClientId,
			CounterpartyConnID:   c.Counterparty.ConnectionId,
			CounterpartyClientID: c.Counterparty.ClientId,
		}] = c.State == conntypes.OPEN
	}
	return nil
}

func (icp *IconChainProcessor) initializeChannelState(ctx context.Context) error {
	// TODO:
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	channels, err := icp.chainProvider.QueryChannels(ctx)
	if err != nil {
		return fmt.Errorf("error querying channels: %w", err)
	}
	for _, ch := range channels {
		if len(ch.ConnectionHops) != 1 {
			icp.log.Error("Found channel using multiple connection hops. Not currently supported, ignoring.",
				zap.String("channel_id", ch.ChannelId),
				zap.String("port_id", ch.PortId),
				zap.Strings("connection_hops", ch.ConnectionHops),
			)
			continue
		}
		icp.channelConnections[ch.ChannelId] = ch.ConnectionHops[0]
		icp.channelStateCache[processor.ChannelKey{
			ChannelID:             ch.ChannelId,
			PortID:                ch.PortId,
			CounterpartyChannelID: ch.Counterparty.ChannelId,
			CounterpartyPortID:    ch.Counterparty.PortId,
		}] = ch.State == chantypes.OPEN
	}

	icp.log.Info("Initialize channel cache",
		zap.Any("ChannelStateCache", icp.channelStateCache))

	return nil
}

func (icp *IconChainProcessor) Provider() provider.ChainProvider {
	return icp.chainProvider
}

func (icp *IconChainProcessor) SetPathProcessors(pathProcessors processor.PathProcessors) {
	icp.pathProcessors = pathProcessors
}

func (icp *IconChainProcessor) GetLatestHeight() uint64 {
	return icp.latestBlock.Height
}

func (icp *IconChainProcessor) monitoring(ctx context.Context, persistence *queryCyclePersistence) error {

	btpBlockReceived := make(chan IconIBCHeader, BTP_MESSAGE_CHAN_CAPACITY)
	incomingEventsBN := make(chan *types.BlockNotification, INCOMING_BN_CAPACITY)
	monitorErr := make(chan error, ERROR_CAPACITY)

	if icp.chainProvider.PCfg.IbcHandlerAddress == "" || icp.chainProvider.PCfg.BTPNetworkID == 0 {
		return errors.New("IbcHandlerAddress or NetworkId not found")
	}

	ibcHeaderCache := make(processor.IBCHeaderCache)

	header := &types.BTPBlockHeader{}
	if err := retry.Do(func() error {
		var err error
		header, err = icp.chainProvider.GetBtpHeader(&types.BTPBlockParam{
			Height:    types.NewHexInt(icp.chainProvider.PCfg.BTPHeight),
			NetworkId: types.NewHexInt(icp.chainProvider.PCfg.BTPNetworkID),
		})
		if err != nil {
			if strings.Contains(err.Error(), "NotFound: E1005:fail to get a BTP block header for") {
				icp.log.Info("Provided Height doesn't contain BTP header:",
					zap.String("ChainName", icp.chainProvider.ChainId()),
					zap.Int64("Height", icp.chainProvider.PCfg.BTPHeight),
					zap.Int64("Network Id", icp.chainProvider.PCfg.BTPNetworkID),
				)
				return nil
			}
			return err
		}

		icp.inSync = true
		ibcHeader := NewIconIBCHeader(header)
		icp.latestBlock = provider.LatestBlock{
			Height: ibcHeader.Height(),
		}

		ibcHeaderCache[uint64(header.MainHeight)] = ibcHeader
		ibcMessagesCache := processor.NewIBCMessagesCache()
		err = icp.handlePathProcessorUpdate(ctx, ibcHeader, ibcMessagesCache, ibcHeaderCache.Clone())
		if err != nil {
			return err
		}

		return nil
	}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
		icp.log.Info(
			"Failed to get header",
			zap.String("ChainName", icp.chainProvider.ChainId()),
			zap.Int64("Height", icp.chainProvider.PCfg.BTPHeight),
			zap.Int64("Network Id", icp.chainProvider.PCfg.BTPNetworkID),
			zap.Error(err),
		)
	})); err != nil {
		return err
	}

	// request parameters
	reqBTPBlocks := &types.BTPRequest{
		Height:    types.NewHexInt(icp.chainProvider.PCfg.BTPHeight),
		NetworkID: types.NewHexInt(icp.chainProvider.PCfg.BTPNetworkID),
		ProofFlag: types.NewHexInt(0),
	}
	reqIconBlocks := &types.BlockRequest{
		Height:       types.NewHexInt(int64(icp.chainProvider.PCfg.BTPHeight)),
		EventFilters: GetMonitorEventFilters(icp.chainProvider.PCfg.IbcHandlerAddress),
	}

	// initalize the processors

	// Create the priority queue and initialize it.
	incomingEventsQueue := &BlockNotificationPriorityQueue{}
	heap.Init(incomingEventsQueue)

	// Start monitoring BTP blocks
	go icp.monitorBTP2Block(ctx, reqBTPBlocks, btpBlockReceived, monitorErr)

	// Start monitoring Icon blocks for eventlogs
	go icp.monitorIconBlock(ctx, reqIconBlocks, incomingEventsBN, monitorErr)

	// ticker
	ticker := time.NewTicker(persistence.minQueryLoopDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Context has been cancelled, stop the loop
			icp.log.Debug("Icon chain closed")
			return nil

		case err := <-monitorErr:
			return err
		case h := <-btpBlockReceived:
			ibcHeaderCache[h.Height()] = &h
			icp.latestBlock = provider.LatestBlock{
				Height: uint64(h.Height()),
			}

		case incomingBN := <-incomingEventsBN:
			heap.Push(incomingEventsQueue, incomingBN)

		case <-ticker.C:
			// Process the block notifications from the priority queue.
			for incomingEventsQueue.Len() > 0 {

				ibcMessagesCache := processor.NewIBCMessagesCache()
				incomingBN := heap.Pop(incomingEventsQueue).(*types.BlockNotification)
				h, _ := (incomingBN.Height).Int()
				header, ok := ibcHeaderCache[uint64(h)]
				if !ok {
					heap.Push(incomingEventsQueue, incomingBN)
					break
				}
				icp.log.Info("Incoming sequence ",
					zap.String("ChainName", icp.chainProvider.ChainId()),
					zap.Int64("Height", int64(h)),
				)
				persistence.latestQueriedHeightMu.Lock()
				persistence.lastQueriedHeight = int64(header.Height())
				persistence.latestQueriedHeightMu.Unlock()

				ibcMessages, err := icp.handleBlockEventRequest(incomingBN)
				if err != nil {
					icp.log.Error(
						fmt.Sprintf("Failed handleBlockEventRequest at height:%v", incomingBN.Height),
						zap.Error(err),
					)
				}
				for _, m := range ibcMessages {
					icp.handleMessage(ctx, *m, ibcMessagesCache)
				}
				icp.inSync = true
				icp.handlePathProcessorUpdate(ctx, header, ibcMessagesCache, ibcHeaderCache.Clone())
			}

		}

	}
}

func (icp *IconChainProcessor) handlePathProcessorUpdate(ctx context.Context,
	latestHeader provider.IBCHeader, messageCache processor.IBCMessagesCache,
	ibcHeaderCache processor.IBCHeaderCache) error {
	chainID := icp.chainProvider.ChainId()

	for _, pp := range icp.pathProcessors {
		clientID := pp.RelevantClientID(chainID)
		clientState, err := icp.clientState(ctx, clientID)
		if err != nil {
			icp.log.Error("Error fetching client state",
				zap.String("client_id", clientID),
				zap.Error(err),
			)
			continue
		}

		pp.HandleNewData(chainID, processor.ChainProcessorCacheData{
			LatestBlock:          icp.latestBlock,
			LatestHeader:         latestHeader,
			IBCMessagesCache:     messageCache,
			InSync:               icp.inSync,
			ClientState:          clientState,
			ConnectionStateCache: icp.connectionStateCache.FilterForClient(clientID),
			ChannelStateCache:    icp.channelStateCache.FilterForClient(clientID, icp.channelConnections, icp.connectionClients),
			IBCHeaderCache:       ibcHeaderCache,
		})
	}
	return nil

}

func (icp *IconChainProcessor) monitorBTP2Block(ctx context.Context, req *types.BTPRequest, receiverChan chan IconIBCHeader, errChan chan error) {

	go func() {
		err := icp.chainProvider.client.MonitorBTP(ctx, req, func(conn *websocket.Conn, v *types.BTPNotification) error {

			bh := &types.BTPBlockHeader{}
			_, err := Base64ToData(v.Header, bh)
			if err != nil {
				return err
			}
			icp.chainProvider.UpdateLastBTPBlockHeight(uint64(bh.MainHeight))
			btpBLockWithProof := NewIconIBCHeader(bh)
			receiverChan <- *btpBLockWithProof
			return nil
		}, func(conn *websocket.Conn) {
		}, func(conn *websocket.Conn, err error) {
			icp.log.Debug(fmt.Sprintf("onError %s err:%+v", conn.LocalAddr().String(), err))
			_ = conn.Close()
			errChan <- err
		})
		if err != nil {
			errChan <- err
		}
	}()
}

func (icp *IconChainProcessor) monitorIconBlock(ctx context.Context, req *types.BlockRequest, incomingEventBN chan *types.BlockNotification, errChan chan error) {

	go func() {
		err := icp.chainProvider.client.MonitorBlock(ctx, req, func(conn *websocket.Conn, v *types.BlockNotification) error {
			if len(v.Indexes) > 0 && len(v.Events) > 0 {
				incomingEventBN <- v
			}
			return nil
		}, func(conn *websocket.Conn) {
		}, func(conn *websocket.Conn, err error) {
			log.Println(fmt.Sprintf("onError %s err:%+v", conn.LocalAddr().String(), err))
			_ = conn.Close()
			errChan <- err
		})
		if err != nil {
			errChan <- err
		}
	}()

}

func (icp *IconChainProcessor) handleBlockEventRequest(request *types.BlockNotification) ([]*ibcMessage, error) {

	height, _ := request.Height.Int()
	blockHeader, err := icp.chainProvider.client.GetBlockHeaderByHeight(int64(height))
	if err != nil {
		return nil, err
	}

	var receiptHash types.BlockHeaderResult
	_, err = codec.RLP.UnmarshalFromBytes(blockHeader.Result, &receiptHash)
	if err != nil {
		return nil, err
	}

	var ibcMessages []*ibcMessage
	for id := 0; id < len(request.Indexes); id++ {
		for i, index := range request.Indexes[id] {
			p := &types.ProofEventsParam{
				Index:     index,
				BlockHash: request.Hash,
				Events:    request.Events[id][i],
			}

			proofs, err := icp.chainProvider.client.GetProofForEvents(p)
			if err != nil {
				icp.log.Info("Error occured when fetching proof", zap.Error(err))
				continue
			}

			// Processing receipt index
			serializedReceipt, err := MptProve(index, proofs[0], receiptHash.ReceiptHash)
			if err != nil {
				return nil, err
			}
			var result types.TxResult
			_, err = codec.RLP.UnmarshalFromBytes(serializedReceipt, &result)
			if err != nil {
				return nil, err
			}

			for j := 0; j < len(p.Events); j++ {
				serializedEventLog, err := MptProve(
					p.Events[j], proofs[j+1], common.HexBytes(result.EventLogsHash))
				if err != nil {
					return nil, err
				}
				var el types.EventLog
				_, err = codec.RLP.UnmarshalFromBytes(serializedEventLog, &el)
				if err != nil {
					return nil, err
				}

				ibcMessage := parseIBCMessageFromEvent(icp.log, el, uint64(height))
				ibcMessages = append(ibcMessages, ibcMessage)
			}

		}
	}

	return ibcMessages, nil
}

// clientState will return the most recent client state if client messages
// have already been observed for the clientID, otherwise it will query for it.
func (icp *IconChainProcessor) clientState(ctx context.Context, clientID string) (provider.ClientState, error) {
	if state, ok := icp.latestClientState[clientID]; ok {
		return state, nil
	}
	cs, err := icp.chainProvider.QueryClientState(ctx, int64(icp.latestBlock.Height), clientID)
	if err != nil {
		return provider.ClientState{}, err
	}

	clientState := provider.ClientState{
		ClientID:        clientID,
		ConsensusHeight: cs.GetLatestHeight().(clienttypes.Height),
	}
	icp.latestClientState[clientID] = clientState
	return clientState, nil
}

func (icp *IconChainProcessor) getLatestHeightWithRetry(ctx context.Context) (int64, error) {
	var blk *types.Block
	var err error
	for i := 0; i < latestHeightQueryRetries; i++ {
		blk, err = icp.chainProvider.client.GetLastBlock()
		if err != nil {

			icp.log.Warn("Failed to query latest height",
				zap.Int("attempts", i),
				zap.Error(err),
			)
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return 0, nil
			}
			continue
		}
		break
	}
	return blk.Height, err
}
