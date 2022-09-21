package substrate

import (
	"context"
	"fmt"
	"math"
	"time"

	beefyclienttypes "github.com/ComposableFi/ics11-beefy/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v5/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v5/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v5/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

var _ provider.QueryProvider = &SubstrateProvider{}

func (sp *SubstrateProvider) QueryTx(ctx context.Context, hashHex string) (*provider.RelayerTxResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryTxs(ctx context.Context, page, limit int, events []string) ([]*provider.RelayerTxResponse, error) {
	//TODO implement me
	panic("implement me")
}

func latestParaHeightHeight(paraHeaders []*beefyclienttypes.ParachainHeader) (int64, error) {
	fmt.Printf("length of parachain headers %v \n", len(paraHeaders))
	header, err := beefyclienttypes.DecodeParachainHeader(paraHeaders[0].ParachainHeader)
	if err != nil {
		return 0, err
	}

	latestHeight := int64(header.Number)
	for _, h := range paraHeaders {
		decodedHeader, err := beefyclienttypes.DecodeParachainHeader(h.ParachainHeader)
		if err != nil {
			return 0, err
		}

		if int64(decodedHeader.Number) < latestHeight {
			latestHeight = int64(decodedHeader.Number)
		}
	}
	return latestHeight, nil
}

func (sp *SubstrateProvider) QueryLatestHeight(ctx context.Context) (int64, error) {
	signedHash, err := sp.RelayerRPCClient.RPC.Beefy.GetFinalizedHead()
	if err != nil {
		return 0, err
	}

	block, err := sp.RelayerRPCClient.RPC.Chain.GetBlock(signedHash)
	if err != nil {
		return 0, err
	}

	header, err := sp.constructBeefyHeader(signedHash, nil)
	if err != nil {
		return 0, err
	}

	decodedHeader, err := beefyclienttypes.DecodeParachainHeader(header.HeadersWithProof.Headers[0].ParachainHeader)
	if err != nil {
		return 0, err
	}

	paraHeight := int64(decodedHeader.Number)
	sp.LatestQueriedRelayHeight[paraHeight] = int64(block.Block.Header.Number)
	return int64(decodedHeader.Number), nil
}

// QueryIBCHeader returns the result of QueryLatestIBCHeader. It is only applicable when creating clients.
// QueryIBCHeaderOverBlocks should be used in the chain processor when querying headers to update the client.
func (sp *SubstrateProvider) QueryIBCHeader(ctx context.Context, h int64) (provider.IBCHeader, error) {
	if h <= 0 {
		return nil, fmt.Errorf("must pass in valid height, %d not valid", h)
	}

	latestFinalizedHeight, err := sp.QueryLatestHeight(ctx)
	if err != nil {
		return nil, err
	}

	if h > latestFinalizedHeight {
		return nil, fmt.Errorf("queried height is not finalized")
	}

	return sp.QueryLatestIBCHeader(h)
}

// QueryLatestIBCHeader returns the IBCHeader at the latest finalized relay chain height. It uses the relay chain height
// which is set in the SubstrateProvider when the QueryLatestHeight method is called.
func (sp *SubstrateProvider) QueryLatestIBCHeader(queriedParaHeight int64) (provider.IBCHeader, error) {
	var relayChainHeight uint64
	if _, ok := sp.LatestQueriedRelayHeight[queriedParaHeight]; !ok {
		return nil, fmt.Errorf("latest finalized parachain height needs to be queried first")
	}

	relayChainHeight = uint64(sp.LatestQueriedRelayHeight[queriedParaHeight])
	blockHash, err := sp.RelayerRPCClient.RPC.Chain.GetBlockHash(relayChainHeight)
	if err != nil {
		return nil, err
	}

	header, err := sp.constructBeefyHeader(blockHash, nil)
	if err != nil {
		return nil, err
	}

	return SubstrateIBCHeader{
		height:       uint64(header.MMRUpdateProof.SignedCommitment.Commitment.BlockNumber),
		SignedHeader: header,
	}, nil
}

func (sp *SubstrateProvider) QueryIBCHeaderOverBlocks(finalizedHeight, previouslyFinalized int64) (provider.IBCHeader, error) {
	// TODO
	return nil, nil
}

func (sp *SubstrateProvider) QuerySendPacket(ctx context.Context, srcChanID, srcPortID string, sequence uint64) (provider.PacketInfo, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryRecvPacket(ctx context.Context, dstChanID, dstPortID string, sequence uint64) (provider.PacketInfo, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryBalance(ctx context.Context, keyName string) (sdk.Coins, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryBalanceWithAddress(ctx context.Context, addr string) (sdk.Coins, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryUnbondingPeriod(ctx context.Context) (time.Duration, error) {
	// TODO: implement a proper unbonding period
	return time.Duration(math.MaxInt), nil
}

func (sp *SubstrateProvider) QueryClientState(ctx context.Context, height int64, clientid string) (ibcexported.ClientState, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryClientStateResponse(ctx context.Context, height int64, srcClientId string) (*clienttypes.QueryClientStateResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryClientConsensusState(ctx context.Context, chainHeight int64, clientid string, clientHeight ibcexported.Height) (*clienttypes.QueryConsensusStateResponse, error) {
	return sp.RPCClient.RPC.IBC.QueryClientConsensusState(ctx, uint32(chainHeight), clientid,
		clientHeight.GetRevisionHeight(), clientHeight.GetRevisionNumber(), true)
}

func (sp *SubstrateProvider) QueryUpgradedClient(ctx context.Context, height int64) (*clienttypes.QueryClientStateResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryUpgradedConsState(ctx context.Context, height int64) (*clienttypes.QueryConsensusStateResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryConsensusState(ctx context.Context, height int64) (ibcexported.ConsensusState, int64, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryClients(ctx context.Context) (clienttypes.IdentifiedClientStates, error) {
	return sp.RPCClient.RPC.IBC.QueryClients(ctx)
}

func (sp *SubstrateProvider) QueryConnection(ctx context.Context, height int64, connectionid string) (*conntypes.QueryConnectionResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryConnections(ctx context.Context) (conns []*conntypes.IdentifiedConnection, err error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryConnectionsUsingClient(ctx context.Context, height int64, clientid string) (*conntypes.QueryConnectionsResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) GenerateConnHandshakeProof(ctx context.Context, height int64, clientId, connId string) (clientState ibcexported.ClientState, clientStateProof []byte, consensusProof []byte, connectionProof []byte, connectionProofHeight ibcexported.Height, err error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryChannel(ctx context.Context, height int64, channelid, portid string) (chanRes *chantypes.QueryChannelResponse, err error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryChannelClient(ctx context.Context, height int64, channelid, portid string) (*clienttypes.IdentifiedClientState, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryConnectionChannels(ctx context.Context, height int64, connectionid string) ([]*chantypes.IdentifiedChannel, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryChannels(ctx context.Context) ([]*chantypes.IdentifiedChannel, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryPacketCommitments(ctx context.Context, height uint64, channelid, portid string) (commitments *chantypes.QueryPacketCommitmentsResponse, err error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryPacketAcknowledgements(ctx context.Context, height uint64, channelid, portid string) (acknowledgements []*chantypes.PacketState, err error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryUnreceivedPackets(ctx context.Context, height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryUnreceivedAcknowledgements(ctx context.Context, height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryNextSeqRecv(ctx context.Context, height int64, channelid, portid string) (recvRes *chantypes.QueryNextSequenceReceiveResponse, err error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryPacketCommitment(ctx context.Context, height int64, channelid, portid string, seq uint64) (comRes *chantypes.QueryPacketCommitmentResponse, err error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryPacketAcknowledgement(ctx context.Context, height int64, channelid, portid string, seq uint64) (ackRes *chantypes.QueryPacketAcknowledgementResponse, err error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryPacketReceipt(ctx context.Context, height int64, channelid, portid string, seq uint64) (recRes *chantypes.QueryPacketReceiptResponse, err error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryDenomTrace(ctx context.Context, denom string) (*transfertypes.DenomTrace, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryDenomTraces(ctx context.Context, offset, limit uint64, height int64) ([]transfertypes.DenomTrace, error) {
	//TODO implement me
	panic("implement me")
}
