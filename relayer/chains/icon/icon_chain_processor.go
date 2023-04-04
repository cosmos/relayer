package icon

import (
	"container/heap"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
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

	//highest block
	latestBlock provider.LatestBlock

	//
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
	// TODO:
	// if trustingPeriod.Milliseconds() == 0 {
	// 	cs, err := icp.chainProvider.queryICONClientState(ctx, int64(icp.latestBlock.Height), clientInfo.clientID)
	// 	if err == nil {
	// 		trustingPeriod = cs.TrustingPeriod
	// 	}
	// }
	clientState := clientInfo.ClientState()
	clientState.TrustingPeriod = trustingPeriod

	// update latest if no existing state or provided consensus height is newer
	l[clientInfo.clientID] = clientState
}

// Implementing the Priority queue interface for BlockNotification
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

// For persistence
type queryCyclePersistence struct {
	latestHeight      int64
	lastQueriedHeight int64
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

func (icp *IconChainProcessor) Run(ctx context.Context, initialBlockHistory uint64) error {

	persistence := queryCyclePersistence{}

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
	icp.log.Debug("Entering main query loop")
	err = icp.monitoring(ctx, persistence)
	return err
}

func (icp *IconChainProcessor) initializeConnectionState(ctx context.Context) error {
	// TODO:
	return nil
}

func (icp *IconChainProcessor) initializeChannelState(ctx context.Context) error {
	// TODO:
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

func (icp *IconChainProcessor) monitoring(ctx context.Context, persistence queryCyclePersistence) error {

	// chan
	btpBlockReceived := make(chan IconIBCHeader, BTP_MESSAGE_CHAN_CAPACITY)
	incomingEventsBN := make(chan *types.BlockNotification, INCOMING_BN_CAPACITY)
	monitorErr := make(chan error, ERROR_CAPACITY)
	chainID := icp.chainProvider.ChainId()

	// caches
	ibcHeaderCache := make(processor.IBCHeaderCache)
	ibcMessagesCache := processor.NewIBCMessagesCache()

	//checking handlerAddress
	if icp.chainProvider.PCfg.IbcHandlerAddress == "" || icp.chainProvider.PCfg.BTPNetworkID == 0 {
		return errors.New("IbcHandlerAddress is not provided")
	}

	// request parameters
	reqBTPBlocks := &types.BTPRequest{
		Height:    types.NewHexInt((persistence.lastQueriedHeight)),
		NetworkID: types.NewHexInt(icp.chainProvider.PCfg.BTPNetworkID),
		ProofFlag: types.NewHexInt(0),
	}
	reqIconBlocks := &types.BlockRequest{
		Height:       types.NewHexInt(int64(persistence.lastQueriedHeight)),
		EventFilters: GetMonitorEventFilters(icp.chainProvider.PCfg.IbcHandlerAddress),
	}

	// Create the priority queue and initialize it.
	incomingEventsQueue := &BlockNotificationPriorityQueue{}
	heap.Init(incomingEventsQueue)

	// Start monitoring BTP blocks
	go icp.monitorBTP2Block(ctx, reqBTPBlocks, btpBlockReceived, monitorErr)

	// Start monitoring Icon blocks for eventlogs
	go icp.monitorIconBlock(ctx, reqIconBlocks, incomingEventsBN, monitorErr)

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

		case incomingBN := <-incomingEventsBN:

			// Add the incoming block notification to the priority queue.
			heap.Push(incomingEventsQueue, incomingBN)

		default:
			// Process the block notifications from the priority queue.
			for incomingEventsQueue.Len() > 0 {
				incomingBN := heap.Pop(incomingEventsQueue).(*types.BlockNotification)
				h, _ := (incomingBN.Height).Int()
				header, ok := ibcHeaderCache[uint64(h)]
				if !ok {
					// Add the block notification back to the priority queue.
					heap.Push(incomingEventsQueue, incomingBN)
					// Break the loop and wait for the missing header.
					break
				}
				persistence.lastQueriedHeight = int64(header.Height())
				ibcMessages, err := icp.handleBlockEventRequest(incomingBN)
				if err != nil {
					icp.log.Error(
						fmt.Sprintf("failed handleBlockEventRequest at height%v", incomingBN.Height),
						zap.Error(err),
					)
				}
				for _, m := range ibcMessages {
					icp.handleMessage(ctx, *m, ibcMessagesCache)
				}

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
						LatestHeader:         ibcHeaderCache[uint64(h)],
						IBCMessagesCache:     ibcMessagesCache.Clone(),
						InSync:               true,
						ClientState:          clientState,
						ConnectionStateCache: icp.connectionStateCache.FilterForClient(clientID),
						ChannelStateCache:    icp.channelStateCache.FilterForClient(clientID, icp.channelConnections, icp.connectionClients),
						IBCHeaderCache:       ibcHeaderCache.Clone(),
					})
				}
			}

		}

	}
}

func (icp *IconChainProcessor) monitorBTP2Block(ctx context.Context, req *types.BTPRequest, receiverChan chan IconIBCHeader, errChan chan error) {

	go func() {
		err := icp.chainProvider.client.MonitorBTP(ctx, req, func(conn *websocket.Conn, v *types.BTPNotification) error {

			h, err := base64.StdEncoding.DecodeString(v.Header)
			if err != nil {
				return err
			}

			bh := &types.BTPBlockHeader{}
			if _, err = codec.RLP.UnmarshalFromBytes(h, bh); err != nil {
				return err
			}

			btpBLockWithProof := NewIconIBCHeader(bh)
			receiverChan <- *btpBLockWithProof
			return nil
		}, func(conn *websocket.Conn) {
			log.Println(fmt.Sprintf("MonitorBtpBlock"))
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
			log.Println(fmt.Sprintf("MonitorIconLoop"))
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
	for i, index := range request.Indexes[0] {
		p := &types.ProofEventsParam{
			Index:     index,
			BlockHash: request.Hash,
			Events:    request.Events[0][i],
		}

		proofs, err := icp.chainProvider.client.GetProofForEvents(p)
		if err != nil {
			icp.log.Info(fmt.Sprintf("error: %v\n", err))
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
