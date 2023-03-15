package icon

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"cosmossdk.io/errors"
	"github.com/gorilla/websocket"
	"github.com/icon-project/goloop/common/codec"
	"github.com/icon-project/ibc-relayer/relayer/chains/icon/types"
	"github.com/icon-project/ibc-relayer/relayer/provider"
	"go.uber.org/zap"
)

const (
	ENDPOINT              = "https://ctz.solidwallet.io/api/v3"
	WSS_ENDPOINT          = "wss://ctz.solidwallet.io/api/v3/icon_dex/event"
	SEND_PACKET_SIGNATURE = "SendPacket(bytes)"
	CONTRACT_ADDRESS      = "cxf17ab3c6daa47e915eab4292fbf3094067e9a026"
)

func (ip *IconProvider) FetchEvent(height int) {

	blockReq := &types.BlockRequest{
		EventFilters: []*types.EventFilter{{
			Addr:      types.Address(CONTRACT_ADDRESS),
			Signature: SEND_PACKET_SIGNATURE,
			// Indexed:   []*string{&dstAddr},
		}},
		Height: types.NewHexInt(int64(height)),
	}
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	l := zap.Logger{}

	client := NewClient(WSS_ENDPOINT, &l)
	h, s := height, 0

	go func() {
		err := client.MonitorBlock(ctx, blockReq, func(conn *websocket.Conn, v *types.BlockNotification) error {
			_h, _ := v.Height.Int()
			if _h != h {
				err := fmt.Errorf("invalid block height: %d, expected: %d", _h, h+1)
				l.Warn(err.Error())
				return err
			}
			h++
			s++
			return nil
		},
			func(conn *websocket.Conn) {
				l.Info("Connected")
			},
			func(conn *websocket.Conn, err error) {
				l.Info("Disconnected")
				_ = conn.Close()
			})
		if err.Error() == "context deadline exceeded" {
			return
		}
	}()

}

type Packet struct {
	Sequence           big.Int
	SourcePort         string
	SourceChannel      string
	DestinationPort    string
	DestinationChannel string
	Data               []byte
	Height             Height
	Timestamp          big.Int
}

type Height struct {
	RevisionHeight big.Int
	RevisionNumber big.Int
}

type ibcMessage struct {
	eventType string
	info      ibcMessageInfo
}

type ibcMessageInfo interface {
	parseAttrs(log *zap.Logger, event types.EventLog)
}

type packetInfo provider.PacketInfo

func (pi *packetInfo) parseAttrs(log *zap.Logger, event types.EventLog) {
	eventType := GetEventLogSignature(event.Indexed)
	packetData := event.Indexed[1]
	packet, err := _parsePacket(packetData)
	if err != nil {
		log.Error("Error parsing packet", zap.String("value", packetData))
		return
	}
	pi.SourcePort = packet.SourcePort
	pi.SourceChannel = packet.SourceChannel
	pi.DestPort = packet.DestinationPort
	pi.DestChannel = packet.DestinationChannel
	pi.Sequence = packet.Sequence.Uint64()
	pi.Data = packet.Data
	pi.TimeoutHeight.RevisionHeight = packet.Height.RevisionHeight.Uint64()
	pi.TimeoutHeight.RevisionNumber = packet.Height.RevisionNumber.Uint64()
	pi.TimeoutTimestamp = packet.Timestamp.Uint64()
	if eventType == EventTypeAcknowledgePacket {
		pi.Ack = []byte(event.Indexed[2])
	}

}

type channelInfo provider.ChannelInfo

func (ch *channelInfo) parseAttrs(log *zap.Logger, event types.EventLog) {

	// the required data are not in Indexed. Placeholders for now

	portId := event.Indexed[1]
	channelId := event.Indexed[2]
	counterpartyPortId := event.Indexed[3]
	counterpartyChannelId := event.Indexed[4]
	version := event.Indexed[6]

	ch.PortID = portId
	ch.ChannelID = channelId
	ch.CounterpartyPortID = counterpartyPortId
	ch.CounterpartyChannelID = counterpartyChannelId
	ch.Version = version
}

type connectionInfo provider.ConnectionInfo

func (co *connectionInfo) parseAttrs(log *zap.Logger, event types.EventLog) {
	connectionId, clientId := event.Indexed[1], event.Indexed[2]
	counterpartyConnectionId, counterpartyClientId := event.Indexed[3], event.Indexed[4]

	co.ConnID = connectionId
	co.ClientID = clientId
	co.CounterpartyConnID = counterpartyConnectionId
	co.CounterpartyClientID = counterpartyClientId
}

type clientInfo struct {
	clientID        string
	consensusHeight Height
	header          []byte
}

func (cl *clientInfo) parseAttrs(log *zap.Logger, event types.EventLog) {
	clientId := event.Indexed[1]
	height := event.Indexed[3]

	revisionSplit := strings.Split(height, "-")
	if len(revisionSplit) != 2 {
		log.Error("Error parsing client consensus height",
			zap.String("client_id", cl.clientID),
			zap.String("value", height),
		)
		return
	}
	revisionNumberString := revisionSplit[0]
	revisionNumber, err := strconv.ParseUint(revisionNumberString, 10, 64)
	if err != nil {
		log.Error("Error parsing client consensus height revision number",
			zap.Error(err),
		)
		return
	}
	revisionHeightString := revisionSplit[1]
	revisionHeight, err := strconv.ParseUint(revisionHeightString, 10, 64)
	if err != nil {
		log.Error("Error parsing client consensus height revision height",
			zap.Error(err),
		)
		return
	}

	cl.consensusHeight = Height{
		RevisionHeight: *big.NewInt(int64(revisionHeight)),
		RevisionNumber: *big.NewInt(int64(revisionNumber)),
	}
	cl.clientID = clientId
}

func parseIBCMessageFromEvent(
	log *zap.Logger,
	event types.EventLog,
	chainID string,
	height uint64,
) *ibcMessage {
	eventType := event.Indexed[0]

	switch eventType {
	case EventTypeSendPacket, EventTypeRecvPacket, EventTypeAcknowledgePacket:

		pi := &packetInfo{Height: height}
		pi.parseAttrs(log, event)

		return &ibcMessage{
			eventType: eventType,
			info:      pi,
		}
	case EventTypeChannelOpenInit, EventTypeChannelOpenTry,
		EventTypeChannelOpenAck, EventTypeConnectionOpenConfirm,
		EventTypeChannelCloseInit, EventTypeChannelCloseConfirm:

		ci := &channelInfo{Height: height}
		ci.parseAttrs(log, event)

		return &ibcMessage{
			eventType: eventType,
			info:      ci,
		}
	case EventTypeConnectionOpenInit, EventTypeConnectionOpenTry,
		EventTypeConnectionOpenAck, EventTypeConnectionOpenConfirm:

		ci := &connectionInfo{Height: height}
		ci.parseAttrs(log, event)

		return &ibcMessage{
			eventType: eventType,
			info:      ci,
		}
	case EventTypeCreateClient, EventTypeUpdateClient:

		ci := &clientInfo{}
		ci.parseAttrs(log, event)

		return &ibcMessage{
			eventType: eventType,
			info:      ci,
		}

	}
	return nil
}

func GetEventLogSignature(indexed []string) string {
	return indexed[0]
}

func _parsePacket(str string) (*Packet, error) {
	p := Packet{}
	e := rlpDecodeHex(str, &p)
	if e != nil {
		return nil, e
	}
	return &p, nil
}

func rlpDecodeHex(str string, out interface{}) error {
	str = strings.TrimPrefix(str, "0x")
	input, err := hex.DecodeString(str)
	if err != nil {
		return errors.Wrap(err, "hex.DecodeString ")
	}
	_, err = codec.RLP.UnmarshalFromBytes(input, out)
	if err != nil {
		return errors.Wrap(err, "rlp.Decode ")
	}
	return nil
}
