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
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

type ProviderConfig interface {
	NewProvider(homepath string, debug bool) (ChainProvider, error)
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
	Events map[string]string
}

type KeyProvider interface {
	CreateKeystore(path string) error
	KeystoreCreated(path string) bool
	AddKey(name string) (output *KeyOutput, err error)
	RestoreKey(name, mnemonic string) (address string, err error)
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
	ConnectionOpenTry(dstQueryProvider QueryProvider, dstHeader ibcexported.Header, srcClientId, dstClientId, srcConnId, dstConnId string) ([]RelayerMessage, error)
	ConnectionOpenAck(dstQueryProvider QueryProvider, dstHeader ibcexported.Header, srcClientId, srcConnId, dstClientId, dstConnId string) ([]RelayerMessage, error)
	ConnectionOpenConfirm(dstQueryProvider QueryProvider, dstHeader ibcexported.Header, dstConnId, srcClientId, srcConnId string) ([]RelayerMessage, error)
	ChannelOpenInit(srcClientId, srcConnId, srcPortId, srcVersion, dstPortId string, order chantypes.Order, dstHeader ibcexported.Header) ([]RelayerMessage, error)
	ChannelOpenTry(dstQueryProvider QueryProvider, dstHeader ibcexported.Header, srcPortId, dstPortId, srcChanId, dstChanId, srcVersion, srcConnectionId, srcClientId string) ([]RelayerMessage, error)
	ChannelOpenAck(dstQueryProvider QueryProvider, dstHeader ibcexported.Header, srcClientId, srcPortId, srcChanId, dstChanId, dstPortId string) ([]RelayerMessage, error)
	ChannelOpenConfirm(dstQueryProvider QueryProvider, dstHeader ibcexported.Header, srcClientId, srcPortId, srcChanId, dstPortId, dstChannId string) ([]RelayerMessage, error)
	ChannelCloseInit(srcPortId, srcChanId string) (RelayerMessage, error)
	ChannelCloseConfirm(dstQueryProvider QueryProvider, dsth int64, dstChanId, dstPortId, srcPortId, srcChanId string) (RelayerMessage, error)

	MsgRelayAcknowledgement(dst ChainProvider, dstChanId, dstPortId, srcChanId, srcPortId string, dsth int64, packet RelayPacket) (RelayerMessage, error)
	MsgTransfer(amount sdk.Coin, dstChainId, dstAddr, srcPortId, srcChanId string, timeoutHeight, timeoutTimestamp uint64) (RelayerMessage, error)
	MsgRelayTimeout(dst ChainProvider, dsth int64, packet RelayPacket, dstChanId, dstPortId, srcChanId, srcPortId string) (RelayerMessage, error)
	MsgRelayRecvPacket(dst ChainProvider, dsth int64, packet RelayPacket, dstChanId, dstPortId, srcChanId, srcPortId string) (RelayerMessage, error)
	MsgUpgradeClient(srcClientId string, consRes *clienttypes.QueryConsensusStateResponse, clientRes *clienttypes.QueryClientStateResponse) (RelayerMessage, error)
	RelayPacketFromSequence(ctx context.Context, src, dst ChainProvider, srch, dsth, seq uint64, dstChanId, dstPortId, srcChanId, srcPortId, srcClientId string) (RelayerMessage, RelayerMessage, error)
	AcknowledgementFromSequence(ctx context.Context, dst ChainProvider, dsth, seq uint64, dstChanId, dstPortId, srcChanId, srcPortId string) (RelayerMessage, error)

	SendMessage(msg RelayerMessage) (*RelayerTxResponse, bool, error)
	SendMessages(msgs []RelayerMessage) (*RelayerTxResponse, bool, error)

	GetLightSignedHeaderAtHeight(h int64) (ibcexported.Header, error)
	GetIBCUpdateHeader(srch int64, dst ChainProvider, dstClientId string) (ibcexported.Header, error)

	ChainId() string
	Type() string
	ProviderConfig() ProviderConfig
	Key() string
	Address() (string, error)
	Timeout() string
	TrustingPeriod() (time.Duration, error)
	WaitForNBlocks(n int64) error
}

// Do we need intermediate types? i.e. can we use the SDK types for both substrate and cosmos?
type QueryProvider interface {
	// chain
	QueryTx(ctx context.Context, hashHex string) (*ctypes.ResultTx, error)
	QueryTxs(ctx context.Context, page, limit int, events []string) ([]*ctypes.ResultTx, error)
	QueryLatestHeight() (int64, error)
	QueryHeaderAtHeight(height int64) (ibcexported.Header, error)

	// bank
	QueryBalance(ctx context.Context, keyName string) (sdk.Coins, error)
	QueryBalanceWithAddress(ctx context.Context, addr string) (sdk.Coins, error)

	// staking
	QueryUnbondingPeriod() (time.Duration, error)

	// ics 02 - client
	QueryClientState(height int64, clientid string) (ibcexported.ClientState, error)
	QueryClientStateResponse(height int64, srcClientId string) (*clienttypes.QueryClientStateResponse, error)
	QueryClientConsensusState(chainHeight int64, clientid string, clientHeight ibcexported.Height) (*clienttypes.QueryConsensusStateResponse, error)
	QueryUpgradedClient(height int64) (*clienttypes.QueryClientStateResponse, error)
	QueryUpgradedConsState(height int64) (*clienttypes.QueryConsensusStateResponse, error)
	QueryConsensusState(height int64) (ibcexported.ConsensusState, int64, error)
	QueryClients() (clienttypes.IdentifiedClientStates, error)
	AutoUpdateClient(dst ChainProvider, thresholdTime time.Duration, srcClientId, dstClientId string) (time.Duration, error)
	FindMatchingClient(counterparty ChainProvider, clientState ibcexported.ClientState) (string, bool)

	// ics 03 - connection
	QueryConnection(height int64, connectionid string) (*conntypes.QueryConnectionResponse, error)
	QueryConnections() (conns []*conntypes.IdentifiedConnection, err error)
	QueryConnectionsUsingClient(height int64, clientid string) (*conntypes.QueryConnectionsResponse, error)
	GenerateConnHandshakeProof(height int64, clientId, connId string) (clientState ibcexported.ClientState,
		clientStateProof []byte, consensusProof []byte, connectionProof []byte,
		connectionProofHeight ibcexported.Height, err error)
	NewClientState(dstUpdateHeader ibcexported.Header, dstTrustingPeriod, dstUbdPeriod time.Duration, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour bool) (ibcexported.ClientState, error)

	// ics 04 - channel
	QueryChannel(height int64, channelid, portid string) (chanRes *chantypes.QueryChannelResponse, err error)
	QueryChannelClient(height int64, channelid, portid string) (*clienttypes.IdentifiedClientState, error)
	QueryConnectionChannels(height int64, connectionid string) ([]*chantypes.IdentifiedChannel, error)
	QueryChannels() ([]*chantypes.IdentifiedChannel, error)
	QueryPacketCommitments(height uint64, channelid, portid string) (commitments *chantypes.QueryPacketCommitmentsResponse, err error)
	QueryPacketAcknowledgements(height uint64, channelid, portid string) (acknowledgements []*chantypes.PacketState, err error)
	QueryUnreceivedPackets(height uint64, channelid, portid string, seqs []uint64) ([]uint64, error)
	QueryUnreceivedAcknowledgements(height uint64, channelid, portid string, seqs []uint64) ([]uint64, error)
	QueryNextSeqRecv(height int64, channelid, portid string) (recvRes *chantypes.QueryNextSequenceReceiveResponse, err error)
	QueryPacketCommitment(height int64, channelid, portid string, seq uint64) (comRes *chantypes.QueryPacketCommitmentResponse, err error)
	QueryPacketAcknowledgement(height int64, channelid, portid string, seq uint64) (ackRes *chantypes.QueryPacketAcknowledgementResponse, err error)
	QueryPacketReceipt(height int64, channelid, portid string, seq uint64) (recRes *chantypes.QueryPacketReceiptResponse, err error)

	// ics 20 - transfer
	QueryDenomTrace(denom string) (*transfertypes.DenomTrace, error)
	QueryDenomTraces(offset, limit uint64, height int64) ([]transfertypes.DenomTrace, error)
}

type RelayPacket interface {
	Msg(src ChainProvider, srcPortId, srcChanId, dstPortId, dstChanId string) (RelayerMessage, error)
	FetchCommitResponse(dst ChainProvider, queryHeight uint64, dstChanId, dstPortId string) error
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
