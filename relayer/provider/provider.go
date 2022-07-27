package provider

import (
	"context"
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v4/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v4/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v4/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v4/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v4/modules/core/exported"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ProviderConfig interface {
	NewProvider(log *zap.Logger, homepath string, debug bool, chainName string) (ChainProvider, error)
	Validate() error
}

type RelayerMessage interface {
	Type() string
	MsgBytes() ([]byte, error)
}

type RelayerTxResponse struct {
	Height int64
	TxHash string
	Code   uint32
	Data   string
	Events []RelayerEvent
}

type RelayerEvent struct {
	EventType  string
	Attributes map[string]string
}

type LatestBlock struct {
	Height uint64
	Time   time.Time
}

type IBCHeader interface {
	IBCHeaderIndicator()
	Height() uint64
}

// ClientState holds the current state of a client from a single chain's perspective
type ClientState struct {
	ClientID        string
	ConsensusHeight clienttypes.Height
}

// ClientTrustedState holds the current state of a client from the perspective of both involved chains,
// i.e. ClientState enriched with the trusted IBC header of the counterparty chain.
type ClientTrustedState struct {
	ClientState ClientState
	IBCHeader   IBCHeader
}

// PacketInfo contains any relevant properties from packet flow messages
// which may be necessary to construct the next message in the packet flow.
type PacketInfo struct {
	Height           uint64
	Sequence         uint64
	SourcePort       string
	SourceChannel    string
	DestPort         string
	DestChannel      string
	ChannelOrder     string
	Data             []byte
	TimeoutHeight    clienttypes.Height
	TimeoutTimestamp uint64
	Ack              []byte
}

// ConnectionInfo contains relevant properties from connection handshake messages
// which may be necessary to construct the next message for the counterparty chain.
type ConnectionInfo struct {
	Height               uint64
	ConnID               string
	ClientID             string
	CounterpartyClientID string
	CounterpartyConnID   string
}

// ChannelInfo contains relevant properties from channel handshake messages
// which may be necessary to construct the next message for the counterparty chain.
type ChannelInfo struct {
	Height                uint64
	PortID                string
	ChannelID             string
	CounterpartyPortID    string
	CounterpartyChannelID string
	ConnID                string

	// CounterpartyConnID doesn't come from any events, but is needed for
	// MsgChannelOpenTry, so should be added manually for MsgChannelOpenInit.
	CounterpartyConnID string

	Order   chantypes.Order
	Version string
}

// PacketProof includes all of the proof parameters needed for packet flows.
type PacketProof struct {
	Proof       []byte
	ProofHeight clienttypes.Height
}

// ConnectionProof includes all of the proof parameters needed for the connection handshake.
type ConnectionProof struct {
	ConsensusStateProof  []byte
	ConnectionStateProof []byte
	ClientStateProof     []byte
	ProofHeight          clienttypes.Height
	ClientState          ibcexported.ClientState
}

type ChannelProof struct {
	Proof       []byte
	ProofHeight clienttypes.Height
	Ordering    chantypes.Order
	Version     string
}

// loggableEvents is an unexported wrapper type for a slice of RelayerEvent,
// to satisfy the zapcore.ArrayMarshaler interface.
type loggableEvents []RelayerEvent

// MarshalLogObject satisfies the zapcore.ObjectMarshaler interface.
func (e RelayerEvent) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("event_type", e.EventType)
	for k, v := range e.Attributes {
		enc.AddString("event_attr: "+k, v)
	}
	return nil
}

// MarshalLogArray satisfies the zapcore.ArrayMarshaler interface.
func (es loggableEvents) MarshalLogArray(enc zapcore.ArrayEncoder) error {
	for _, e := range es {
		enc.AppendObject(e)
	}
	return nil
}

// MarshalLogObject satisfies the zapcore.ObjectMarshaler interface.
func (r RelayerTxResponse) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddInt64("height", r.Height)
	enc.AddString("tx_hash", r.TxHash)
	enc.AddUint32("code", r.Code)
	enc.AddString("data", r.Data)
	enc.AddArray("events", loggableEvents(r.Events))
	return nil
}

type KeyProvider interface {
	CreateKeystore(path string) error
	KeystoreCreated(path string) bool
	AddKey(name string, coinType uint32) (output *KeyOutput, err error)
	RestoreKey(name, mnemonic string, coinType uint32) (address string, err error)
	ShowAddress(name string) (address string, err error)
	ListAddresses() (map[string]string, error)
	DeleteKey(name string) error
	KeyExists(name string) bool
	ExportPrivKeyArmor(keyName string) (armor string, err error)
}

type ChainProvider interface {
	QueryProvider
	KeyProvider

	Init() error
	CreateClient(clientState ibcexported.ClientState, dstHeader ibcexported.Header, signer string) (RelayerMessage, error)
	SubmitMisbehavior( /*TODO TBD*/ ) (RelayerMessage, error)
	ConnectionOpenInit(srcClientId, dstClientId string, dstHeader ibcexported.Header) ([]RelayerMessage, error)
	ConnectionOpenTry(ctx context.Context, dstQueryProvider QueryProvider, dstHeader ibcexported.Header, srcClientId, dstClientId, srcConnId, dstConnId string) ([]RelayerMessage, error)
	ConnectionOpenAck(ctx context.Context, dstQueryProvider QueryProvider, dstHeader ibcexported.Header, srcClientId, srcConnId, dstClientId, dstConnId string) ([]RelayerMessage, error)
	ConnectionOpenConfirm(ctx context.Context, dstQueryProvider QueryProvider, dstHeader ibcexported.Header, dstConnId, srcClientId, srcConnId string) ([]RelayerMessage, error)
	ChannelOpenInit(srcClientId, srcConnId, srcPortId, srcVersion, dstPortId string, order chantypes.Order, dstHeader ibcexported.Header) ([]RelayerMessage, error)
	ChannelOpenTry(ctx context.Context, dstQueryProvider QueryProvider, dstHeader ibcexported.Header, srcPortId, dstPortId, srcChanId, dstChanId, srcVersion, srcConnectionId, srcClientId string) ([]RelayerMessage, error)
	ChannelOpenAck(ctx context.Context, dstQueryProvider QueryProvider, dstHeader ibcexported.Header, srcClientId, srcPortId, srcChanId, dstChanId, dstPortId string) ([]RelayerMessage, error)
	ChannelOpenConfirm(ctx context.Context, dstQueryProvider QueryProvider, dstHeader ibcexported.Header, srcClientId, srcPortId, srcChanId, dstPortId, dstChannId string) ([]RelayerMessage, error)
	ChannelCloseInit(srcPortId, srcChanId string) (RelayerMessage, error)
	ChannelCloseConfirm(ctx context.Context, dstQueryProvider QueryProvider, dsth int64, dstChanId, dstPortId, srcPortId, srcChanId string) (RelayerMessage, error)

	// ValidatePacket makes sure packet is valid to be relayed.
	// It should return TimeoutHeightError, TimeoutTimestampError, or TimeoutOnCloseError
	// for packet timeout scenarios so that timeout message can be written to other chain.
	ValidatePacket(msgTransfer PacketInfo, latestBlock LatestBlock) error

	// [Begin] Packet flow IBC message assembly functions

	// These functions query the proof of the packet state on the chain.

	// PacketCommitment queries for proof that a MsgTransfer has been committed on the chain.
	PacketCommitment(ctx context.Context, msgTransfer PacketInfo, height uint64) (PacketProof, error)

	// PacketAcknowledgement queries for proof that a MsgRecvPacket has been committed on the chain.
	PacketAcknowledgement(ctx context.Context, msgRecvPacket PacketInfo, height uint64) (PacketProof, error)

	// PacketReceipt queries for proof that a MsgRecvPacket has not been committed to the chain.
	PacketReceipt(ctx context.Context, msgTransfer PacketInfo, height uint64) (PacketProof, error)

	// NextSeqRecv queries for the appropriate proof required to prove the next expected packet sequence number
	// for a given counterparty channel. This is used in ORDERED channels to ensure packets are being delivered in the
	// exact same order as they were sent over the wire.
	NextSeqRecv(ctx context.Context, msgTransfer PacketInfo, height uint64) (PacketProof, error)

	// MsgRecvPacket takes the packet information from a MsgTransfer along with the packet commitment,
	// and assembles a full MsgRecvPacket ready to write to the chain.
	MsgRecvPacket(msgTransfer PacketInfo, proof PacketProof) (RelayerMessage, error)

	// MsgAcknowledgement takes the packet information from a MsgRecvPacket along with the packet acknowledgement,
	// and assembles a full MsgAcknowledgement ready to write to the chain.
	MsgAcknowledgement(msgRecvPacket PacketInfo, proofAcked PacketProof) (RelayerMessage, error)

	// MsgTimeout takes the packet information from a MsgTransfer along
	// with the packet receipt to prove that the packet was never relayed,
	// i.e. that the MsgRecvPacket was never written to the counterparty chain,
	// and assembles a full MsgTimeout ready to write to the chain,
	// i.e. the chain where the MsgTransfer was committed.
	MsgTimeout(msgTransfer PacketInfo, proofUnreceived PacketProof) (RelayerMessage, error)

	// MsgTimeoutOnClose takes the packet information from a MsgTransfer along
	// with the packet receipt to prove that the packet was never relayed,
	// i.e. that the MsgRecvPacket was never written to the counterparty chain,
	// and assembles a full MsgTimeoutOnClose ready to write to the chain,
	// i.e. the chain where the MsgTransfer was committed.
	MsgTimeoutOnClose(msgTransfer PacketInfo, proofUnreceived PacketProof) (RelayerMessage, error)

	// [End] Packet flow IBC message assembly

	// [Begin] Connection handshake IBC message assembly

	// ConnectionHandshakeProof queries for proof of an initialized connection handshake.
	ConnectionHandshakeProof(ctx context.Context, msgOpenInit ConnectionInfo, height uint64) (ConnectionProof, error)

	// ConnectionProof queries for proof of an acked handshake.
	ConnectionProof(ctx context.Context, msgOpenAck ConnectionInfo, height uint64) (ConnectionProof, error)

	// MsgConnectionOpenInit takes connection info and assembles a MsgConnectionOpenInit message
	// ready to write to the chain. The connection proof is not needed here, but it needs
	// the same signature as the other connection message assembly methods.
	MsgConnectionOpenInit(info ConnectionInfo, proof ConnectionProof) (RelayerMessage, error)

	// MsgConnectionOpenTry takes connection info along with the proof that the connection has been initialized
	// on the counterparty chain, and assembles a MsgConnectionOpenTry message ready to write to the chain.
	MsgConnectionOpenTry(msgOpenInit ConnectionInfo, proof ConnectionProof) (RelayerMessage, error)

	// MsgConnectionOpenAck takes connection info along with the proof that the connection try has occurred
	// on the counterparty chain, and assembles a MsgConnectionOpenAck message ready to write to the chain.
	MsgConnectionOpenAck(msgOpenTry ConnectionInfo, proof ConnectionProof) (RelayerMessage, error)

	// MsgConnectionOpenConfirm takes connection info along with the proof that the connection ack has occurred
	// on the counterparty chain, and assembles a MsgConnectionOpenConfirm message ready to write to the chain.
	MsgConnectionOpenConfirm(msgOpenAck ConnectionInfo, proof ConnectionProof) (RelayerMessage, error)

	// [End] Connection handshake IBC message assembly

	// [Begin] Channel handshake IBC message assembly

	// ChannelProof queries for proof of a channel state.
	ChannelProof(ctx context.Context, msg ChannelInfo, height uint64) (ChannelProof, error)

	// MsgChannelOpenInit takes channel info and assembles a MsgChannelOpenInit message
	// ready to write to the chain. The channel proof is not needed here, but it needs
	// the same signature as the other channel message assembly methods.
	MsgChannelOpenInit(info ChannelInfo, proof ChannelProof) (RelayerMessage, error)

	// MsgChannelOpenTry takes channel info along with the proof that the channel has been initialized
	// on the counterparty chain, and assembles a MsgChannelOpenTry message ready to write to the chain.
	MsgChannelOpenTry(msgOpenInit ChannelInfo, proof ChannelProof) (RelayerMessage, error)

	// MsgChannelOpenAck takes channel info along with the proof that the channel try has occurred
	// on the counterparty chain, and assembles a MsgChannelOpenAck message ready to write to the chain.
	MsgChannelOpenAck(msgOpenTry ChannelInfo, proof ChannelProof) (RelayerMessage, error)

	// MsgChannelOpenConfirm takes connection info along with the proof that the channel ack has occurred
	// on the counterparty chain, and assembles a MsgChannelOpenConfirm message ready to write to the chain.
	MsgChannelOpenConfirm(msgOpenAck ChannelInfo, proof ChannelProof) (RelayerMessage, error)

	// MsgChannelCloseInit takes channel info and assembles a MsgChannelCloseInit message
	// ready to write to the chain. The channel proof is not needed here, but it needs
	// the same signature as the other channel message assembly methods.
	MsgChannelCloseInit(info ChannelInfo, proof ChannelProof) (RelayerMessage, error)

	// MsgChannelCloseConfirm takes connection info along with the proof that the channel close has occurred
	// on the counterparty chain, and assembles a MsgChannelCloseConfirm message ready to write to the chain.
	MsgChannelCloseConfirm(msgCloseInit ChannelInfo, proof ChannelProof) (RelayerMessage, error)

	// [End] Channel handshake IBC message assembly

	// [Begin] Client IBC message assembly

	// MsgUpdateClientHeader takes the latest chain header, in addition to the latest client trusted header
	// and assembles a new header for updating the light client on the counterparty chain.
	MsgUpdateClientHeader(latestHeader IBCHeader, trustedHeight clienttypes.Height, trustedHeader IBCHeader) (ibcexported.Header, error)

	// MsgUpdateClient takes an update client header to prove trust for the previous
	// consensus height and the new height, and assembles a MsgUpdateClient message
	// formatted for this chain.
	MsgUpdateClient(clientId string, counterpartyHeader ibcexported.Header) (RelayerMessage, error)

	// [End] Client IBC message assembly

	// TODO remove these message assembly functions in favor of the above.
	MsgRelayAcknowledgement(ctx context.Context, dst ChainProvider, dstChanId, dstPortId, srcChanId, srcPortId string, dsth int64, packet RelayPacket) (RelayerMessage, error)
	MsgTransfer(amount sdk.Coin, dstChainId, dstAddr, srcPortId, srcChanId string, timeoutHeight, timeoutTimestamp uint64) (RelayerMessage, error)
	MsgRelayTimeout(ctx context.Context, dst ChainProvider, dsth int64, packet RelayPacket, dstChanId, dstPortId, srcChanId, srcPortId string, order chantypes.Order) (RelayerMessage, error)
	MsgRelayRecvPacket(ctx context.Context, dst ChainProvider, dsth int64, packet RelayPacket, dstChanId, dstPortId, srcChanId, srcPortId string) (RelayerMessage, error)
	MsgUpgradeClient(srcClientId string, consRes *clienttypes.QueryConsensusStateResponse, clientRes *clienttypes.QueryClientStateResponse) (RelayerMessage, error)
	RelayPacketFromSequence(ctx context.Context, src, dst ChainProvider, srch, dsth, seq uint64, dstChanId, dstPortId, dstClientId, srcChanId, srcPortId, srcClientId string, order chantypes.Order) (RelayerMessage, RelayerMessage, error)
	AcknowledgementFromSequence(ctx context.Context, dst ChainProvider, dsth, seq uint64, dstChanId, dstPortId, srcChanId, srcPortId string) (RelayerMessage, error)

	SendMessage(ctx context.Context, msg RelayerMessage, memo string) (*RelayerTxResponse, bool, error)
	SendMessages(ctx context.Context, msgs []RelayerMessage, memo string) (*RelayerTxResponse, bool, error)

	// TODO consolidate with IBCHeaderAtHeight
	GetLightSignedHeaderAtHeight(ctx context.Context, h int64) (ibcexported.Header, error)
	GetIBCUpdateHeader(ctx context.Context, srch int64, dst ChainProvider, dstClientId string) (ibcexported.Header, error)

	// IBCHeaderAtHeight returns the IBC compatible block header at a specific height.
	IBCHeaderAtHeight(ctx context.Context, h int64) (IBCHeader, error)

	ChainName() string
	ChainId() string
	Type() string
	ProviderConfig() ProviderConfig
	Key() string
	Address() (string, error)
	Timeout() string
	TrustingPeriod(ctx context.Context) (time.Duration, error)
	WaitForNBlocks(ctx context.Context, n int64) error
	Sprint(toPrint proto.Message) (string, error)
}

// Do we need intermediate types? i.e. can we use the SDK types for both substrate and cosmos?
type QueryProvider interface {
	// chain
	BlockTime(ctx context.Context, height int64) (int64, error)
	QueryTx(ctx context.Context, hashHex string) (*RelayerTxResponse, error)
	QueryTxs(ctx context.Context, page, limit int, events []string) ([]*RelayerTxResponse, error)
	QueryLatestHeight(ctx context.Context) (int64, error)
	QueryHeaderAtHeight(ctx context.Context, height int64) (ibcexported.Header, error)

	// bank
	QueryBalance(ctx context.Context, keyName string) (sdk.Coins, error)
	QueryBalanceWithAddress(ctx context.Context, addr string) (sdk.Coins, error)

	// staking
	QueryUnbondingPeriod(context.Context) (time.Duration, error)

	// ics 02 - client
	QueryClientState(ctx context.Context, height int64, clientid string) (ibcexported.ClientState, error)
	QueryClientStateResponse(ctx context.Context, height int64, srcClientId string) (*clienttypes.QueryClientStateResponse, error)
	QueryClientConsensusState(ctx context.Context, chainHeight int64, clientid string, clientHeight ibcexported.Height) (*clienttypes.QueryConsensusStateResponse, error)
	QueryUpgradedClient(ctx context.Context, height int64) (*clienttypes.QueryClientStateResponse, error)
	QueryUpgradedConsState(ctx context.Context, height int64) (*clienttypes.QueryConsensusStateResponse, error)
	QueryConsensusState(ctx context.Context, height int64) (ibcexported.ConsensusState, int64, error)
	QueryClients(ctx context.Context) (clienttypes.IdentifiedClientStates, error)
	AutoUpdateClient(ctx context.Context, dst ChainProvider, thresholdTime time.Duration, srcClientId, dstClientId string) (time.Duration, error)

	// ics 03 - connection
	QueryConnection(ctx context.Context, height int64, connectionid string) (*conntypes.QueryConnectionResponse, error)
	QueryConnections(ctx context.Context) (conns []*conntypes.IdentifiedConnection, err error)
	QueryConnectionsUsingClient(ctx context.Context, height int64, clientid string) (*conntypes.QueryConnectionsResponse, error)
	GenerateConnHandshakeProof(ctx context.Context, height int64, clientId, connId string) (clientState ibcexported.ClientState,
		clientStateProof []byte, consensusProof []byte, connectionProof []byte,
		connectionProofHeight ibcexported.Height, err error)
	NewClientState(dstUpdateHeader ibcexported.Header, dstTrustingPeriod, dstUbdPeriod time.Duration, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour bool) (ibcexported.ClientState, error)

	// ics 04 - channel
	QueryChannel(ctx context.Context, height int64, channelid, portid string) (chanRes *chantypes.QueryChannelResponse, err error)
	QueryChannelClient(ctx context.Context, height int64, channelid, portid string) (*clienttypes.IdentifiedClientState, error)
	QueryConnectionChannels(ctx context.Context, height int64, connectionid string) ([]*chantypes.IdentifiedChannel, error)
	QueryChannels(ctx context.Context) ([]*chantypes.IdentifiedChannel, error)
	QueryPacketCommitments(ctx context.Context, height uint64, channelid, portid string) (commitments *chantypes.QueryPacketCommitmentsResponse, err error)
	QueryPacketAcknowledgements(ctx context.Context, height uint64, channelid, portid string) (acknowledgements []*chantypes.PacketState, err error)
	QueryUnreceivedPackets(ctx context.Context, height uint64, channelid, portid string, seqs []uint64) ([]uint64, error)
	QueryUnreceivedAcknowledgements(ctx context.Context, height uint64, channelid, portid string, seqs []uint64) ([]uint64, error)
	QueryNextSeqRecv(ctx context.Context, height int64, channelid, portid string) (recvRes *chantypes.QueryNextSequenceReceiveResponse, err error)
	QueryPacketCommitment(ctx context.Context, height int64, channelid, portid string, seq uint64) (comRes *chantypes.QueryPacketCommitmentResponse, err error)
	QueryPacketAcknowledgement(ctx context.Context, height int64, channelid, portid string, seq uint64) (ackRes *chantypes.QueryPacketAcknowledgementResponse, err error)
	QueryPacketReceipt(ctx context.Context, height int64, channelid, portid string, seq uint64) (recRes *chantypes.QueryPacketReceiptResponse, err error)

	// ics 20 - transfer
	QueryDenomTrace(ctx context.Context, denom string) (*transfertypes.DenomTrace, error)
	QueryDenomTraces(ctx context.Context, offset, limit uint64, height int64) ([]transfertypes.DenomTrace, error)
}

type RelayPacket interface {
	Msg(src ChainProvider, srcPortId, srcChanId, dstPortId, dstChanId string) (RelayerMessage, error)
	FetchCommitResponse(ctx context.Context, dst ChainProvider, queryHeight uint64, dstChanId, dstPortId string) error
	Data() []byte
	Seq() uint64
	Timeout() clienttypes.Height
	TimeoutStamp() uint64
}

// KeyOutput contains mnemonic and address of key
type KeyOutput struct {
	Mnemonic string `json:"mnemonic" yaml:"mnemonic"`
	Address  string `json:"address" yaml:"address"`
}

// TimeoutHeightError is used during packet validation to inform the PathProcessor
// that the current chain height has exceeded the packet height timeout so that
// a MsgTimeout can be assembled for the counterparty chain.
type TimeoutHeightError struct {
	latestHeight  uint64
	timeoutHeight uint64
}

func (t *TimeoutHeightError) Error() string {
	return fmt.Sprintf("latest height %d is greater than expiration height: %d", t.latestHeight, t.timeoutHeight)
}

func NewTimeoutHeightError(latestHeight, timeoutHeight uint64) *TimeoutHeightError {
	return &TimeoutHeightError{latestHeight, timeoutHeight}
}

// TimeoutTimestampError is used during packet validation to inform the PathProcessor
// that current block timestamp has exceeded the packet timestamp timeout so that
// a MsgTimeout can be assembled for the counterparty chain.
type TimeoutTimestampError struct {
	latestTimestamp  uint64
	timeoutTimestamp uint64
}

func (t *TimeoutTimestampError) Error() string {
	return fmt.Sprintf("latest block timestamp %d is greater than expiration timestamp: %d", t.latestTimestamp, t.timeoutTimestamp)
}

func NewTimeoutTimestampError(latestTimestamp, timeoutTimestamp uint64) *TimeoutTimestampError {
	return &TimeoutTimestampError{latestTimestamp, timeoutTimestamp}
}

type TimeoutOnCloseError struct {
	msg string
}

func (t *TimeoutOnCloseError) Error() string {
	return fmt.Sprintf("packet timeout on close error: %s", t.msg)
}

func NewTimeoutOnCloseError(msg string) *TimeoutOnCloseError {
	return &TimeoutOnCloseError{msg}
}
