package icon

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/chains/icon/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/gorilla/websocket"
	"github.com/icon-project/goloop/common"
	"github.com/icon-project/goloop/common/codec"
	"github.com/pkg/errors"
)

const (
	queryTimeout                = 5 * time.Second
	latestHeightQueryRetryDelay = 1 * time.Second
	queryRetries                = 5
)

const (
	notProcessed = "not-processed"
	processed    = "processed"
)

type IconChainProcessor struct {
	log           *zap.Logger
	chainProvider *IconProvider

	pathProcessors processor.PathProcessors

	inSync    bool
	firstTime bool

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
	if ok {
		if clientInfo.consensusHeight.LT(existingClientInfo.ConsensusHeight) {
			// height is less than latest, so no-op
			return
		}
	}

	clientState := clientInfo.ClientState()
	l[clientInfo.clientID] = clientState
}

type btpBlockResponse struct {
	Height      int64
	Header      IconIBCHeader
	EventLogs   []types.EventLog
	IsProcessed string
}
type btpBlockRequest struct {
	height   int64
	hash     types.HexBytes
	indexes  [][]types.HexInt
	events   [][][]types.HexInt
	err      error
	retry    int
	response *btpBlockResponse
}

// ************************************************** For persistence **************************************************
type queryCyclePersistence struct {
	latestHeight   int64
	latestHeightMu sync.Mutex

	lastQueriedHeight     int64
	latestQueriedHeightMu sync.Mutex

	minQueryLoopDuration time.Duration
}

func (icp *IconChainProcessor) Run(ctx context.Context, initialBlockHistory uint64) error {
	persistence := queryCyclePersistence{
		minQueryLoopDuration: time.Second,
	}

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
	err := icp.monitoring(ctx, &persistence)
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

		icp.log.Info("found connection",
			zap.String("ClientId ", c.ClientId),
			zap.String("ConnectionID ", c.Id),
		)
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

		icp.log.Info("Found channel",
			zap.String("channelID", ch.ChannelId),
			zap.String("Port id ", ch.PortId))
		zap.String("Counterparty Channel Id ", ch.Counterparty.ChannelId)
		zap.String("Counterparty Port Id", ch.Counterparty.PortId)
	}

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

	errCh := make(chan error)                                            // error channel
	reconnectCh := make(chan struct{}, 1)                                // reconnect channel
	btpBlockNotifCh := make(chan *types.BlockNotification, 10)           // block notification channel
	btpBlockRespCh := make(chan *btpBlockResponse, cap(btpBlockNotifCh)) // block result channel

	reconnect := func() {
		select {
		case reconnectCh <- struct{}{}:
		default:
		}
		for len(btpBlockRespCh) > 0 || len(btpBlockNotifCh) > 0 {
			select {
			case <-btpBlockRespCh: // clear block result channel
			case <-btpBlockNotifCh: // clear block notification channel
			}
		}
	}

	var err error
	processedheight := int64(icp.chainProvider.lastBTPBlockHeight)
	if processedheight == 0 {
		processedheight, err = icp.chainProvider.QueryLatestHeight(ctx)
		if err != nil {
			return err
		}
	}

	// subscribe to monitor block
	ctxMonitorBlock, cancelMonitorBlock := context.WithCancel(ctx)
	reconnect()

	ibcHeaderCache := make(processor.IBCHeaderCache)

	icp.firstTime = true

	blockReq := &types.BlockRequest{
		Height:       types.NewHexInt(int64(icp.chainProvider.PCfg.BTPHeight)),
		EventFilters: GetMonitorEventFilters(icp.chainProvider.PCfg.IbcHandlerAddress),
	}

loop:
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errCh:
			return err

		case <-reconnectCh:
			cancelMonitorBlock()
			ctxMonitorBlock, cancelMonitorBlock = context.WithCancel(ctx)

			go func(ctx context.Context, cancel context.CancelFunc) {
				blockReq.Height = types.NewHexInt(processedheight)
				icp.log.Debug("Querying Height", zap.Int64("height", processedheight))
				err := icp.chainProvider.client.MonitorBlock(ctx, blockReq, func(conn *websocket.Conn, v *types.BlockNotification) error {
					if !errors.Is(ctx.Err(), context.Canceled) {
						btpBlockNotifCh <- v
					}
					return nil
				}, func(conn *websocket.Conn) {
				}, func(conn *websocket.Conn, err error) {})
				if err != nil {
					if errors.Is(err, context.Canceled) {
						return
					}
					time.Sleep(time.Second * 5)
					reconnect()
					icp.log.Warn("Error occured during monitor block", zap.Error(err))
				}

			}(ctxMonitorBlock, cancelMonitorBlock)
		case br := <-btpBlockRespCh:
			for ; br != nil; processedheight++ {
				icp.latestBlockMu.Lock()
				icp.latestBlock = provider.LatestBlock{
					Height: uint64(processedheight),
				}
				icp.latestBlockMu.Unlock()

				ibcMessage := parseIBCMessagesFromEventlog(icp.log, br.EventLogs, uint64(br.Height))
				ibcMessageCache := processor.NewIBCMessagesCache()
				// message handler
				for _, m := range ibcMessage {
					icp.handleMessage(ctx, *m, ibcMessageCache)
				}

				ibcHeaderCache[uint64(br.Height)] = br.Header
				icp.log.Info("Queried Latest height: ",
					zap.String("chain id ", icp.chainProvider.ChainId()),
					zap.Int64("height", br.Height))
				err := icp.handlePathProcessorUpdate(ctx, br.Header, ibcMessageCache, ibcHeaderCache)
				if err != nil {
					reconnect()
					icp.log.Warn("Reconnect: error occured during handle block response  ",
						zap.Int64("got", br.Height),
					)
					break
				}
				icp.firstTime = false
				time.Sleep(100 * time.Millisecond)
				if br = nil; len(btpBlockRespCh) > 0 {
					br = <-btpBlockRespCh
				}
			}
			// remove unprocessed blockResponses
			for len(btpBlockRespCh) > 0 {
				<-btpBlockRespCh
			}

		default:
			select {
			default:
			case bn := <-btpBlockNotifCh:
				requestCh := make(chan *btpBlockRequest, cap(btpBlockNotifCh))
				for i := int64(0); bn != nil; i++ {
					height, err := bn.Height.Value()

					// icp.log.Info("for loop when receiving blockNotification",
					// 	zap.Int64("height", height),
					// 	zap.Int64("index", i),
					// 	zap.Int64("processedheight", processedheight))

					if err != nil {
						return err
					} else if height != processedheight+i {
						icp.log.Warn("Reconnect: missing block notification ",
							zap.Int64("got", height),
							zap.Int64("expected", processedheight+i),
						)
						reconnect()
						continue loop
					}

					requestCh <- &btpBlockRequest{
						height:  height,
						hash:    bn.Hash,
						indexes: bn.Indexes,
						events:  bn.Events,
						retry:   queryRetries,
					}
					if bn = nil; len(btpBlockNotifCh) > 0 && len(requestCh) < cap(requestCh) {
						bn = <-btpBlockNotifCh
					}
				}

				brs := make([]*btpBlockResponse, 0, len(requestCh))
				for request := range requestCh {
					switch {
					case request.err != nil:
						if request.retry > 0 {
							request.retry--
							request.response, request.err = nil, nil
							requestCh <- request
							continue
						}
						icp.log.Info("Request error ",
							zap.Any("height", request.height),
							zap.Error(request.err))
						brs = append(brs, nil)
						if len(brs) == cap(brs) {
							close(requestCh)
						}
					case request.response != nil:
						brs = append(brs, request.response)
						if len(brs) == cap(brs) {
							close(requestCh)
						}
					default:
						go icp.handleBTPBlockRequest(request, requestCh)

					}

				}
				// filter nil
				_brs, brs := brs, brs[:0]
				for _, v := range _brs {
					if v.IsProcessed == processed {
						brs = append(brs, v)
					}
				}

				// sort and forward notifications
				if len(brs) > 0 {
					sort.SliceStable(brs, func(i, j int) bool {
						return brs[i].Height < brs[j].Height
					})
					for i, d := range brs {
						if d.Height == processedheight+int64(i) {
							btpBlockRespCh <- d
						}
					}
				}

			}
		}
	}
}

func (icp *IconChainProcessor) handleBTPBlockRequest(
	request *btpBlockRequest, requestCh chan *btpBlockRequest) {
	defer func() {
		time.Sleep(500 * time.Millisecond)
		requestCh <- request
	}()

	if request.response == nil {
		request.response = &btpBlockResponse{
			IsProcessed: notProcessed,
			Height:      request.height,
		}
	}

	containsEventlogs := len(request.indexes) > 0 && len(request.events) > 0
	if containsEventlogs {
		blockHeader, err := icp.chainProvider.client.GetBlockHeaderByHeight(request.height)
		if err != nil {
			request.err = errors.Wrapf(request.err, "getBlockHeader: %v", err)
			return
		}

		var receiptHash types.BlockHeaderResult
		_, err = codec.RLP.UnmarshalFromBytes(blockHeader.Result, &receiptHash)
		if err != nil {
			request.err = errors.Wrapf(err, "BlockHeaderResult.UnmarshalFromBytes: %v", err)
			return

		}

		var eventlogs []types.EventLog
		for id := 0; id < len(request.indexes); id++ {
			for i, index := range request.indexes[id] {
				p := &types.ProofEventsParam{
					Index:     index,
					BlockHash: request.hash,
					Events:    request.events[id][i],
				}

				proofs, err := icp.chainProvider.client.GetProofForEvents(p)
				if err != nil {
					request.err = errors.Wrapf(err, "GetProofForEvents: %v", err)
					return

				}

				// Processing receipt index
				serializedReceipt, err := MptProve(index, proofs[0], receiptHash.ReceiptHash)
				if err != nil {
					request.err = errors.Wrapf(err, "MPTProve Receipt: %v", err)
					return

				}
				var result types.TxResult
				_, err = codec.RLP.UnmarshalFromBytes(serializedReceipt, &result)
				if err != nil {
					request.err = errors.Wrapf(err, "Unmarshal Receipt: %v", err)
					return
				}

				for j := 0; j < len(p.Events); j++ {
					serializedEventLog, err := MptProve(
						p.Events[j], proofs[j+1], common.HexBytes(result.EventLogsHash))
					if err != nil {
						request.err = errors.Wrapf(err, "event.MPTProve: %v", err)
						return
					}
					var el types.EventLog
					_, err = codec.RLP.UnmarshalFromBytes(serializedEventLog, &el)
					if err != nil {
						request.err = errors.Wrapf(err, "event.UnmarshalFromBytes: %v", err)
						return
					}
					icp.log.Info("Detected eventlog: ", zap.Int64("Height", request.height),
						zap.String("Eventlog", string(el.Indexed[0])))
					eventlogs = append(eventlogs, el)
				}

			}
		}
		request.response.EventLogs = eventlogs
	}

	validators, err := icp.chainProvider.GetProofContextByHeight(request.height)
	if err != nil {
		request.err = errors.Wrapf(err, "Failed to get proof context: %v", err)
		return
	}

	btpHeader, err := icp.chainProvider.GetBtpHeader(request.height)
	if err != nil {
		if RequiresBtpHeader(request.response.EventLogs) {
			request.err = errors.Wrapf(err, "Btp header required but not present: %v", err)
			return
		}
		if btpBlockNotPresent(err) {
			request.response.Header = NewIconIBCHeader(nil, validators, (request.height))
			request.response.IsProcessed = processed
			return
		}
		request.err = errors.Wrapf(err, "failed to get btp header: %v", err)
		return
	}
	request.response.Header = NewIconIBCHeader(btpHeader, validators, int64(btpHeader.MainHeight))
	request.response.IsProcessed = processed

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
			InSync:               true,
			ClientState:          clientState,
			ConnectionStateCache: icp.connectionStateCache.FilterForClient(clientID),
			ChannelStateCache:    icp.channelStateCache.FilterForClient(clientID, icp.channelConnections, icp.connectionClients),
			IBCHeaderCache:       ibcHeaderCache,
			IsGenesis:            icp.firstTime,
		})
	}
	return nil

}

// clientState will return the most recent client state if client messages
// have already been observed for the clientID, otherwise it will query for it.
func (icp *IconChainProcessor) clientState(ctx context.Context, clientID string) (provider.ClientState, error) {
	// if state, ok := icp.latestClientState[clientID]; ok {
	// 	return state, nil
	// }
	cs, err := icp.chainProvider.QueryClientStateWithoutProof(ctx, int64(icp.latestBlock.Height), clientID)
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
