package archway

import (
	"context"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

func (ap *ArchwayProvider) BlockTime(ctx context.Context, height int64) (time.Time, error) {
	return time.Now(), nil
}
func (ap *ArchwayProvider) QueryTx(ctx context.Context, hashHex string) (*provider.RelayerTxResponse, error) {
	return nil, nil
}
func (ap *ArchwayProvider) QueryTxs(ctx context.Context, page, limit int, events []string) ([]*provider.RelayerTxResponse, error) {
	return nil, nil
}
func (ap *ArchwayProvider) QueryLatestHeight(ctx context.Context) (int64, error) {
	return 0, nil
}

// QueryIBCHeader returns the IBC compatible block header at a specific height.
func (ap *ArchwayProvider) QueryIBCHeader(ctx context.Context, h int64) (provider.IBCHeader, error) {
	return nil, nil
}

// query packet info for sequence
func (ap *ArchwayProvider) QuerySendPacket(ctx context.Context, srcChanID, srcPortID string, sequence uint64) (provider.PacketInfo, error) {
	return provider.PacketInfo{}, nil
}
func (ap *ArchwayProvider) QueryRecvPacket(ctx context.Context, dstChanID, dstPortID string, sequence uint64) (provider.PacketInfo, error) {
	return provider.PacketInfo{}, nil
}

// bank
func (ap *ArchwayProvider) QueryBalance(ctx context.Context, keyName string) (sdk.Coins, error) {
	return nil, nil
}
func (ap *ArchwayProvider) QueryBalanceWithAddress(ctx context.Context, addr string) (sdk.Coins, error) {
	return nil, nil
}

// staking
func (ap *ArchwayProvider) QueryUnbondingPeriod(context.Context) (time.Duration, error) {
	return 0, nil
}

// ics 02 - client
func (ap *ArchwayProvider) QueryClientState(ctx context.Context, height int64, clientid string) (ibcexported.ClientState, error) {
	return nil, nil
}
func (ap *ArchwayProvider) QueryClientStateResponse(ctx context.Context, height int64, srcClientId string) (*clienttypes.QueryClientStateResponse, error) {
	return nil, nil
}
func (ap *ArchwayProvider) QueryClientConsensusState(ctx context.Context, chainHeight int64, clientid string, clientHeight ibcexported.Height) (*clienttypes.QueryConsensusStateResponse, error) {
	return nil, nil
}
func (ap *ArchwayProvider) QueryUpgradedClient(ctx context.Context, height int64) (*clienttypes.QueryClientStateResponse, error) {
	return nil, nil
}
func (ap *ArchwayProvider) QueryUpgradedConsState(ctx context.Context, height int64) (*clienttypes.QueryConsensusStateResponse, error) {
	return nil, nil
}
func (ap *ArchwayProvider) QueryConsensusState(ctx context.Context, height int64) (ibcexported.ConsensusState, int64, error) {
	return nil, 0, nil
}
func (ap *ArchwayProvider) QueryClients(ctx context.Context) (clienttypes.IdentifiedClientStates, error) {
	return nil, nil
}

// ics 03 - connection
func (ap *ArchwayProvider) QueryConnection(ctx context.Context, height int64, connectionid string) (*conntypes.QueryConnectionResponse, error) {
	return nil, nil
}
func (ap *ArchwayProvider) QueryConnections(ctx context.Context) (conns []*conntypes.IdentifiedConnection, err error) {
	return nil, nil
}
func (ap *ArchwayProvider) QueryConnectionsUsingClient(ctx context.Context, height int64, clientid string) (*conntypes.QueryConnectionsResponse, error) {
	return nil, nil
}
func (ap *ArchwayProvider) GenerateConnHandshakeProof(ctx context.Context, height int64, clientId, connId string) (clientState ibcexported.ClientState,
	clientStateProof []byte, consensusProof []byte, connectionProof []byte,
	connectionProofHeight ibcexported.Height, err error) {
	return nil, nil, nil, nil, nil, nil
}

// ics 04 - channel
func (ap *ArchwayProvider) QueryChannel(ctx context.Context, height int64, channelid, portid string) (chanRes *chantypes.QueryChannelResponse, err error) {
	return nil, nil
}
func (ap *ArchwayProvider) QueryChannelClient(ctx context.Context, height int64, channelid, portid string) (*clienttypes.IdentifiedClientState, error) {
	return nil, nil
}
func (ap *ArchwayProvider) QueryConnectionChannels(ctx context.Context, height int64, connectionid string) ([]*chantypes.IdentifiedChannel, error) {
	return nil, nil
}
func (ap *ArchwayProvider) QueryChannels(ctx context.Context) ([]*chantypes.IdentifiedChannel, error) {
	return nil, nil
}
func (ap *ArchwayProvider) QueryPacketCommitments(ctx context.Context, height uint64, channelid, portid string) (commitments *chantypes.QueryPacketCommitmentsResponse, err error) {
	return nil, nil
}
func (ap *ArchwayProvider) QueryPacketAcknowledgements(ctx context.Context, height uint64, channelid, portid string) (acknowledgements []*chantypes.PacketState, err error) {
	return nil, nil
}
func (ap *ArchwayProvider) QueryUnreceivedPackets(ctx context.Context, height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	return nil, nil
}
func (ap *ArchwayProvider) QueryUnreceivedAcknowledgements(ctx context.Context, height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	return nil, nil
}
func (ap *ArchwayProvider) QueryNextSeqRecv(ctx context.Context, height int64, channelid, portid string) (recvRes *chantypes.QueryNextSequenceReceiveResponse, err error) {
	return nil, nil
}
func (ap *ArchwayProvider) QueryPacketCommitment(ctx context.Context, height int64, channelid, portid string, seq uint64) (comRes *chantypes.QueryPacketCommitmentResponse, err error) {
	return nil, nil
}
func (ap *ArchwayProvider) QueryPacketAcknowledgement(ctx context.Context, height int64, channelid, portid string, seq uint64) (ackRes *chantypes.QueryPacketAcknowledgementResponse, err error) {
	return nil, nil
}
func (ap *ArchwayProvider) QueryPacketReceipt(ctx context.Context, height int64, channelid, portid string, seq uint64) (recRes *chantypes.QueryPacketReceiptResponse, err error) {
	return nil, nil
}

// ics 20 - transfer
func (ap *ArchwayProvider) QueryDenomTrace(ctx context.Context, denom string) (*transfertypes.DenomTrace, error) {
	return nil, nil
}
func (ap *ArchwayProvider) QueryDenomTraces(ctx context.Context, offset, limit uint64, height int64) ([]transfertypes.DenomTrace, error) {
	return nil, nil
}
