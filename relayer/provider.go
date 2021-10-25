package relayer

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

type TxProvider interface {
	QueryProvider

	CreateClient() (sdk.Msg, error)
	SubmitMisbehavior()
	UpdateClient()
	ConnectionOpenInit()
	ConnectionOpenTry()
	ConnectionOpenAck()
	ConnectionOpenConfirm()
	ChannelOpenInit()
	ChannelOpenTry()
	ChannelOpenAck()
	ChannelOpenConfirm()
	ChannelCloseInit()
	ChannelCloseConfirm()
}

type QueryProvider interface {
	Init() error

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
