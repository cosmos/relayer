package provider

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v2/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v2/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v2/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v2/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v2/modules/core/exported"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

type ChainType int

const (
	Cosmos    ChainType = iota // 0
	Substrate                  // 1
)

type RelayerMessage interface {
	Type() ChainType
}

type RelayerTxResponse struct {
	Code  int
	Error string
}

type KeyProvider interface {
	// TODO: figure out what is needed for key provider interface
	// to offer same CLI UX to users of each chain. Is this practical?
}

type TxProvider interface {
	QueryProvider
	KeyProvider

	Init() error
	CreateClient(dstHeader ibcexported.Header) (*RelayerMessage, error)
	SubmitMisbehavior( /*TBD*/ ) (*RelayerMessage, error)
	UpdateClient(dstHeader ibcexported.Header) (*RelayerMessage, error)
	ConnectionOpenInit(srcClientId, dstClientId string, dstHeader ibcexported.Header) (*RelayerMessage, error)
	ConnectionOpenTry(dstQueryProvider QueryProvider, dstHeader ibcexported.Header, srcClientId, dstClientId, srcConnId, dstConnId string) (*RelayerMessage, error)
	ConnectionOpenAck(dstQueryProvider QueryProvider, dstHeader ibcexported.Header, srcConnId, dstConnId string) (*RelayerMessage, error)
	ConnectionOpenConfirm(dstQueryProvider QueryProvider, dstHeader ibcexported.Header, srcConnId string) (*RelayerMessage, error)
	ChannelOpenInit(srcPortId, srcVersion string, order chantypes.Order, dstHeader ibcexported.Header) (*RelayerMessage, error)
	ChannelOpenTry(dstQueryProvider QueryProvider, dstHeader ibcexported.Header, srcPortId, dstPortId, srcChanId, dstChanId, srcVersion, srcConnectionId string) (*RelayerMessage, error)
	ChannelOpenAck(dstQueryProvider QueryProvider, dstHeader ibcexported.Header, srcPortId, srcChanId, dstChanId string) (*RelayerMessage, error)
	ChannelOpenConfirm(dstQueryProvider QueryProvider, dstHeader ibcexported.Header, srcPortId, srcChanId string) (*RelayerMessage, error)
	ChannelCloseInit(srcPortId, srcChanId string) (*RelayerMessage, error)
	ChannelCloseConfirm(dstQueryProvider QueryProvider, srcPortId, srcChanId string) (*RelayerMessage, error)
	SendMessage(*RelayerMessage) (*RelayerTxResponse, error)
	SendMessages([]*RelayerMessage) (*RelayerTxResponse, error)
}

// Do we need intermediate types? i.e. can we use the SDK types for both substrate and cosmos?
//
type QueryProvider interface {
	// chain
	QueryTx(hashHex string) (*ctypes.ResultTx, error)
	QueryTxs(height uint64, events []string) ([]*ctypes.ResultTx, error)
	QueryLatestHeight() (int64, error)

	// bank
	QueryBalances(addr string) (sdk.Coins, error)

	// staking
	QueryUnbondingPeriod() (time.Duration, error)

	// ics 02 - client
	QueryClientState(height int64, clientid string) (*clienttypes.QueryClientStateResponse, error)
	QueryClientConsensusState(chainHeight int64, clientid string, clientHeight ibcexported.Height) (*clienttypes.QueryConsensusStateResponse, error)
	QueryUpgradedClient(height int64) (*clienttypes.QueryClientStateResponse, error)
	QueryUpgradedConsState(height int64) (*clienttypes.QueryConsensusStateResponse, error)
	QueryConsensusState(height int64) (ibcexported.ConsensusState, int64, error)
	QueryClients() ([]*clienttypes.IdentifiedClientState, error)

	// ics 03 - connection
	QueryConnection(height int64, connectionid string) (*conntypes.QueryConnectionResponse, error)
	QueryConnections() (conns []*conntypes.IdentifiedConnection, err error)
	QueryConnectionsUsingClient(height int64, clientid string) (clientConns []string, err error)
	GenerateConnHandshakeProof(height int64) (clientState ibcexported.ClientState,
		clientStateProof []byte, consensusProof []byte, connectionProof []byte,
		connectionProofHeight ibcexported.Height, err error)

	// ics 04 - channel
	QueryChannel(height int64, channelid, portid string) (chanRes *chantypes.QueryChannelResponse, err error)
	QueryChannelClient(height int64, channelid, portid string) (*clienttypes.IdentifiedClientState, error)
	QueryConnectionChannels(height int64, connectionid string) ([]*chantypes.IdentifiedChannel, error)
	QueryChannels() ([]*chantypes.IdentifiedChannel, error)
	QueryPacketCommitments(height uint64, channelid, portid string) (commitments []*chantypes.PacketState, err error)
	QueryPacketAcknowledgements(height uint64, channelid, portid string) (acknowledgements []*chantypes.PacketState, err error)
	QueryUnreceivedPackets(height uint64, channelid, portid string, seqs []uint64) ([]uint64, error)
	QueryUnreceivedAcknowledgements(height uint64, channelid, portid string, seqs []uint64) ([]uint64, error)
	QueryNextSeqRecv(height int64, channelid, portid string) (recvRes *chantypes.QueryNextSequenceReceiveResponse, err error)
	QueryPacketCommitment(height int64, channelid, portid string, seq uint64) (comRes *chantypes.QueryPacketCommitmentResponse, err error)
	QueryPacketAcknowledgement(height int64, channelid, portid string, seq uint64) (ackRes *chantypes.QueryPacketAcknowledgementResponse, err error)
	QueryPacketReceipt(height int64, channelid, portid string, seq uint64) (recRes *chantypes.QueryPacketReceiptResponse, err error)

	// ics 20 - transfer
	QueryDenomTrace(denom string) (*transfertypes.DenomTrace, error)
	QueryDenomTraces(offset, limit uint64, height int64) ([]*transfertypes.DenomTrace, error)
}
