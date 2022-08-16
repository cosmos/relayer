package substrate

import (
	"encoding/json"
	"strconv"

	rpcclienttypes "github.com/ComposableFi/go-substrate-rpc-client/v4/types"
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap/zapcore"
)

// ibcMessage is the type used for parsing all possible properties of IBC messages
type ibcMessage struct {
	eventType string
	info      ibcMessageInfo
}

type ibcMessageInfo interface {
	MarshalLogObject(enc zapcore.ObjectEncoder) error
}

// ibcMessagesFromTransaction parses all events within a transaction to find IBC messages
func (scp *SubstrateChainProcessor) handleIBCMessagesFromEvents(ibcEvents rpcclienttypes.IBCEventsQueryResult, height uint64, c processor.IBCMessagesCache) error {
	for i := 0; i < len(ibcEvents); i++ {
		for eType, data := range ibcEvents[i] {
			var info ibcMessageInfo
			var eventType string

			marshalled, _ := json.Marshal(data)

			switch eType {

			case "CreateClient":
			case "UpdateClient":
			case "UpgradeClient":
			case "ClientMisbehaviour":
				ce := clientInfo{}
				json.Unmarshal(marshalled, &ce)
				info = &ce
				eventType = eType

			case "OpenInitConnection":
			case "OpenTryConnection":
			case "OpenAckConnection":
			case "OpenConfirmConnection":
				ce := connectionEvent{}
				json.Unmarshal(marshalled, &ce)
				info = &connectionInfo{
					Height:               height,
					ConnID:               ce.connectionID,
					ClientID:             ce.clientID,
					CounterpartyConnID:   ce.counterpartyConnectionID,
					CounterpartyClientID: ce.counterpartyClientID,
				}
				eventType = eType

			case "OpenInitChannel":
			case "OpenTryChannel":
			case "OpenAckChannel":
			case "OpenConfirmChannel":
			case "CloseInitChannel":
			case "CloseConfirmChannel":
				ce := channelEvent{}
				json.Unmarshal(marshalled, &ce)
				info = &channelInfo{
					Height:                height,
					PortID:                ce.portID,
					ChannelID:             strconv.FormatUint(ce.channelID, 10),
					CounterpartyPortID:    strconv.FormatUint(ce.counterpartyPortID, 10),
					CounterpartyChannelID: ce.counterpartyChannelID,
					ConnID:                ce.connectionID,
					CounterpartyConnID:    ce.counterpartyChannelID,
					// TODO: What is order
					// Order: ,
					// TODO: What is version
					// Version: ,
				}
				eventType = eType

			case "SendPacket":
			case "ReceivePacket":
			case "WriteAcknowledgement":
			case "AcknowledgePacket":
			case "TimeoutPacket":
			case "TimeoutOnClosePacket":
				ce := packetEvent{}
				json.Unmarshal(marshalled, &ce)
				info = &packetInfo{
					Height:        height,
					Sequence:      ce.sequence,
					SourcePort:    ce.sourcePort,
					SourceChannel: strconv.FormatUint(ce.sourceCannel, 10),
					DestPort:      ce.destinationPort,
					DestChannel:   strconv.FormatUint(ce.destinationChannel, 10),
					// TODO: What is channel order
					// ChannelOrder: ,
					Data:             ce.data,
					TimeoutHeight:    ce.timeoutHeight,
					TimeoutTimestamp: ce.timeoutTimestamp,
					// TODO: What is Ack
					// Ack: ,

				}
				eventType = eType

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

type clientInfo struct {
	height          clienttypes.Height `json:"height"`
	clientID        string             `json:"client_id"`
	clientType      uint32             `json:"client_type"`
	consensusHeight clienttypes.Height `json:"consensus_height"`
}

func (c clientInfo) ClientState() provider.ClientState {
	return provider.ClientState{
		ClientID:        c.clientID,
		ConsensusHeight: c.consensusHeight,
	}
}

func (res *clientInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("client_id", res.clientID)
	enc.AddUint64("consensus_height", res.consensusHeight.RevisionHeight)
	enc.AddUint64("consensus_height_revision", res.consensusHeight.RevisionNumber)
	return nil
}

type packetEvent struct {
	height             clienttypes.Height `json:"height"`
	sequence           uint64             `json:"sequence"`
	sourcePort         string             `json:"source_port"`
	sourceCannel       uint64             `json:"source_channel"`
	destinationPort    string             `json:"destination_port"`
	destinationChannel uint64             `json:"destination_channel"`
	data               []byte             `json:"data"`
	timeoutHeight      clienttypes.Height `json:"timeout_height"`
	timeoutTimestamp   uint64             `json:"timeout_timestamp"`
}

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

type channelEvent struct {
	height                clienttypes.Height `json:"height"`
	channelID             uint64             `json:"channel_id"`
	portID                string             `json:"port_id"`
	connectionID          string             `json:"connection_id"`
	counterpartyPortID    uint64             `json:"counterparty_port_id"`
	counterpartyChannelID string             `json:"counterparty_channel_id"`
}

// alias type to the provider types, used for adding parser methods
type channelInfo provider.ChannelInfo

func (res *channelInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("channel_id", res.ChannelID)
	enc.AddString("port_id", res.PortID)
	enc.AddString("counterparty_channel_id", res.CounterpartyChannelID)
	enc.AddString("counterparty_port_id", res.CounterpartyPortID)
	return nil
}

type connectionEvent struct {
	height                   clienttypes.Height `json:"height"`
	connectionID             string             `json:"connection_id"`
	clientID                 string             `json:"client_id"`
	counterpartyConnectionID string             `json:"counterparty_connection_id"`
	counterpartyClientID     string             `json:"counterparty_client_id"`
}

// alias type to the provider types, used for adding parser methods
type connectionInfo provider.ConnectionInfo

func (res *connectionInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("connection_id", res.ConnID)
	enc.AddString("client_id", res.ClientID)
	enc.AddString("counterparty_connection_id", res.CounterpartyConnID)
	enc.AddString("counterparty_client_id", res.CounterpartyClientID)
	return nil
}
