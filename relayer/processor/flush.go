package processor

import (
	"context"
	"sync"

	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	commitmenttypes "github.com/cosmos/ibc-go/v7/modules/core/23-commitment/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func (pp *PathProcessor) flush(ctx context.Context) {
	var (
		pathEnd1Cache                    = NewIBCMessagesCache()
		pathEnd2Cache                    = NewIBCMessagesCache()
		pathEnd1CacheMu, pathEnd2CacheMu sync.Mutex
	)
	var wg sync.WaitGroup
	wg.Add(3)

	go pp.flushPackets(ctx, pathEnd1Cache, pathEnd2Cache, &pathEnd1CacheMu, &pathEnd2CacheMu, &wg)
	go pp.flushChannels(ctx, pathEnd1Cache, pathEnd2Cache, &pathEnd1CacheMu, &pathEnd2CacheMu, &wg)
	go pp.flushConnections(ctx, pathEnd1Cache, pathEnd2Cache, &pathEnd1CacheMu, &pathEnd2CacheMu, &wg)

	wg.Wait()

	pp.pathEnd1.mergeMessageCache(pathEnd1Cache, pp.pathEnd2.info.ChainID, pp.pathEnd2.inSync)
	pp.pathEnd2.mergeMessageCache(pathEnd2Cache, pp.pathEnd1.info.ChainID, pp.pathEnd1.inSync)

}

// flushPackets runs queries to relay any pending messages which may have been
// in blocks before the height that the chain processors started querying.
func (pp *PathProcessor) flushPackets(
	ctx context.Context,
	pathEnd1Cache, pathEnd2Cache IBCMessagesCache,
	pathEnd1CacheMu, pathEnd2CacheMu sync.Locker,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	var (
		commitments1                   = make(map[ChannelKey][]uint64)
		commitments2                   = make(map[ChannelKey][]uint64)
		commitments1Mu, commitments2Mu sync.Mutex
	)

	// Query remaining packet commitments on both chains
	var eg errgroup.Group
	for k, open := range pp.pathEnd1.channelStateCache {
		if !open {
			continue
		}
		eg.Go(queryPacketCommitments(ctx, pp.pathEnd1, k, commitments1, &commitments1Mu))
	}
	for k, open := range pp.pathEnd2.channelStateCache {
		if !open {
			continue
		}
		eg.Go(queryPacketCommitments(ctx, pp.pathEnd2, k, commitments2, &commitments2Mu))
	}

	if err := eg.Wait(); err != nil {
		pp.log.Error("Failed to query packet commitments", zap.Error(err))
	}

	// From remaining packet commitments, determine if:
	// 1. Packet commitment is on source, but MsgRecvPacket has not yet been relayed to destination
	// 2. Packet commitment is on source, and MsgRecvPacket has been relayed to destination, but MsgAcknowledgement has not been written to source to clear the packet commitment.
	// Based on above conditions, enqueue MsgRecvPacket and MsgAcknowledgement messages
	for k, seqs := range commitments1 {
		eg.Go(queuePendingRecvAndAcks(ctx, pp.pathEnd1, pp.pathEnd2, k, seqs, pathEnd1Cache.PacketFlow, pathEnd2Cache.PacketFlow, pathEnd1CacheMu, pathEnd2CacheMu))
	}

	for k, seqs := range commitments2 {
		eg.Go(queuePendingRecvAndAcks(ctx, pp.pathEnd2, pp.pathEnd1, k, seqs, pathEnd2Cache.PacketFlow, pathEnd1Cache.PacketFlow, pathEnd2CacheMu, pathEnd1CacheMu))
	}

	if err := eg.Wait(); err != nil {
		pp.log.Error("Failed to enqueue pending messages for flush", zap.Error(err))
	}
}

// shouldTerminateForFlushComplete will determine if the relayer should exit
// when FlushLifecycle is used. It will exit when all of the message caches are cleared.
func (pp *PathProcessor) shouldTerminateForFlushComplete() bool {
	if _, ok := pp.messageLifecycle.(*FlushLifecycle); !ok {
		return false
	}
	for k, packetMessagesCache := range pp.pathEnd1.messageCache.PacketFlow {
		if open, ok := pp.pathEnd1.channelStateCache[k]; !ok || !open {
			continue
		}
		for _, c := range packetMessagesCache {
			if len(c) > 0 {
				return false
			}
		}
	}
	for _, c := range pp.pathEnd1.messageCache.ChannelHandshake {
		for k := range pp.pathEnd1.channelStateCache {
			if _, ok := c[k]; ok {
				return false
			}
		}
	}
	for _, c := range pp.pathEnd1.messageCache.ConnectionHandshake {
		for k := range pp.pathEnd1.connectionStateCache {
			if _, ok := c[k]; ok {
				return false
			}
		}
	}
	for k, packetMessagesCache := range pp.pathEnd2.messageCache.PacketFlow {
		if open, ok := pp.pathEnd1.channelStateCache[k]; !ok || !open {
			continue
		}
		for _, c := range packetMessagesCache {
			if len(c) > 0 {
				return false
			}
		}
	}
	for _, c := range pp.pathEnd2.messageCache.ChannelHandshake {
		for k := range pp.pathEnd1.channelStateCache {
			if _, ok := c[k]; ok {
				return false
			}
		}
	}
	for _, c := range pp.pathEnd2.messageCache.ConnectionHandshake {
		for k := range pp.pathEnd1.connectionStateCache {
			if _, ok := c[k]; ok {
				return false
			}
		}
	}
	pp.log.Info("Found termination condition for flush, all caches cleared")
	return true
}

type ChannelHandshakeState struct {
	StateSrc  chantypes.State
	StateDst  chantypes.State
	Order     chantypes.Order
	Version   string
	SrcConnID string
	DstConnID string
}

// flushChannels runs queries to relay any pending channel handshake messages which may have been
// in blocks before the height that the chain processors started querying.
func (pp *PathProcessor) flushChannels(
	ctx context.Context,
	pathEnd1Cache, pathEnd2Cache IBCMessagesCache,
	pathEnd1CacheMu, pathEnd2CacheMu sync.Locker,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	var (
		pathEnd1Chans, pathEnd2Chans []*chantypes.IdentifiedChannel
		eg                           errgroup.Group
	)
	eg.Go(func() (err error) {
		pathEnd1Chans, err = pp.pathEnd1.chainProvider.QueryChannels(ctx)
		return err
	})
	eg.Go(func() (err error) {
		pathEnd2Chans, err = pp.pathEnd2.chainProvider.QueryChannels(ctx)
		return err
	})
	if err := eg.Wait(); err != nil {
		pp.log.Error("Failed to query channels for channel handshake flush", zap.Error(err))
		return
	}

	srcChanKeys := make(map[ChannelKey]ChannelHandshakeState)

	for _, channel := range pathEnd1Chans {
		k := ChannelKey{
			ChannelID:             channel.ChannelId,
			PortID:                channel.PortId,
			CounterpartyChannelID: channel.Counterparty.ChannelId,
			CounterpartyPortID:    channel.Counterparty.PortId,
		}
		srcConnID := channel.ConnectionHops[0]
		if !pp.pathEnd1.isRelevantConnection(srcConnID) {
			continue
		}
		if !pp.pathEnd1.info.ShouldRelayChannel(ChainChannelKey{
			ChainID:             pp.pathEnd1.info.ChainID,
			CounterpartyChainID: pp.pathEnd2.info.ChainID,
			ChannelKey:          k,
		}) {
			continue
		}
		var dstConnID string
		for k := range pp.pathEnd1.connectionStateCache {
			if k.ConnectionID == srcConnID {
				dstConnID = k.CounterpartyConnID
			}
		}
		srcChanKeys[k] = ChannelHandshakeState{
			StateSrc:  channel.State,
			Order:     channel.Ordering,
			Version:   channel.Version,
			SrcConnID: srcConnID,
			DstConnID: dstConnID,
		}
	}

DstLoop:
	for _, channel := range pathEnd2Chans {
		k := ChannelKey{
			ChannelID:             channel.ChannelId,
			PortID:                channel.PortId,
			CounterpartyChannelID: channel.Counterparty.ChannelId,
			CounterpartyPortID:    channel.Counterparty.PortId,
		}
		dstConnID := channel.ConnectionHops[0]
		if !pp.pathEnd2.isRelevantConnection(dstConnID) {
			continue DstLoop
		}
		if !pp.pathEnd2.info.ShouldRelayChannel(ChainChannelKey{
			ChainID:             pp.pathEnd2.info.ChainID,
			CounterpartyChainID: pp.pathEnd1.info.ChainID,
			ChannelKey:          k,
		}) {
			continue DstLoop
		}
		ck := k.Counterparty()
		var srcConnID string
		for k := range pp.pathEnd2.connectionStateCache {
			if k.ConnectionID == dstConnID {
				srcConnID = k.CounterpartyConnID
			}
		}
		// check if counterparty key already exists observed from src
		if s, ok := srcChanKeys[ck]; ok {
			s.StateDst = channel.State
			srcChanKeys[ck] = s
			continue DstLoop
		}
		// check if counterparty init key already exists observed from src
		msgInitKey := ck.MsgInitKey()
		if s, ok := srcChanKeys[msgInitKey]; ok {
			s.StateDst = channel.State
			srcChanKeys[ck] = s
			delete(srcChanKeys, msgInitKey)
			continue DstLoop
		}
		// check if counterparty key already exists observed from src (compared to src init key)
		for k, s := range srcChanKeys {
			if k.MsgInitKey() == ck {
				// have an init on dst with try on src
				s.StateDst = channel.State
				srcChanKeys[k] = s
				continue DstLoop
			}
		}
		// populate dst only
		srcChanKeys[ck] = ChannelHandshakeState{
			StateDst:  channel.State,
			Order:     channel.Ordering,
			Version:   channel.Version,
			SrcConnID: srcConnID,
			DstConnID: dstConnID,
		}
	}

	for k, s := range srcChanKeys {
		if s.StateSrc == chantypes.OPEN && s.StateDst == chantypes.OPEN {
			// already open on both sides. nothing needed.
			continue
		}

		if s.StateSrc == chantypes.CLOSED || s.StateDst == chantypes.CLOSED {
			// don't handle channel closure here.
			// Channel closure is a manual operation for now with rly tx channel-close.
			continue
		}

		pp.log.Info("Found channel that needs to complete handshake",
			zap.String("src_channel_id", k.ChannelID),
			zap.String("src_port_id", k.PortID),
			zap.String("dst_channel_id", k.CounterpartyChannelID),
			zap.String("dst_port_id", k.CounterpartyPortID),
			zap.String("src_state", s.StateSrc.String()),
			zap.String("dst_state", s.StateDst.String()),
		)

		switch {
		case s.StateSrc == chantypes.INIT && s.StateDst == chantypes.UNINITIALIZED:
			pp.log.Info("Chan is init on src but no try yet on dst")
			populateChannelHandshake(pathEnd1Cache, pathEnd1CacheMu, chantypes.EventTypeChannelOpenInit,
				k, s.Order, s.Version, s.SrcConnID, s.DstConnID)
		case s.StateSrc == chantypes.UNINITIALIZED && s.StateDst == chantypes.INIT:
			pp.log.Info("Chan is init on dst but no try yet on src")
			populateChannelHandshake(pathEnd2Cache, pathEnd2CacheMu, chantypes.EventTypeChannelOpenInit,
				k.Counterparty(), s.Order, s.Version, s.DstConnID, s.SrcConnID)
		case s.StateSrc == chantypes.TRYOPEN && s.StateDst == chantypes.INIT:
			pp.log.Info("Chan is try on src but no ack yet on dst")
			populateChannelHandshake(pathEnd1Cache, pathEnd1CacheMu, chantypes.EventTypeChannelOpenTry,
				k, s.Order, s.Version, s.SrcConnID, s.DstConnID)
		case s.StateSrc == chantypes.INIT && s.StateDst == chantypes.TRYOPEN:
			pp.log.Info("Chan is try on dst but no ack yet on src")
			populateChannelHandshake(pathEnd2Cache, pathEnd2CacheMu, chantypes.EventTypeChannelOpenTry,
				k.Counterparty(), s.Order, s.Version, s.DstConnID, s.SrcConnID)
		case s.StateSrc == chantypes.OPEN && s.StateDst == chantypes.TRYOPEN:
			pp.log.Info("Chan is ack on src but no confirm yet on dst")
			populateChannelHandshake(pathEnd1Cache, pathEnd1CacheMu, chantypes.EventTypeChannelOpenAck,
				k, s.Order, s.Version, s.SrcConnID, s.DstConnID)
		case s.StateSrc == chantypes.TRYOPEN && s.StateDst == chantypes.OPEN:
			pp.log.Info("Chan is ack on dst but no confirm yet on src")
			populateChannelHandshake(pathEnd2Cache, pathEnd2CacheMu, chantypes.EventTypeChannelOpenAck,
				k.Counterparty(), s.Order, s.Version, s.DstConnID, s.SrcConnID)
		}
	}
}

func populateChannelHandshake(
	c IBCMessagesCache,
	mu sync.Locker,
	eventType string,
	k ChannelKey,
	order chantypes.Order,
	version,
	connID, counterpartyConnID string,
) {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := c.ChannelHandshake[eventType]; !ok {
		c.ChannelHandshake[eventType] = make(ChannelMessageCache)
	}
	c.ChannelHandshake[eventType][k] = provider.ChannelInfo{
		PortID:                k.PortID,
		ChannelID:             k.ChannelID,
		CounterpartyPortID:    k.CounterpartyPortID,
		CounterpartyChannelID: k.CounterpartyChannelID,
		Order:                 order,
		Version:               version,
		ConnID:                connID,
		CounterpartyConnID:    counterpartyConnID,
	}
}

type ConnectionHandshakeState struct {
	StateSrc  conntypes.State
	StateDst  conntypes.State
	PrefixSrc commitmenttypes.MerklePrefix
	PrefixDst commitmenttypes.MerklePrefix
}

// flushChannels runs queries to relay any pending connection handshake messages which may have been
// in blocks before the height that the chain processors started querying.
func (pp *PathProcessor) flushConnections(
	ctx context.Context,
	pathEnd1Cache, pathEnd2Cache IBCMessagesCache,
	pathEnd1CacheMu, pathEnd2CacheMu sync.Locker,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	var (
		pathEnd1Conns, pathEnd2Conns []*conntypes.IdentifiedConnection
		eg                           errgroup.Group
	)
	eg.Go(func() (err error) {
		pathEnd1Conns, err = pp.pathEnd1.chainProvider.QueryConnections(ctx)
		return err
	})
	eg.Go(func() (err error) {
		pathEnd2Conns, err = pp.pathEnd2.chainProvider.QueryConnections(ctx)
		return err
	})
	if err := eg.Wait(); err != nil {
		pp.log.Error("Failed to query connections for connection handshake flush", zap.Error(err))
		return
	}

	srcConnKeys := make(map[ConnectionKey]ConnectionHandshakeState)

	for _, conn := range pathEnd1Conns {
		if conn.ClientId != pp.pathEnd1.info.ClientID {
			continue
		}

		k := ConnectionKey{
			ConnectionID:         conn.Id,
			ClientID:             conn.ClientId,
			CounterpartyConnID:   conn.Counterparty.ConnectionId,
			CounterpartyClientID: conn.Counterparty.ClientId,
		}

		srcConnKeys[k] = ConnectionHandshakeState{
			StateSrc:  conn.State,
			PrefixDst: conn.Counterparty.Prefix,
		}
	}

DstLoop:
	for _, conn := range pathEnd2Conns {
		if conn.ClientId != pp.pathEnd2.info.ClientID {
			continue
		}

		k := ConnectionKey{
			ConnectionID:         conn.Id,
			ClientID:             conn.ClientId,
			CounterpartyConnID:   conn.Counterparty.ConnectionId,
			CounterpartyClientID: conn.Counterparty.ClientId,
		}

		ck := k.Counterparty()

		// check if counterparty key already exists observed from src
		if s, ok := srcConnKeys[ck]; ok {
			s.StateDst = conn.State
			s.PrefixSrc = conn.Counterparty.Prefix
			srcConnKeys[ck] = s
			continue DstLoop
		}
		// check if counterparty init key already exists observed from src
		msgInitKey := ck.MsgInitKey()
		if s, ok := srcConnKeys[msgInitKey]; ok {
			s.StateDst = conn.State
			s.PrefixSrc = conn.Counterparty.Prefix
			srcConnKeys[ck] = s
			delete(srcConnKeys, msgInitKey)
			continue DstLoop
		}
		// check if counterparty key already exists observed from src (compared to src init key)
		for k, s := range srcConnKeys {
			if k.MsgInitKey() == ck {
				// have an init on dst with try on src
				s.StateDst = conn.State
				s.PrefixSrc = conn.Counterparty.Prefix
				srcConnKeys[k] = s
				continue DstLoop
			}
		}
		// populate dst only
		srcConnKeys[ck] = ConnectionHandshakeState{
			StateDst:  conn.State,
			PrefixSrc: conn.Counterparty.Prefix,
		}
	}

	for k, s := range srcConnKeys {
		if s.StateSrc == conntypes.OPEN && s.StateDst == conntypes.OPEN {
			// already open on both sides. nothing needed.
			continue
		}

		pp.log.Info("Found connection that needs to complete handshake",
			zap.String("src_connection_id", k.ConnectionID),
			zap.String("src_client_id", k.ClientID),
			zap.String("dst_connection_id", k.CounterpartyConnID),
			zap.String("dst_client_id", k.CounterpartyClientID),
			zap.String("src_state", s.StateSrc.String()),
			zap.String("dst_state", s.StateDst.String()),
		)

		switch {
		case s.StateSrc == conntypes.INIT && s.StateDst == conntypes.UNINITIALIZED:
			pp.log.Info("Conn is init on src but no try yet on dst")
			populateConnectionHandshake(pathEnd1Cache, pathEnd1CacheMu, conntypes.EventTypeConnectionOpenInit,
				k, s.PrefixDst)
		case s.StateSrc == conntypes.UNINITIALIZED && s.StateDst == conntypes.INIT:
			pp.log.Info("Conn is init on dst but no try yet on src")
			populateConnectionHandshake(pathEnd2Cache, pathEnd2CacheMu, conntypes.EventTypeConnectionOpenInit,
				k.Counterparty(), s.PrefixSrc)
		case s.StateSrc == conntypes.TRYOPEN && s.StateDst == conntypes.INIT:
			pp.log.Info("Conn is try on src but no ack yet on dst")
			populateConnectionHandshake(pathEnd1Cache, pathEnd1CacheMu, conntypes.EventTypeConnectionOpenTry,
				k, s.PrefixDst)
		case s.StateSrc == conntypes.INIT && s.StateDst == conntypes.TRYOPEN:
			pp.log.Info("Conn is try on dst but no ack yet on src")
			populateConnectionHandshake(pathEnd2Cache, pathEnd2CacheMu, conntypes.EventTypeConnectionOpenTry,
				k.Counterparty(), s.PrefixSrc)
		case s.StateSrc == conntypes.OPEN && s.StateDst == conntypes.TRYOPEN:
			pp.log.Info("Conn is ack on src but no confirm yet on dst")
			populateConnectionHandshake(pathEnd1Cache, pathEnd1CacheMu, conntypes.EventTypeConnectionOpenAck,
				k, s.PrefixDst)
		case s.StateSrc == conntypes.TRYOPEN && s.StateDst == conntypes.OPEN:
			pp.log.Info("Conn is ack on dst but no confirm yet on src")
			populateConnectionHandshake(pathEnd2Cache, pathEnd2CacheMu, conntypes.EventTypeConnectionOpenAck,
				k.Counterparty(), s.PrefixSrc)
		}
	}
}

func populateConnectionHandshake(
	c IBCMessagesCache,
	mu sync.Locker,
	eventType string,
	k ConnectionKey,
	counterpartyCommitmentPrefix commitmenttypes.MerklePrefix,
) {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := c.ConnectionHandshake[eventType]; !ok {
		c.ConnectionHandshake[eventType] = make(ConnectionMessageCache)
	}
	c.ConnectionHandshake[eventType][k] = provider.ConnectionInfo{
		ConnID:                       k.ConnectionID,
		ClientID:                     k.ClientID,
		CounterpartyConnID:           k.CounterpartyConnID,
		CounterpartyClientID:         k.CounterpartyClientID,
		CounterpartyCommitmentPrefix: counterpartyCommitmentPrefix,
	}
}
