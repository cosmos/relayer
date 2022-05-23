package cosmos

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	cosmosClient "github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	tmclient "github.com/cosmos/ibc-go/v3/modules/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/v2/relayer/ibc"
	"github.com/cosmos/relayer/v2/relayer/paths"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	abci "github.com/tendermint/tendermint/abci/types"
	provtypes "github.com/tendermint/tendermint/light/provider"
	prov "github.com/tendermint/tendermint/light/provider/http"
	tmtypes "github.com/tendermint/tendermint/types"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	minQueryLoopDuration       = 1 * time.Second
	heightQueryTimeout         = 5 * time.Second
	initialBlockHistoryToQuery = 100
	validatorSetsToCache       = 5
)

type CosmosChainProcessor struct {
	ctx context.Context
	log *zap.Logger

	pathProcessors []*paths.PathProcessor
	ChainProvider  *cosmos.CosmosProvider

	// sdk context
	cc *cosmosClient.Context
	// for fetching light blocks from tendermint
	lightProvider provtypes.Provider

	// number after the - in chainID
	revisionNumber uint64

	// the rest of the properties are concurrently accessed

	// is the query loop up to date with the latest blocks of the chain
	inSync uint8

	// latest light block header
	latest     *tmclient.Header
	latestLock sync.Mutex

	// latest height and timestamp
	latestBlock     ibc.LatestBlock
	latestBlockLock sync.Mutex

	// used for caching recent validator sets
	validatorSets     map[int64]*tmtypes.ValidatorSet
	validatorSetsLock sync.Mutex

	// light client trust height, needed by other chain when assembling MsgUpdateClient
	clientHeight     map[string]clienttypes.Height
	clientHeightLock sync.Mutex

	// keeps track of open channels
	channelOpenState     map[ibc.ChannelKey]bool
	channelOpenStateLock sync.Mutex
}

type TransactionMessage struct {
	Action      string
	PacketInfo  *ibc.PacketInfo
	ChannelInfo *ibc.ChannelInfo
	ClientInfo  *ibc.ClientInfo
}

func NewCosmosChainProcessor(ctx context.Context, log *zap.Logger, rpcAddress string, provider provider.ChainProvider, pathProcessors ...*paths.PathProcessor) (*CosmosChainProcessor, error) {
	chainID := provider.ChainId()
	cc, err := getCosmosClient(rpcAddress, chainID)
	if err != nil {
		return nil, fmt.Errorf("error getting cosmos client: %w", err)
	}
	lightProvider, err := prov.New(chainID, rpcAddress)
	if err != nil {
		return nil, fmt.Errorf("error getting light provider: %w", err)
	}
	revisionSplit := strings.Split(chainID, "-")
	revisionNumberString := revisionSplit[len(revisionSplit)-1]
	revisionNumber, err := strconv.ParseUint(revisionNumberString, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error getting revision number from chain ID %s: %v", chainID, err)
	}
	cosmosProvider, ok := provider.(*cosmos.CosmosProvider)
	if !ok {
		return nil, errors.New("only cosmos providers are supported for cosmos chain processors")
	}
	return &CosmosChainProcessor{
		ctx:              ctx,
		log:              log,
		ChainProvider:    cosmosProvider,
		revisionNumber:   revisionNumber,
		cc:               cc,
		pathProcessors:   pathProcessors,
		lightProvider:    lightProvider,
		validatorSets:    make(map[int64]*tmtypes.ValidatorSet),
		clientHeight:     make(map[string]clienttypes.Height),
		channelOpenState: make(map[ibc.ChannelKey]bool),
	}, nil
}

func (ccp *CosmosChainProcessor) Provider() provider.ChainProvider {
	return ccp.ChainProvider
}

func (ccp *CosmosChainProcessor) Latest() ibc.LatestBlock {
	ccp.latestBlockLock.Lock()
	defer ccp.latestBlockLock.Unlock()
	return ccp.latestBlock
}

func (ccp *CosmosChainProcessor) ClientHeight(clientID string) (clienttypes.Height, error) {
	ccp.clientHeightLock.Lock()
	defer ccp.clientHeightLock.Unlock()
	clientHeight, ok := ccp.clientHeight[clientID]
	if !ok {
		// query for it if it's not in the cache
		clientState, err := ccp.ChainProvider.QueryClientState(ccp.ctx, ccp.latestHeight(), clientID)
		if err != nil {
			return clienttypes.Height{}, fmt.Errorf("{%s} height could not be found for client: %s\n", ccp.ChainProvider.ChainId(), clientID)
		}
		return clientState.GetLatestHeight().(clienttypes.Height), nil
	}
	return clientHeight, nil
}

func (ccp *CosmosChainProcessor) LatestHeaderWithTrustedVals(height uint64) (ibcexported.Header, error) {
	ccp.latestLock.Lock()
	h := ccp.latest
	ccp.latestLock.Unlock()
	if h == nil {
		return nil, fmt.Errorf("{%s} latest header is nil for height: %d\n", ccp.ChainProvider.ChainId(), height)
	}
	ccp.validatorSetsLock.Lock()
	defer ccp.validatorSetsLock.Unlock()
	trustedValidators, ok := ccp.validatorSets[int64(height)]
	if !ok {
		// query for it if it's not in the cache
		lightBlock, err := ccp.lightProvider.LightBlock(ccp.ctx, int64(height))
		if err != nil {
			return nil, fmt.Errorf("{%s} trusted validators could not be added for height: %d, %v\n", ccp.ChainProvider.ChainId(), height, err)
		}
		trustedValidators = lightBlock.ValidatorSet
	}
	trustedValidatorsProto, err := trustedValidators.ToProto()
	if err != nil {
		return nil, fmt.Errorf("{%s} trusted validators could not be transformed to proto for height: %d, %v\n", ccp.ChainProvider.ChainId(), height, err)
	}
	h.TrustedValidators = trustedValidatorsProto
	return h, nil
}

func (ccp *CosmosChainProcessor) InSync() bool {
	return ccp.inSync == 1
}

func (ccp *CosmosChainProcessor) CacheValidatorSet(height int64, validatorSet *tmtypes.ValidatorSet) {
	ccp.validatorSetsLock.Lock()
	defer ccp.validatorSetsLock.Unlock()
	keys := make([]int64, 0)
	for k := range ccp.validatorSets {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	toRemove := len(keys) - validatorSetsToCache - 1
	if toRemove > 0 {
		for i := 0; i < toRemove; i++ {
			delete(ccp.validatorSets, keys[i])
		}
	}
	ccp.validatorSets[height] = validatorSet
}

func (ccp *CosmosChainProcessor) Start(ctx context.Context, errCh chan<- error) {
	chainID := ccp.ChainProvider.ChainId()
	latestHeight, err := QueryLatestHeight(ctx, ccp.cc)
	if err != nil {
		panic(fmt.Errorf("{%s} error querying latest height: %w", chainID, err))
	}

	// this will make initial QueryLoop iteration look back initialBlockHistoryToQuery blocks in history
	latestQueriedBlock := latestHeight - initialBlockHistoryToQuery

	if latestQueriedBlock < 0 {
		latestQueriedBlock = 0
	}

	if err := ccp.BootstrapChannels(); err != nil {
		panic(fmt.Errorf("{%s} error initializing channel state: %w", chainID, err))
	}

	ccp.log.Info("entering main query loop", zap.String("chainID", chainID))

QueryLoop:
	for {
		if ccp.ctx.Err() != nil {
			return
		}
		cycleTimeStart := time.Now()
		doneWithThisCycle := func() {
			queryDuration := time.Since(cycleTimeStart)
			if queryDuration < minQueryLoopDuration {
				time.Sleep(minQueryLoopDuration - queryDuration)
			}
		}
		latestHeightQueryCtx, latestHeightQueryCtxCancel := context.WithTimeout(ctx, heightQueryTimeout)
		latestHeight, err := QueryLatestHeight(latestHeightQueryCtx, ccp.cc)
		latestHeightQueryCtxCancel()
		if err != nil {
			errCh <- fmt.Errorf("{%s} error querying latest height: %w", chainID, err)
			doneWithThisCycle()
			continue
		}

		// until in sync, determine if our latest queries are up to date with the current chain height
		// this will cause the PathProcessors to start processing the backlog of message state
		firstTimeInSync := false
		if ccp.inSync != 1 {
			if (latestHeight - latestQueriedBlock) < 2 {
				ccp.inSync = 1
				firstTimeInSync = true
				ccp.log.Info("chain is in sync", zap.String("chainID", chainID))
			} else {
				ccp.log.Warn("chain is not yet in sync",
					zap.String("chainID", chainID),
					zap.Int64("latestQueriedBlock", latestQueriedBlock),
					zap.Int64("latestHeight", latestHeight),
				)
			}
		}

		ccp.log.Debug("queried latest height",
			zap.String("chainID", chainID),
			zap.Int64("latestHeight", latestHeight),
		)

		for i := latestQueriedBlock + 1; i <= latestHeight; i++ {
			lightBlock, err := ccp.lightProvider.LightBlock(ctx, i)
			if err != nil {
				errCh <- fmt.Errorf("{%s} error getting light block: %w\n", chainID, err)
				doneWithThisCycle()
				continue QueryLoop
			}

			if i == latestHeight {
				validatorSet, err := lightBlock.ValidatorSet.ToProto()
				if err != nil {
					errCh <- fmt.Errorf("{%s} error converting validator set to proto: %w\n", chainID, err)
				} else {
					ccp.latestLock.Lock()
					ccp.latest = &tmclient.Header{
						SignedHeader: lightBlock.SignedHeader.ToProto(),
						ValidatorSet: validatorSet,
					}
					ccp.latestLock.Unlock()
				}
				ccp.latestBlockLock.Lock()
				ccp.latestBlock = ibc.LatestBlock{
					Height: uint64(latestHeight),
					Time:   lightBlock.Time,
				}
				ccp.latestBlockLock.Unlock()
			}

			ccp.CacheValidatorSet(i, lightBlock.ValidatorSet)

			blockRes, err := ccp.cc.Client.BlockResults(ctx, &i)
			if err != nil {
				errCh <- fmt.Errorf("{%s} error getting block results: %w\n", chainID, err)
				doneWithThisCycle()
				continue QueryLoop
			}

			foundMessages := make(map[ibc.ChannelKey]map[string]map[uint64]provider.RelayerMessage)
			for _, tx := range blockRes.TxsResults {
				if tx.Code != 0 {
					// tx was not successful
					continue
				}
				messages := ccp.processTransaction(tx, errCh)
				for _, m := range messages {
					if handler, ok := messageHandlers[m.Action]; ok {
						handler(MsgHandlerParams{
							CCP:           ccp,
							Height:        i,
							PacketInfo:    m.PacketInfo,
							ChannelInfo:   m.ChannelInfo,
							ClientInfo:    m.ClientInfo,
							FoundMessages: foundMessages,
						})
					}
				}
			}

			ccp.channelOpenStateLock.Lock()
			for channelKey, messages := range foundMessages {
				// do not relay on closed channels
				if !ccp.channelOpenState[channelKey] {
					continue
				}
				for _, pp := range ccp.pathProcessors {
					pp.HandleNewMessages(chainID, channelKey, messages)
				}
			}
			ccp.channelOpenStateLock.Unlock()
			latestQueriedBlock = i
		}

		if firstTimeInSync {
			for _, pp := range ccp.pathProcessors {
				go pp.ScheduleNextProcess(true)
			}
		}

		doneWithThisCycle()
	}
}

func (ccp *CosmosChainProcessor) IsRelevantChannel(channelKey ibc.ChannelKey) bool {
	for _, pathProcessor := range ccp.pathProcessors {
		if pathProcessor.IsRelevantChannel(ccp.ChainProvider.ChainId(), channelKey) {
			return true
		}
	}
	return false
}

func (ccp *CosmosChainProcessor) IsRelevantClient(clientID string) bool {
	for _, pathProcessor := range ccp.pathProcessors {
		if pathProcessor.IsRelevantClient(clientID) {
			return true
		}
	}
	return false
}

func (ccp *CosmosChainProcessor) BootstrapChannels() error {
	chainID := ccp.ChainProvider.ChainId()
	uniqueConnectionIDs := []string{}
	for _, pp := range ccp.pathProcessors {
		connectionID, err := pp.GetRelevantConnectionID(chainID)
		if err != nil {
			return fmt.Errorf("{%s} error getting connection ID: %w\n", chainID, err)
		}
		found := false
		for _, cID := range uniqueConnectionIDs {
			if connectionID == cID {
				found = true
				break
			}
		}
		if !found {
			uniqueConnectionIDs = append(uniqueConnectionIDs, connectionID)
		}
	}

	var eg errgroup.Group
	for _, connectionID := range uniqueConnectionIDs {
		connectionID := connectionID
		eg.Go(func() error {
			return ccp.UpdateChannelState(connectionID)
		})
	}
	return eg.Wait()
}

func (ccp *CosmosChainProcessor) UpdateChannelState(connectionID string) error {
	channels, err := ccp.ChainProvider.QueryConnectionChannels(ccp.ctx, ccp.latestHeight(), connectionID)
	if err != nil {
		return err
	}
	ccp.channelOpenStateLock.Lock()
	defer ccp.channelOpenStateLock.Unlock()
	for _, channel := range channels {
		ccp.channelOpenState[ibc.ChannelKey{
			ChannelID:             channel.ChannelId,
			PortID:                channel.PortId,
			CounterpartyChannelID: channel.Counterparty.ChannelId,
			CounterpartyPortID:    channel.Counterparty.PortId,
		}] = channel.State == chantypes.OPEN
	}
	return nil
}

func (ccp *CosmosChainProcessor) processTransaction(tx *abci.ResponseDeliverTx, errCh chan<- error) []TransactionMessage {
	messages := []TransactionMessage{}
	parsedLogs, err := sdk.ParseABCILogs(tx.Log)
	if err != nil {
		return messages
	}
	for _, messageLog := range parsedLogs {
		var packetInfo *ibc.PacketInfo
		var channelInfo *ibc.ChannelInfo
		var clientInfo *ibc.ClientInfo
		var action string
		for _, event := range messageLog.Events {
			switch event.Type {
			case "create_client", "update_client", "upgrade_client", "submit_misbehaviour":
				clientInfo = &ibc.ClientInfo{}
				for _, attr := range event.Attributes {
					switch attr.Key {
					case "client_id":
						clientInfo.ClientID = attr.Value
					case "consensus_height":
						revisionSplit := strings.Split(attr.Value, "-")
						if len(revisionSplit) != 2 {
							continue
						}
						revisionNumberString := revisionSplit[0]
						revisionNumber, err := strconv.ParseUint(revisionNumberString, 10, 64)
						if err != nil {
							errCh <- fmt.Errorf("error parsing revision number: %w\n", err)
							continue
						}
						revisionHeightString := revisionSplit[1]
						revisionHeight, err := strconv.ParseUint(revisionHeightString, 10, 64)
						if err != nil {
							errCh <- fmt.Errorf("error parsing revision height: %w\n", err)
							continue
						}
						clientInfo.ConsensusHeight = clienttypes.Height{
							RevisionNumber: revisionNumber,
							RevisionHeight: revisionHeight,
						}
					}
				}
			case "send_packet", "recv_packet",
				"acknowledge_packet", "timeout_packet", "write_acknowledgement":
				packetInfo = &ibc.PacketInfo{}
				for _, attr := range event.Attributes {
					var err error
					switch attr.Key {
					case "packet_sequence":
						packetInfo.Sequence, err = strconv.ParseUint(attr.Value, 10, 64)
						if err != nil {
							continue
						}
					case "packet_timeout_timestamp":
						packetInfo.TimeoutTimestamp, err = strconv.ParseUint(attr.Value, 10, 64)
						if err != nil {
							continue
						}
					case "packet_data":
						packetInfo.Data = []byte(attr.Value)
					case "packet_data_hex":
						data, err := hex.DecodeString(attr.Value)
						if err == nil {
							packetInfo.Data = data
						}
					case "packet_ack":
						packetInfo.Ack = []byte(attr.Value)
					case "packet_ack_hex":
						data, err := hex.DecodeString(attr.Value)
						if err == nil {
							packetInfo.Ack = data
						}
					case "packet_timeout_height":
						timeoutSplit := strings.Split(attr.Value, "-")
						if len(timeoutSplit) != 2 {
							continue
						}
						revisionNumber, err := strconv.ParseUint(timeoutSplit[0], 10, 64)
						if err != nil {
							continue
						}
						revisionHeight, err := strconv.ParseUint(timeoutSplit[1], 10, 64)
						if err != nil {
							continue
						}
						packetInfo.TimeoutHeight = clienttypes.Height{
							RevisionNumber: revisionNumber,
							RevisionHeight: revisionHeight,
						}
					case "packet_src_port":
						packetInfo.SourcePortID = attr.Value
					case "packet_src_channel":
						packetInfo.SourceChannelID = attr.Value
					case "packet_dst_port":
						packetInfo.DestinationPortID = attr.Value
					case "packet_dst_channel":
						packetInfo.DestinationChannelID = attr.Value
					case "packet_channel_ordering":
						packetInfo.ChannelOrdering = attr.Value
					case "packet_connection":
						packetInfo.ConnectionID = attr.Value
					}
				}
			case "message":
				for _, attr := range event.Attributes {
					switch attr.Key {
					case "action":
						action = attr.Value
					}
				}
			case "connection_open_init", "connection_open_try", "connection_open_ack",
				"connection_open_confirm", "channel_open_init", "channel_open_try",
				"channel_open_ack", "channel_open_confirm", "channel_close_init":
				channelInfo = &ibc.ChannelInfo{}
				for _, attr := range event.Attributes {
					switch attr.Key {
					case "port_id":
						channelInfo.PortID = attr.Value
					case "connection_id":
						channelInfo.ConnectionID = attr.Value
					case "client_id":
						channelInfo.ClientID = attr.Value
					case "channel_id":
						channelInfo.ChannelID = attr.Value
					case "counterparty_port_id":
						channelInfo.CounterpartyPortID = attr.Value
					case "counterparty_connection_id":
						channelInfo.CounterpartyConnectionID = attr.Value
					case "counterparty_client_id":
						channelInfo.CounterpartyClientID = attr.Value
					case "counterparty_channel_id":
						channelInfo.CounterpartyChannelID = attr.Value
					}
				}
			}
		}
		messages = append(messages, TransactionMessage{
			Action:      action,
			PacketInfo:  packetInfo,
			ChannelInfo: channelInfo,
			ClientInfo:  clientInfo,
		})
	}

	return messages
}
