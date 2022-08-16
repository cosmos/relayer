package substrate

import (
	"context"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	querytypes "github.com/cosmos/cosmos-sdk/types/query"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	transfertypes "github.com/cosmos/ibc-go/v5/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v5/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v5/modules/core/exported"
	tmclient "github.com/cosmos/ibc-go/v5/modules/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	abci "github.com/tendermint/tendermint/abci/types"
	"go.uber.org/zap"
)

var _ provider.QueryProvider = &SubstrateProvider{}

// queryIBCMessages returns an array of IBC messages given a tag
func (cc *SubstrateProvider) queryIBCMessages(ctx context.Context, log *zap.Logger, page, limit int, query string) ([]ibcMessage, error) {
	return []ibcMessage{}, nil
}

// QueryTx takes a transaction hash and returns the transaction
func (cc *SubstrateProvider) QueryTx(ctx context.Context, hashHex string) (*provider.RelayerTxResponse, error) {
	return &provider.RelayerTxResponse{}, nil
}

// QueryTxs returns an array of transactions given a tag
func (cc *SubstrateProvider) QueryTxs(ctx context.Context, page, limit int, events []string) ([]*provider.RelayerTxResponse, error) {
	return []*provider.RelayerTxResponse{}, nil
}

// parseEventsFromResponseDeliverTx parses the events from a ResponseDeliverTx and builds a slice
// of provider.RelayerEvent's.
func parseEventsFromResponseDeliverTx(resp abci.ResponseDeliverTx) []provider.RelayerEvent {
	return []provider.RelayerEvent{}
}

// QueryBalance returns the amount of coins in the relayer account
func (cc *SubstrateProvider) QueryBalance(ctx context.Context, keyName string) (sdk.Coins, error) {
	return sdk.Coins{}, nil
}

// QueryBalanceWithAddress returns the amount of coins in the relayer account with address as input
// TODO add pagination support
func (cc *SubstrateProvider) QueryBalanceWithAddress(ctx context.Context, address string) (sdk.Coins, error) {
	return sdk.Coins{}, nil
}

// QueryUnbondingPeriod returns the unbonding period of the chain
func (cc *SubstrateProvider) QueryUnbondingPeriod(ctx context.Context) (time.Duration, error) {
	return 0, nil
}

// QueryTendermintProof performs an ABCI query with the given key and returns
// the value of the query, the proto encoded merkle proof, and the height of
// the Tendermint block containing the state root. The desired tendermint height
// to perform the query should be set in the client context. The query will be
// performed at one below this height (at the IAVL version) in order to obtain
// the correct merkle proof. Proof queries at height less than or equal to 2 are
// not supported. Queries with a client context height of 0 will perform a query
// at the latest state available.
// Issue: https://github.com/cosmos/cosmos-sdk/issues/6567
func (cc *SubstrateProvider) QueryTendermintProof(ctx context.Context, height int64, key []byte) ([]byte, []byte, clienttypes.Height, error) {
	return nil, nil, clienttypes.Height{}, nil
}

// QueryClientStateResponse retrieves the latest consensus state for a client in state at a given height
func (cc *SubstrateProvider) QueryClientStateResponse(ctx context.Context, height int64, srcClientId string) (*clienttypes.QueryClientStateResponse, error) {
	return &clienttypes.QueryClientStateResponse{}, nil
}

// QueryClientState retrieves the latest consensus state for a client in state at a given height
// and unpacks it to exported client state interface
func (cc *SubstrateProvider) QueryClientState(ctx context.Context, height int64, clientid string) (ibcexported.ClientState, error) {
	return nil, nil
}

// QueryClientConsensusState retrieves the latest consensus state for a client in state at a given height
func (cc *SubstrateProvider) QueryClientConsensusState(ctx context.Context, chainHeight int64, clientid string, clientHeight ibcexported.Height) (*clienttypes.QueryConsensusStateResponse, error) {
	return &clienttypes.QueryConsensusStateResponse{}, nil
}

// QueryUpgradeProof performs an abci query with the given key and returns the proto encoded merkle proof
// for the query and the height at which the proof will succeed on a tendermint verifier.
func (cc *SubstrateProvider) QueryUpgradeProof(ctx context.Context, key []byte, height uint64) ([]byte, clienttypes.Height, error) {
	return nil, clienttypes.Height{}, nil
}

// QueryUpgradedClient returns upgraded client info
func (cc *SubstrateProvider) QueryUpgradedClient(ctx context.Context, height int64) (*clienttypes.QueryClientStateResponse, error) {
	return &clienttypes.QueryClientStateResponse{}, nil
}

// QueryUpgradedConsState returns upgraded consensus state and height of client
func (cc *SubstrateProvider) QueryUpgradedConsState(ctx context.Context, height int64) (*clienttypes.QueryConsensusStateResponse, error) {
	return &clienttypes.QueryConsensusStateResponse{}, nil
}

// QueryConsensusState returns a consensus state for a given chain to be used as a
// client in another chain, fetches latest height when passed 0 as arg
func (cc *SubstrateProvider) QueryConsensusState(ctx context.Context, height int64) (ibcexported.ConsensusState, int64, error) {
	return nil, height, nil
}

// QueryClients queries all the clients!
// TODO add pagination support
func (cc *SubstrateProvider) QueryClients(ctx context.Context) (clienttypes.IdentifiedClientStates, error) {
	return clienttypes.IdentifiedClientStates{}, nil
}

// QueryConnection returns the remote end of a given connection
func (cc *SubstrateProvider) QueryConnection(ctx context.Context, height int64, connectionid string) (*conntypes.QueryConnectionResponse, error) {
	return &conntypes.QueryConnectionResponse{}, nil
}

func (cc *SubstrateProvider) queryConnectionABCI(ctx context.Context, height int64, connectionID string) (*conntypes.QueryConnectionResponse, error) {
	return &conntypes.QueryConnectionResponse{}, nil
}

// QueryConnections gets any connections on a chain
// TODO add pagination support
func (cc *SubstrateProvider) QueryConnections(ctx context.Context) (conns []*conntypes.IdentifiedConnection, err error) {
	return []*conntypes.IdentifiedConnection{}, err
}

// QueryConnectionsUsingClient gets any connections that exist between chain and counterparty
// TODO add pagination support
func (cc *SubstrateProvider) QueryConnectionsUsingClient(ctx context.Context, height int64, clientid string) (*conntypes.QueryConnectionsResponse, error) {
	return &conntypes.QueryConnectionsResponse{}, nil
}

// GenerateConnHandshakeProof generates all the proofs needed to prove the existence of the
// connection state on this chain. A counterparty should use these generated proofs.
func (cc *SubstrateProvider) GenerateConnHandshakeProof(ctx context.Context, height int64, clientId, connId string) (
	clientState ibcexported.ClientState, clientStateProof []byte, consensusProof []byte, connectionProof []byte, connectionProofHeight ibcexported.Height, err error,
) {
	return nil, nil, nil, nil, nil, nil
}

// QueryChannel returns the channel associated with a channelID
func (cc *SubstrateProvider) QueryChannel(ctx context.Context, height int64, channelid, portid string) (chanRes *chantypes.QueryChannelResponse, err error) {
	return &chantypes.QueryChannelResponse{}, nil
}

func (cc *SubstrateProvider) queryChannelABCI(ctx context.Context, height int64, portID, channelID string) (*chantypes.QueryChannelResponse, error) {
	return &chantypes.QueryChannelResponse{}, nil
}

// QueryChannelClient returns the client state of the client supporting a given channel
func (cc *SubstrateProvider) QueryChannelClient(ctx context.Context, height int64, channelid, portid string) (*clienttypes.IdentifiedClientState, error) {
	return &clienttypes.IdentifiedClientState{}, nil
}

// QueryConnectionChannels queries the channels associated with a connection
func (cc *SubstrateProvider) QueryConnectionChannels(ctx context.Context, height int64, connectionid string) ([]*chantypes.IdentifiedChannel, error) {
	return []*chantypes.IdentifiedChannel{}, nil
}

// QueryChannels returns all the channels that are registered on a chain
// TODO add pagination support
func (cc *SubstrateProvider) QueryChannels(ctx context.Context) ([]*chantypes.IdentifiedChannel, error) {
	return []*chantypes.IdentifiedChannel{}, nil
}

// QueryPacketCommitments returns an array of packet commitments
// TODO add pagination support
func (cc *SubstrateProvider) QueryPacketCommitments(ctx context.Context, height uint64, channelid, portid string) (commitments *chantypes.QueryPacketCommitmentsResponse, err error) {
	return &chantypes.QueryPacketCommitmentsResponse{}, nil
}

// QueryPacketAcknowledgements returns an array of packet acks
// TODO add pagination support
func (cc *SubstrateProvider) QueryPacketAcknowledgements(ctx context.Context, height uint64, channelid, portid string) (acknowledgements []*chantypes.PacketState, err error) {
	return []*chantypes.PacketState{}, nil
}

// QueryUnreceivedPackets returns a list of unrelayed packet commitments
func (cc *SubstrateProvider) QueryUnreceivedPackets(ctx context.Context, height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	return []uint64{}, nil
}

func sendPacketQuery(channelID string, portID string, seq uint64) string {
	return ""
}

func writeAcknowledgementQuery(channelID string, portID string, seq uint64) string {
	return ""
}

func (cc *SubstrateProvider) QuerySendPacket(
	ctx context.Context,
	srcChanID,
	srcPortID string,
	sequence uint64,
) (provider.PacketInfo, error) {
	return provider.PacketInfo{}, nil
}

func (cc *SubstrateProvider) QueryRecvPacket(
	ctx context.Context,
	dstChanID,
	dstPortID string,
	sequence uint64,
) (provider.PacketInfo, error) {
	return provider.PacketInfo{}, nil
}

// QueryUnreceivedAcknowledgements returns a list of unrelayed packet acks
func (cc *SubstrateProvider) QueryUnreceivedAcknowledgements(ctx context.Context, height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	return []uint64{}, nil
}

// QueryNextSeqRecv returns the next seqRecv for a configured channel
func (cc *SubstrateProvider) QueryNextSeqRecv(ctx context.Context, height int64, channelid, portid string) (recvRes *chantypes.QueryNextSequenceReceiveResponse, err error) {
	return &chantypes.QueryNextSequenceReceiveResponse{}, nil
}

// QueryPacketCommitment returns the packet commitment proof at a given height
func (cc *SubstrateProvider) QueryPacketCommitment(ctx context.Context, height int64, channelid, portid string, seq uint64) (comRes *chantypes.QueryPacketCommitmentResponse, err error) {
	return &chantypes.QueryPacketCommitmentResponse{}, nil
}

// QueryPacketAcknowledgement returns the packet ack proof at a given height
func (cc *SubstrateProvider) QueryPacketAcknowledgement(ctx context.Context, height int64, channelid, portid string, seq uint64) (ackRes *chantypes.QueryPacketAcknowledgementResponse, err error) {
	return &chantypes.QueryPacketAcknowledgementResponse{}, nil
}

// QueryPacketReceipt returns the packet receipt proof at a given height
func (cc *SubstrateProvider) QueryPacketReceipt(ctx context.Context, height int64, channelid, portid string, seq uint64) (recRes *chantypes.QueryPacketReceiptResponse, err error) {
	return &chantypes.QueryPacketReceiptResponse{}, nil
}

func (cc *SubstrateProvider) QueryLatestHeight(ctx context.Context) (int64, error) {
	return 0, nil
}

// QueryHeaderAtHeight returns the header at a given height
func (cc *SubstrateProvider) QueryHeaderAtHeight(ctx context.Context, height int64) (ibcexported.Header, error) {
	return &tmclient.Header{}, nil
}

// QueryDenomTrace takes a denom from IBC and queries the information about it
func (cc *SubstrateProvider) QueryDenomTrace(ctx context.Context, denom string) (*transfertypes.DenomTrace, error) {
	return &transfertypes.DenomTrace{}, nil
}

// QueryDenomTraces returns all the denom traces from a given chain
// TODO add pagination support
func (cc *SubstrateProvider) QueryDenomTraces(ctx context.Context, offset, limit uint64, height int64) ([]transfertypes.DenomTrace, error) {
	return []transfertypes.DenomTrace{}, nil
}

func (cc *SubstrateProvider) QueryStakingParams(ctx context.Context) (*stakingtypes.Params, error) {
	return &stakingtypes.Params{}, nil
}

func DefaultPageRequest() *querytypes.PageRequest {
	return &querytypes.PageRequest{}
}

func (cc *SubstrateProvider) QueryConsensusStateABCI(ctx context.Context, clientID string, height ibcexported.Height) (*clienttypes.QueryConsensusStateResponse, error) {
	return &clienttypes.QueryConsensusStateResponse{}, nil
}
