package icon

import (
	"context"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/provider"

	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
)

func (icp *IconProvider) BlockTime(ctx context.Context, height int64) (time.Time, error)

func (icp *IconProvider) QueryTx(ctx context.Context, hashHex string) (*provider.RelayerTxResponse, error) {
	return nil, nil
}
func (icp *IconProvider) QueryTxs(ctx context.Context, page, limit int, events []string) ([]*provider.RelayerTxResponse, error) {
	return nil, nil
}
func (icp *IconProvider) QueryLatestHeight(ctx context.Context) (int64, error) {
	return 0, nil
}
func (icp *IconProvider) QueryIBCHeader(ctx context.Context, h int64) (provider.IBCHeader, error) {
	return IconIBCHeader{}, nil
}
func (icp *IconProvider) QuerySendPacket(ctx context.Context, srcChanID, srcPortID string, sequence uint64) (provider.PacketInfo, error) {
	return provider.PacketInfo{}, nil
}
func (icp *IconProvider) QueryRecvPacket(ctx context.Context, dstChanID, dstPortID string, sequence uint64) (provider.PacketInfo, error) {
	return provider.PacketInfo{}, nil
}

func (icp *IconProvider) QueryBalance(ctx context.Context, keyName string) (sdk.Coins, error) {
	return sdk.Coins{}, nil
}
func (icp *IconProvider) QueryBalanceWithAddress(ctx context.Context, addr string) (sdk.Coins, error) {
	return sdk.Coins{}, nil
}
func (icp *IconProvider) QueryUnbondingPeriod(context.Context) (time.Duration, error) {
	return 0, nil
}
func (icp *IconProvider) QueryClientState(ctx context.Context, height int64, clientid string) (ibcexported.ClientState, error) {
	return nil, nil
}
func (icp *IconProvider) QueryClientStateResponse(ctx context.Context, height int64, srcClientId string) (*clienttypes.QueryClientStateResponse, error) {
	return nil, nil
}
func (icp *IconProvider) QueryClientConsensusState(ctx context.Context, chainHeight int64, clientid string, clientHeight ibcexported.Height) (*clienttypes.QueryConsensusStateResponse, error) {
	return nil, nil
}
func (icp *IconProvider) QueryUpgradedClient(ctx context.Context, height int64) (*clienttypes.QueryClientStateResponse, error) {
	return nil, nil
}
func (icp *IconProvider) QueryUpgradedConsState(ctx context.Context, height int64) (*clienttypes.QueryConsensusStateResponse, error) {
	return nil, nil
}
func (icp *IconProvider) QueryConsensusState(ctx context.Context, height int64) (ibcexported.ConsensusState, int64, error) {
	return nil, 0, nil
}
func (icp *IconProvider) QueryClients(ctx context.Context) (clienttypes.IdentifiedClientStates, error) {
	return nil, nil
}
func (icp *IconProvider) QueryConnection(ctx context.Context, height int64, connectionid string) (*conntypes.QueryConnectionResponse, error) {
	return nil, nil
}
func (icp *IconProvider) QueryConnections(ctx context.Context) (conns []*conntypes.IdentifiedConnection, err error) {
	return nil, nil
}
func (icp *IconProvider) QueryConnectionsUsingClient(ctx context.Context, height int64, clientid string) (*conntypes.QueryConnectionsResponse, error) {
	return nil, nil
}
func (icp *IconProvider) GenerateConnHandshakeProof(ctx context.Context, height int64, clientId, connId string) (clientState ibcexported.ClientState,
	clientStateProof []byte, consensusProof []byte, connectionProof []byte,
	connectionProofHeight ibcexported.Height, err error) {
	return
}

// ics 04 - channel
func (icp *IconProvider) QueryChannel(ctx context.Context, height int64, channelid, portid string) (chanRes *chantypes.QueryChannelResponse, err error) {
	return nil, nil
}
func (icp *IconProvider) QueryChannelClient(ctx context.Context, height int64, channelid, portid string) (*clienttypes.IdentifiedClientState, error) {
	return nil, nil
}
func (icp *IconProvider) QueryConnectionChannels(ctx context.Context, height int64, connectionid string) ([]*chantypes.IdentifiedChannel, error) {
	return nil, nil

}
func (icp *IconProvider) QueryChannels(ctx context.Context) ([]*chantypes.IdentifiedChannel, error) {
	return nil, nil

}
func (icp *IconProvider) QueryPacketCommitments(ctx context.Context, height uint64, channelid, portid string) (commitments *chantypes.QueryPacketCommitmentsResponse, err error) {
	return nil, nil

}
func (icp *IconProvider) QueryPacketAcknowledgements(ctx context.Context, height uint64, channelid, portid string) (acknowledgements []*chantypes.PacketState, err error) {
	return nil, nil

}
func (icp *IconProvider) QueryUnreceivedPackets(ctx context.Context, height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	return nil, nil
}

func (icp *IconProvider) QueryUnreceivedAcknowledgements(ctx context.Context, height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	return nil, nil
}

func (icp *IconProvider) QueryNextSeqRecv(ctx context.Context, height int64, channelid, portid string) (recvRes *chantypes.QueryNextSequenceReceiveResponse, err error) {
	return nil, nil
}

func (icp *IconProvider) QueryPacketCommitment(ctx context.Context, height int64, channelid, portid string, seq uint64) (comRes *chantypes.QueryPacketCommitmentResponse, err error) {
	return nil, nil
}

func (icp *IconProvider) QueryPacketAcknowledgement(ctx context.Context, height int64, channelid, portid string, seq uint64) (ackRes *chantypes.QueryPacketAcknowledgementResponse, err error) {
	return nil, nil
}

func (icp *IconProvider) QueryPacketReceipt(ctx context.Context, height int64, channelid, portid string, seq uint64) (recRes *chantypes.QueryPacketReceiptResponse, err error) {
	return nil, nil
}

// ics 20 - transfer
func (icp *IconProvider) QueryDenomTrace(ctx context.Context, denom string) (*transfertypes.DenomTrace, error) {
	return nil, nil
}

func (icp *IconProvider) QueryDenomTraces(ctx context.Context, offset, limit uint64, height int64) ([]transfertypes.DenomTrace, error) {
	return nil, nil
}
