package provider

import (
	"context"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v3/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ProviderConfig interface {
	NewProvider(log *zap.Logger, homepath string, debug bool) (ChainProvider, error)
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
	CreateClient(clientState ibcexported.ClientState, dstHeader ibcexported.Header) (RelayerMessage, error)
	SubmitMisbehavior( /*TODO TBD*/ ) (RelayerMessage, error)
	UpdateClient(srcClientId string, dstHeader ibcexported.Header) (RelayerMessage, error)
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

	MsgRelayAcknowledgement(ctx context.Context, dst ChainProvider, dstChanId, dstPortId, srcChanId, srcPortId string, dsth int64, packet RelayPacket) (RelayerMessage, error)
	MsgTransfer(amount sdk.Coin, dstChainId, dstAddr, srcPortId, srcChanId string, timeoutHeight, timeoutTimestamp uint64) (RelayerMessage, error)
	MsgRelayTimeout(ctx context.Context, dst ChainProvider, dsth int64, packet RelayPacket, dstChanId, dstPortId, srcChanId, srcPortId string, order chantypes.Order) (RelayerMessage, error)
	MsgRelayRecvPacket(ctx context.Context, dst ChainProvider, dsth int64, packet RelayPacket, dstChanId, dstPortId, srcChanId, srcPortId string) (RelayerMessage, error)
	MsgUpgradeClient(srcClientId string, consRes *clienttypes.QueryConsensusStateResponse, clientRes *clienttypes.QueryClientStateResponse) (RelayerMessage, error)
	RelayPacketFromSequence(ctx context.Context, src, dst ChainProvider, srch, dsth, seq uint64, dstChanId, dstPortId, dstClientId, srcChanId, srcPortId, srcClientId string, order chantypes.Order) (RelayerMessage, RelayerMessage, error)
	AcknowledgementFromSequence(ctx context.Context, dst ChainProvider, dsth, seq uint64, dstChanId, dstPortId, srcChanId, srcPortId string) (RelayerMessage, error)

	SendMessage(ctx context.Context, msg RelayerMessage) (*RelayerTxResponse, bool, error)
	SendMessages(ctx context.Context, msgs []RelayerMessage) (*RelayerTxResponse, bool, error)

	GetLightSignedHeaderAtHeight(ctx context.Context, h int64) (ibcexported.Header, error)
	GetIBCUpdateHeader(ctx context.Context, srch int64, dst ChainProvider, dstClientId string) (ibcexported.Header, error)

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
	FindMatchingClient(ctx context.Context, counterparty ChainProvider, clientState ibcexported.ClientState) (string, bool)

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
