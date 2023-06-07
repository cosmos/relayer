package processor

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap/zapcore"
)

var _ zapcore.ObjectMarshaler = packetIBCMessage{}
var _ zapcore.ObjectMarshaler = channelIBCMessage{}
var _ zapcore.ObjectMarshaler = connectionIBCMessage{}
var _ zapcore.ObjectMarshaler = clientICQMessage{}

// pathEndMessages holds the different IBC messages that
// will attempt to be sent to the pathEnd.
type pathEndMessages struct {
	connectionMessages []connectionIBCMessage
	channelMessages    []channelIBCMessage
	packetMessages     []packetIBCMessage
	clientICQMessages  []clientICQMessage
}

type ibcMessage interface {
	// assemble executes the appropriate proof query function,
	// then, if successful, assembles the message for the destination.
	assemble(ctx context.Context, src, dst *pathEndRuntime) (provider.RelayerMessage, error)

	// tracker creates a message tracker for message status
	tracker(assembled provider.RelayerMessage) messageToTrack

	// msgType returns a human readable string for logging describing the message type.
	msgType() string

	// satisfies zapcore.ObjectMarshaler interface for use with zap.Object().
	MarshalLogObject(enc zapcore.ObjectEncoder) error
}

// packetIBCMessage holds a packet message's eventType and sequence along with it,
// useful for sending packets around internal to the PathProcessor.
type packetIBCMessage struct {
	info      provider.PacketInfo
	eventType string
}

// assemble executes the appropriate proof query function,
// then, if successful, assembles the packet message for the destination.
func (msg packetIBCMessage) assemble(
	ctx context.Context,
	src, dst *pathEndRuntime,
) (provider.RelayerMessage, error) {
	var packetProof func(context.Context, provider.PacketInfo, uint64) (provider.PacketProof, error)
	var assembleMessage func(provider.PacketInfo, provider.PacketProof) (provider.RelayerMessage, error)
	switch msg.eventType {
	case chantypes.EventTypeRecvPacket:
		packetProof = src.chainProvider.PacketCommitment
		assembleMessage = dst.chainProvider.MsgRecvPacket
	case chantypes.EventTypeAcknowledgePacket:
		packetProof = src.chainProvider.PacketAcknowledgement
		assembleMessage = dst.chainProvider.MsgAcknowledgement
	case chantypes.EventTypeTimeoutPacket:
		if msg.info.ChannelOrder == chantypes.ORDERED.String() {
			packetProof = src.chainProvider.NextSeqRecv
		} else {
			packetProof = src.chainProvider.PacketReceipt
		}

		assembleMessage = dst.chainProvider.MsgTimeout
	case chantypes.EventTypeTimeoutPacketOnClose:
		if msg.info.ChannelOrder == chantypes.ORDERED.String() {
			packetProof = src.chainProvider.NextSeqRecv
		} else {
			packetProof = src.chainProvider.PacketReceipt
		}

		assembleMessage = dst.chainProvider.MsgTimeoutOnClose
	default:
		return nil, fmt.Errorf("unexepected packet message eventType for message assembly: %s", msg.eventType)
	}
	if src.clientState.ClientID == ibcexported.LocalhostClientID {
		packetProof = src.localhostSentinelProofPacket
	}

	ctx, cancel := context.WithTimeout(ctx, packetProofQueryTimeout)
	defer cancel()

	var proof provider.PacketProof
	var err error
	proof, err = packetProof(ctx, msg.info, src.latestBlock.Height)
	if err != nil {
		return nil, fmt.Errorf("error querying packet proof: %w", err)
	}
	return assembleMessage(msg.info, proof)
}

// tracker creates a message tracker for message status
func (msg packetIBCMessage) tracker(assembled provider.RelayerMessage) messageToTrack {
	return packetMessageToTrack{
		msg:       msg,
		assembled: assembled,
	}
}

func (packetIBCMessage) msgType() string {
	return "packet"
}

// MarshalLogObject satisfies the zapcore.ObjectMarshaler interface
// so that you can use zap.Object("messages", r) when logging.
// This is typically useful when logging details about a partially sent result.
func (msg packetIBCMessage) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("type", msg.eventType)
	enc.AddString("src_port", msg.info.SourcePort)
	enc.AddString("src_channel", msg.info.SourceChannel)
	enc.AddString("dst_port", msg.info.DestPort)
	enc.AddString("dst_channel", msg.info.DestChannel)
	enc.AddUint64("sequence", msg.info.Sequence)
	enc.AddString("timeout_height", fmt.Sprintf(
		"%d-%d",
		msg.info.TimeoutHeight.RevisionNumber,
		msg.info.TimeoutHeight.RevisionHeight,
	))
	enc.AddUint64("timeout_timestamp", msg.info.TimeoutTimestamp)
	enc.AddString("data", base64.StdEncoding.EncodeToString(msg.info.Data))
	enc.AddString("ack", base64.StdEncoding.EncodeToString(msg.info.Ack))
	return nil
}

// channelKey returns channel key for new message by this eventType
// based on prior eventType.
func (msg packetIBCMessage) channelKey() (ChannelKey, error) {
	return PacketInfoChannelKey(msg.eventType, msg.info)
}

// channelIBCMessage holds a channel handshake message's eventType along with its details,
// useful for sending messages around internal to the PathProcessor.
type channelIBCMessage struct {
	eventType string
	info      provider.ChannelInfo
}

// assemble executes the appropriate proof query function,
// then, if successful, assembles the message for the destination.
func (msg channelIBCMessage) assemble(
	ctx context.Context,
	src, dst *pathEndRuntime,
) (provider.RelayerMessage, error) {
	var chanProof func(context.Context, provider.ChannelInfo, uint64) (provider.ChannelProof, error)
	var assembleMessage func(provider.ChannelInfo, provider.ChannelProof) (provider.RelayerMessage, error)
	switch msg.eventType {
	case chantypes.EventTypeChannelOpenInit:
		// don't need proof for this message
		assembleMessage = dst.chainProvider.MsgChannelOpenInit
	case chantypes.EventTypeChannelOpenTry:
		chanProof = src.chainProvider.ChannelProof
		assembleMessage = dst.chainProvider.MsgChannelOpenTry
	case chantypes.EventTypeChannelOpenAck:
		chanProof = src.chainProvider.ChannelProof
		assembleMessage = dst.chainProvider.MsgChannelOpenAck
	case chantypes.EventTypeChannelOpenConfirm:
		chanProof = src.chainProvider.ChannelProof
		assembleMessage = dst.chainProvider.MsgChannelOpenConfirm
	case chantypes.EventTypeChannelCloseInit:
		// don't need proof for this message
		assembleMessage = dst.chainProvider.MsgChannelCloseInit
	case chantypes.EventTypeChannelCloseConfirm:
		chanProof = src.chainProvider.ChannelProof
		assembleMessage = dst.chainProvider.MsgChannelCloseConfirm
	default:
		return nil, fmt.Errorf("unexepected channel message eventType for message assembly: %s", msg.eventType)
	}
	if src.clientState.ClientID == ibcexported.LocalhostClientID {
		chanProof = src.localhostSentinelProofChannel
	}

	var proof provider.ChannelProof
	var err error
	if chanProof != nil {
		proof, err = chanProof(ctx, msg.info, src.latestBlock.Height)
		if err != nil {
			return nil, fmt.Errorf("error querying channel proof: %w", err)
		}
	}
	return assembleMessage(msg.info, proof)
}

// tracker creates a message tracker for message status
func (msg channelIBCMessage) tracker(assembled provider.RelayerMessage) messageToTrack {
	return channelMessageToTrack{
		msg:       msg,
		assembled: assembled,
	}
}

func (channelIBCMessage) msgType() string {
	return "channel handshake"
}

// MarshalLogObject satisfies the zapcore.ObjectMarshaler interface
// so that you can use zap.Object("messages", r) when logging.
// This is typically useful when logging details about a partially sent result.
func (msg channelIBCMessage) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("type", msg.eventType)
	enc.AddString("port_id", msg.info.PortID)
	enc.AddString("channel_id", msg.info.ChannelID)
	enc.AddString("counterparty_port_id", msg.info.CounterpartyPortID)
	enc.AddString("counterparty_channel_id", msg.info.CounterpartyChannelID)
	enc.AddString("connection_id", msg.info.ConnID)
	enc.AddString("counterparty_connection_id", msg.info.CounterpartyConnID)
	enc.AddString("order", msg.info.Order.String())
	enc.AddString("version", msg.info.Version)
	return nil
}

// connectionIBCMessage holds a connection handshake message's eventType along with its details,
// useful for sending messages around internal to the PathProcessor.
type connectionIBCMessage struct {
	eventType string
	info      provider.ConnectionInfo
}

// assemble executes the appropriate proof query function,
// then, if successful, assembles the message for the destination.
func (msg connectionIBCMessage) assemble(
	ctx context.Context,
	src, dst *pathEndRuntime,
) (provider.RelayerMessage, error) {
	var connProof func(context.Context, provider.ConnectionInfo, uint64) (provider.ConnectionProof, error)
	var assembleMessage func(provider.ConnectionInfo, provider.ConnectionProof) (provider.RelayerMessage, error)
	switch msg.eventType {
	case conntypes.EventTypeConnectionOpenInit:
		// don't need proof for this message
		msg.info.CounterpartyCommitmentPrefix = src.chainProvider.CommitmentPrefix()
		assembleMessage = dst.chainProvider.MsgConnectionOpenInit
	case conntypes.EventTypeConnectionOpenTry:
		msg.info.CounterpartyCommitmentPrefix = src.chainProvider.CommitmentPrefix()
		connProof = src.chainProvider.ConnectionHandshakeProof
		assembleMessage = dst.chainProvider.MsgConnectionOpenTry
	case conntypes.EventTypeConnectionOpenAck:
		connProof = src.chainProvider.ConnectionHandshakeProof
		assembleMessage = dst.chainProvider.MsgConnectionOpenAck
	case conntypes.EventTypeConnectionOpenConfirm:
		connProof = src.chainProvider.ConnectionProof
		assembleMessage = dst.chainProvider.MsgConnectionOpenConfirm
	default:
		return nil, fmt.Errorf("unexepected connection message eventType for message assembly: %s", msg.eventType)
	}

	var proof provider.ConnectionProof
	var err error
	if connProof != nil {
		proof, err = connProof(ctx, msg.info, src.latestBlock.Height)
		if err != nil {
			return nil, fmt.Errorf("error querying connection proof: %w", err)
		}
	}

	return assembleMessage(msg.info, proof)
}

// tracker creates a message tracker for message status
func (msg connectionIBCMessage) tracker(assembled provider.RelayerMessage) messageToTrack {
	return connectionMessageToTrack{
		msg:       msg,
		assembled: assembled,
	}
}

func (connectionIBCMessage) msgType() string {
	return "connection handshake"
}

// MarshalLogObject satisfies the zapcore.ObjectMarshaler interface
// so that you can use zap.Object("messages", r) when logging.
// This is typically useful when logging details about a partially sent result.
func (msg connectionIBCMessage) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("type", msg.eventType)
	enc.AddString("client_id", msg.info.ClientID)
	enc.AddString("cntrprty_client_id", msg.info.CounterpartyClientID)
	enc.AddString("conn_id", msg.info.ConnID)
	enc.AddString("cntrprty_conn_id", msg.info.CounterpartyConnID)
	enc.AddString("cntrprty_commitment_prefix", msg.info.CounterpartyCommitmentPrefix.String())
	return nil
}

const (
	ClientICQTypeRequest  ClientICQType = "query_request"
	ClientICQTypeResponse ClientICQType = "query_response"
)

// clientICQMessage holds a client ICQ message info,
// useful for sending messages around internal to the PathProcessor.
type clientICQMessage struct {
	info provider.ClientICQInfo
}

// assemble executes the query against the source chain,
// then, if successful, assembles the response message for the destination.
func (msg clientICQMessage) assemble(
	ctx context.Context,
	src, dst *pathEndRuntime,
) (provider.RelayerMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, interchainQueryTimeout)
	defer cancel()

	proof, err := src.chainProvider.QueryICQWithProof(ctx, msg.info.Type, msg.info.Request, src.latestBlock.Height-1)
	if err != nil {
		return nil, fmt.Errorf("error during interchain query: %w", err)
	}

	return dst.chainProvider.MsgSubmitQueryResponse(msg.info.Chain, msg.info.QueryID, proof)
}

// tracker creates a message tracker for message status
func (msg clientICQMessage) tracker(assembled provider.RelayerMessage) messageToTrack {
	return clientICQMessageToTrack{
		msg:       msg,
		assembled: assembled,
	}
}

func (clientICQMessage) msgType() string {
	return "client ICQ"
}

// MarshalLogObject satisfies the zapcore.ObjectMarshaler interface
// so that you can use zap.Object("messages", r) when logging.
// This is typically useful when logging details about a partially sent result.
func (msg clientICQMessage) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("type", msg.info.Type)
	enc.AddString("query_id", string(msg.info.QueryID))
	enc.AddString("request", string(msg.info.Request))
	return nil
}

// processingMessage tracks the state of a IBC message currently being processed.
type processingMessage struct {
	assembled           bool
	lastProcessedHeight uint64
	retryCount          uint64
}

type packetProcessingCache map[ChannelKey]packetChannelMessageCache
type packetChannelMessageCache map[string]packetMessageSendCache
type packetMessageSendCache map[uint64]processingMessage

func (c packetChannelMessageCache) deleteMessages(toDelete ...map[string][]uint64) {
	for _, toDeleteMap := range toDelete {
		for message, toDeleteMessages := range toDeleteMap {
			for _, sequence := range toDeleteMessages {
				delete(c[message], sequence)
			}
		}
	}
}

type channelProcessingCache map[string]channelKeySendCache
type channelKeySendCache map[ChannelKey]processingMessage

func (c channelProcessingCache) deleteMessages(toDelete ...map[string][]ChannelKey) {
	for _, toDeleteMap := range toDelete {
		for message, toDeleteMessages := range toDeleteMap {
			for _, channel := range toDeleteMessages {
				delete(c[message], channel)
			}
		}
	}
}

type connectionProcessingCache map[string]connectionKeySendCache
type connectionKeySendCache map[ConnectionKey]processingMessage

func (c connectionProcessingCache) deleteMessages(toDelete ...map[string][]ConnectionKey) {
	for _, toDeleteMap := range toDelete {
		for message, toDeleteMessages := range toDeleteMap {
			for _, connection := range toDeleteMessages {
				delete(c[message], connection)
			}
		}
	}
}

type clientICQProcessingCache map[provider.ClientICQQueryID]processingMessage

// contains MsgRecvPacket from counterparty
// entire packet flow
type pathEndPacketFlowMessages struct {
	Src                   *pathEndRuntime
	Dst                   *pathEndRuntime
	ChannelKey            ChannelKey
	SrcPreTransfer        PacketSequenceCache
	SrcMsgTransfer        PacketSequenceCache
	DstMsgRecvPacket      PacketSequenceCache
	SrcMsgAcknowledgement PacketSequenceCache
	SrcMsgTimeout         PacketSequenceCache
	SrcMsgTimeoutOnClose  PacketSequenceCache
}

type pathEndConnectionHandshakeMessages struct {
	Src                         *pathEndRuntime
	Dst                         *pathEndRuntime
	SrcMsgConnectionPreInit     ConnectionMessageCache
	SrcMsgConnectionOpenInit    ConnectionMessageCache
	DstMsgConnectionOpenTry     ConnectionMessageCache
	SrcMsgConnectionOpenAck     ConnectionMessageCache
	DstMsgConnectionOpenConfirm ConnectionMessageCache
}

type pathEndChannelHandshakeMessages struct {
	Src                      *pathEndRuntime
	Dst                      *pathEndRuntime
	SrcMsgChannelPreInit     ChannelMessageCache
	SrcMsgChannelOpenInit    ChannelMessageCache
	DstMsgChannelOpenTry     ChannelMessageCache
	SrcMsgChannelOpenAck     ChannelMessageCache
	DstMsgChannelOpenConfirm ChannelMessageCache
}

type pathEndChannelCloseMessages struct {
	Src                       *pathEndRuntime
	Dst                       *pathEndRuntime
	SrcMsgChannelPreInit      ChannelMessageCache
	SrcMsgChannelCloseInit    ChannelMessageCache
	DstMsgChannelCloseConfirm ChannelMessageCache
}

type pathEndPacketFlowResponse struct {
	SrcMessages []packetIBCMessage
	DstMessages []packetIBCMessage

	DstChannelMessage []channelIBCMessage
}

type pathEndChannelHandshakeResponse struct {
	SrcMessages []channelIBCMessage
	DstMessages []channelIBCMessage
}

type pathEndConnectionHandshakeResponse struct {
	SrcMessages []connectionIBCMessage
	DstMessages []connectionIBCMessage
}

func packetInfoChannelKey(p provider.PacketInfo) ChannelKey {
	return ChannelKey{
		ChannelID:             p.SourceChannel,
		PortID:                p.SourcePort,
		CounterpartyChannelID: p.DestChannel,
		CounterpartyPortID:    p.DestPort,
	}
}

type messageToTrack interface {
	// assembledMsg returns the assembled message ready to send.
	assembledMsg() provider.RelayerMessage

	// msgType returns a human readable string for logging describing the message type.
	msgType() string

	// satisfies zapcore.ObjectMarshaler interface for use with zap.Object().
	MarshalLogObject(enc zapcore.ObjectEncoder) error
}

type packetMessageToTrack struct {
	msg       packetIBCMessage
	assembled provider.RelayerMessage
}

func (t packetMessageToTrack) assembledMsg() provider.RelayerMessage {
	return t.assembled
}

func (t packetMessageToTrack) msgType() string {
	return t.msg.msgType()
}

func (t packetMessageToTrack) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	return t.msg.MarshalLogObject(enc)
}

type connectionMessageToTrack struct {
	msg       connectionIBCMessage
	assembled provider.RelayerMessage
}

func (t connectionMessageToTrack) assembledMsg() provider.RelayerMessage {
	return t.assembled
}

func (t connectionMessageToTrack) msgType() string {
	return t.msg.msgType()
}

func (t connectionMessageToTrack) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	return t.msg.MarshalLogObject(enc)
}

type channelMessageToTrack struct {
	msg       channelIBCMessage
	assembled provider.RelayerMessage
}

func (t channelMessageToTrack) assembledMsg() provider.RelayerMessage {
	return t.assembled
}

func (t channelMessageToTrack) msgType() string {
	return t.msg.msgType()
}

func (t channelMessageToTrack) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	return t.msg.MarshalLogObject(enc)
}

type clientICQMessageToTrack struct {
	msg       clientICQMessage
	assembled provider.RelayerMessage
}

func (t clientICQMessageToTrack) assembledMsg() provider.RelayerMessage {
	return t.assembled
}

func (t clientICQMessageToTrack) msgType() string {
	return t.msg.msgType()
}

func (t clientICQMessageToTrack) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	return t.msg.MarshalLogObject(enc)
}

// orderFromString parses a string into a channel order byte.
func orderFromString(order string) chantypes.Order {
	switch strings.ToUpper(order) {
	case chantypes.UNORDERED.String():
		return chantypes.UNORDERED
	case chantypes.ORDERED.String():
		return chantypes.ORDERED
	default:
		return chantypes.NONE
	}
}
