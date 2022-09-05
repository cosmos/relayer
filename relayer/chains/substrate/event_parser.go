package substrate

import (
	"fmt"
	"strconv"

	rpcclienttypes "github.com/ComposableFi/go-substrate-rpc-client/v4/types"
	beefyclienttypes "github.com/ComposableFi/ics11-beefy/types"
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ibcMessage is the type used for parsing all possible properties of IBC messages
type ibcMessage struct {
	eventType string
	info      ibcMessageInfo
}

type ibcMessageInfo interface {
	parseAttrs(log *zap.Logger, attrs interface{})
	MarshalLogObject(enc zapcore.ObjectEncoder) error
}

// ibcMessagesFromTransaction parses all events within a transaction to find IBC messages
func (scp *SubstrateChainProcessor) handleIBCMessagesFromEvents(ibcEvents rpcclienttypes.IBCEventsQueryResult, height uint64, c processor.IBCMessagesCache) error {
	packetAccumulator := make(map[uint64]*packetInfo)
	for i := 0; i < len(ibcEvents); i++ {

		for eType, data := range ibcEvents[i] {
			var info ibcMessageInfo
			var eventType string

			switch eType {

			case CreateClient, UpgradeClient, ClientMisbehaviour:

				ce := new(clientInfo)
				ce.parseAttrs(scp.log, data)
				info = ce

				eventType = intoIBCEventType(eType)

			case UpdateClient:

				ce := new(clientUpdateInfo)
				ce.parseAttrs(scp.log, data)
				info = ce

				eventType = intoIBCEventType(eType)

			case OpenInitConnection, OpenTryConnection, OpenAckConnection, OpenConfirmConnection:

				ce := &connectionInfo{Height: height}
				ce.parseAttrs(scp.log, data)
				info = ce

				eventType = eType

			case OpenInitChannel, OpenTryChannel, OpenAckChannel, OpenConfirmChannel, CloseInitChannel, CloseConfirmChannel:
				ce := &channelInfo{Height: height}
				ce.parseAttrs(scp.log, data)
				info = ce

				eventType = intoIBCEventType(eType)

			case SendPacket, ReceivePacket, WriteAcknowledgement, AcknowledgePacket, TimeoutPacket, TimeoutOnClosePacket:

				// TODO: determine if is it a good key
				accumKey := data.(map[string]interface{})["sequence"].(uint64)

				_, exists := packetAccumulator[accumKey]
				if !exists {
					packetAccumulator[accumKey] = &packetInfo{Height: height}
				}

				packetAccumulator[accumKey].parseAttrs(scp.log, data)

				info = packetAccumulator[accumKey]
				if eType != WriteAcknowledgement {
					eventType = intoIBCEventType(eType)
				}

			default:
				panic("event not recognized")
			}

			if info == nil {
				// Not an IBC message, don't need to log here
				continue
			}

			scp.handleMessage(ibcMessage{
				eventType: eventType,
				info:      info,
			}, c)

		}
	}

	return nil
}

// client info attributes and methods
type clientInfo struct {
	Height          clienttypes.Height `json:"height"`
	ClientID        string             `json:"client_id"`
	ClientType      uint32             `json:"client_type"`
	ConsensusHeight clienttypes.Height `json:"consensus_height"`
}

func (c clientInfo) ClientState() provider.ClientState {
	return provider.ClientState{
		ClientID:        c.ClientID,
		ConsensusHeight: c.ConsensusHeight,
	}
}

func (res *clientInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("client_id", res.ClientID)
	enc.AddUint64("consensus_height", res.ConsensusHeight.RevisionHeight)
	enc.AddUint64("consensus_height_revision", res.ConsensusHeight.RevisionNumber)
	return nil
}

func (res *clientInfo) parseAttrs(log *zap.Logger, attributes interface{}) {
	attrs := attributes.(map[string]interface{})

	var err error
	if res.Height, err = parseHeight(attrs["height"]); err != nil {
		log.Error("error parsing client consensus height: ",
			zap.Error(err),
		)
		return
	}

	res.ClientID = attrs["client_id"].(string)
	res.ClientType = attrs["client_type"].(uint32)

	if res.ConsensusHeight, err = parseHeight(attrs["consensus_height"]); err != nil {
		log.Error("error parsing client consensus height: ",
			zap.Error(err),
		)
		return
	}

}

// client update info attributes and methods
type clientUpdateInfo struct {
	Common clientInfo              `json:"common"`
	Header beefyclienttypes.Header `json:"header"`
}

func (res *clientUpdateInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("client_id", res.Common.ClientID)
	enc.AddUint64("consensus_height", res.Common.ConsensusHeight.RevisionHeight)
	enc.AddUint64("consensus_height_revision", res.Common.ConsensusHeight.RevisionNumber)
	// TODO: include header if
	return nil
}
func (res *clientUpdateInfo) parseAttrs(log *zap.Logger, attributes interface{}) {
	attrs := attributes.(map[string]interface{})

	clientInfo := new(clientInfo)
	clientInfo.parseAttrs(log, attrs["common"])

	res.Common = *clientInfo

	var err error
	if res.Header, err = parseHeader(attrs["header"]); err != nil {
		log.Error("error parsing beefy header: ",
			zap.Error(err),
		)
		return
	}
}

// // packet attributes
// type packetEvent struct {
// 	height             clienttypes.Height `json:"height"`
// 	sequence           uint64             `json:"sequence"`
// 	sourcePort         string             `json:"source_port"`
// 	sourceCannel       uint64             `json:"source_channel"`
// 	destinationPort    string             `json:"destination_port"`
// 	destinationChannel uint64             `json:"destination_channel"`
// 	data               []byte             `json:"data"`
// 	timeoutHeight      clienttypes.Height `json:"timeout_height"`
// 	timeoutTimestamp   uint64             `json:"timeout_timestamp"`
// }

// alias type to the provider types, used for adding parser methods
type packetInfo provider.PacketInfo

func (res *packetInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddUint64("sequence", res.Sequence)
	enc.AddString("src_channel", res.SourceChannel)
	enc.AddString("src_port", res.SourcePort)
	enc.AddString("dst_channel", res.DestChannel)
	enc.AddString("dst_port", res.DestPort)
	return nil
}

func (res *packetInfo) parseAttrs(log *zap.Logger, attributes interface{}) {
	attrs := attributes.(map[string]interface{})

	res.Sequence = attrs["sequence"].(uint64)
	res.SourcePort = attrs["source_port"].(string)
	res.SourceChannel = attrs["source_channel"].(string)
	res.DestPort = attrs["destination_port"].(string)
	res.DestChannel = attrs["destination_channel"].(string)
	res.Data = attrs["data"].([]byte)

	var err error
	if res.TimeoutHeight, err = parseHeight(attrs["timeout_height"]); err != nil {
		log.Error("error parsing packet height: ",
			zap.Error(err),
		)
		return
	}

	res.TimeoutTimestamp = attrs["timeout_timestamp"].(uint64)

	ack, found := attrs["ack"]
	if found {
		res.Ack = ack.([]byte)
	}

	// TODO: how to populate order
	// Order: ,
	// TODO: how to populate version
	// Version

}

// // channel attributes
// type channelEvent struct {
// 	height                clienttypes.Height `json:"height"`
// 	channelID             uint64             `json:"channel_id"`
// 	portID                string             `json:"port_id"`
// 	connectionID          string             `json:"connection_id"`
// 	counterpartyPortID    uint64             `json:"counterparty_port_id"`
// 	counterpartyChannelID string             `json:"counterparty_channel_id"`
// }

// alias type to the provider types, used for adding parser methods
type channelInfo provider.ChannelInfo

func (res *channelInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("channel_id", res.ChannelID)
	enc.AddString("port_id", res.PortID)
	enc.AddString("counterparty_channel_id", res.CounterpartyChannelID)
	enc.AddString("counterparty_port_id", res.CounterpartyPortID)
	return nil
}

func (res *channelInfo) parseAttrs(log *zap.Logger, attributes interface{}) {
	attrs := attributes.(map[string]interface{})

	res.PortID = attrs["port_id"].(string)
	res.ChannelID = attrs["channel_id"].(string)
	res.CounterpartyPortID = attrs["counterparty_port_id"].(string)
	res.CounterpartyChannelID = attrs["counterparty_channel_id"].(string)
	res.ConnID = attrs["connection_id"].(string)
}

// // connection attributes
// type connectionEvent struct {
// 	height                   clienttypes.Height `json:"height"`
// 	connectionID             string             `json:"connection_id"`
// 	clientID                 string             `json:"client_id"`
// 	counterpartyConnectionID string             `json:"counterparty_connection_id"`
// 	counterpartyClientID     string             `json:"counterparty_client_id"`
// }

// alias type to the provider types, used for adding parser methods
type connectionInfo provider.ConnectionInfo

func (res *connectionInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("connection_id", res.ConnID)
	enc.AddString("client_id", res.ClientID)
	enc.AddString("counterparty_connection_id", res.CounterpartyConnID)
	enc.AddString("counterparty_client_id", res.CounterpartyClientID)
	return nil
}

func (res *connectionInfo) parseAttrs(log *zap.Logger, attributes interface{}) {
	attrs := attributes.(map[string]interface{})

	res.ConnID = attrs["connection_id"].(string)
	res.ClientID = attrs["client_id"].(string)
	res.CounterpartyClientID = attrs["counterparty_client_id"].(string)
	res.CounterpartyConnID = attrs["counterparty_connection_id"].(string)
}

func parseHeight(h interface{}) (clienttypes.Height, error) {
	height := h.(map[string]interface{})

	revisionNumber, err := strconv.ParseUint(height["revision_number"].(string), 10, 64)
	if err != nil {

		return clienttypes.Height{}, fmt.Errorf("error parsing revision number: %s", err)
	}

	revisionHeight, err := strconv.ParseUint(height["revision_height"].(string), 10, 64)
	if err != nil {
		return clienttypes.Height{}, fmt.Errorf("error parsing revision height: %s", err)
	}

	return clienttypes.Height{
		RevisionNumber: revisionNumber,
		RevisionHeight: revisionHeight,
	}, nil
}

func parseHeader(header interface{}) (beefyclienttypes.Header, error) {
	// TODO: parse beefy header
	// headerAttrs := header.(map[string]interface{})

	var headers beefyclienttypes.Header

	return headers, nil
}
