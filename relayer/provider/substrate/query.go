package substrate

import (
	"context"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	querytypes "github.com/cosmos/cosmos-sdk/types/query"
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v3/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	"github.com/cosmos/relayer/relayer/provider"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

// QueryTx takes a transaction hash and returns the transaction
func (sp *SubstrateProvider) QueryTx(hashHex string) (*ctypes.ResultTx, error) {
	return nil, nil
}

func (sp *SubstrateProvider) QueryTxs(page, limit int, events []string) ([]*ctypes.ResultTx, error) {
	return nil, nil
}

func (sp *SubstrateProvider) QueryLatestHeight() (int64, error) {
	signedBlock, err := sp.RPCClient.RPC.Chain.GetBlockLatest()
	if err != nil {
		return 0, err
	}

	return int64(signedBlock.Block.Header.Number), nil
}

func (sp *SubstrateProvider) QueryHeaderAtHeight(height int64) (ibcexported.Header, error) {
	blockHash, err := sp.RPCClient.RPC.Chain.GetBlockHash(uint64(height))
	if err != nil {
		return nil, err
	}

	return constructBeefyHeader(sp.RPCClient, blockHash)
}

func (sp *SubstrateProvider) QueryBalance(keyName string) (sdk.Coins, error) {
	var (
		addr string
		err  error
	)
	if keyName == "" {
		addr, err = sp.Address()
	} else {
		sp.Config.Key = keyName
		addr, err = sp.Address()
	}

	if err != nil {
		return nil, err
	}
	return sp.QueryBalanceWithAddress(addr)
}

func (sp *SubstrateProvider) QueryBalanceWithAddress(addr string) (sdk.Coins, error) {
	return nil, nil
}

func (sp *SubstrateProvider) QueryUnbondingPeriod() (time.Duration, error) {
	return 0, nil
}

func (sp *SubstrateProvider) QueryClientState(height int64, clientid string) (ibcexported.ClientState, error) {
	blockHash, err := sp.RPCClient.RPC.Chain.GetBlockHash(uint64(height))
	if err != nil {
		return nil, err
	}

	commitment, err := signedCommitment(sp.RPCClient, blockHash)
	if err != nil {
		return nil, err
	}

	cs, err := clientState(sp.RPCClient, commitment)
	if err != nil {
		return nil, err
	}

	return cs, nil

}

func (sp *SubstrateProvider) QueryClientStateResponse(height int64, srcClientId string) (*clienttypes.QueryClientStateResponse, error) {
	return nil, nil
}

func (sp *SubstrateProvider) QueryClientConsensusState(chainHeight int64, clientid string, clientHeight ibcexported.Height) (*clienttypes.QueryConsensusStateResponse, error) {
	return nil, nil
}

func (sp *SubstrateProvider) QueryUpgradedClient(height int64) (*clienttypes.QueryClientStateResponse, error) {
	return nil, nil
}

func (sp *SubstrateProvider) QueryUpgradedConsState(height int64) (*clienttypes.QueryConsensusStateResponse, error) {
	return nil, nil
}

func (sp *SubstrateProvider) QueryConsensusState(height int64) (ibcexported.ConsensusState, int64, error) {
	return nil, 0, nil
}

func (sp *SubstrateProvider) QueryClients() (clienttypes.IdentifiedClientStates, error) {
	return nil, nil
}

func (sp *SubstrateProvider) AutoUpdateClient(dst provider.ChainProvider, thresholdTime time.Duration, srcClientId, dstClientId string) (time.Duration, error) {
	return 0, nil
}

func (sp *SubstrateProvider) FindMatchingClient(counterparty provider.ChainProvider, clientState ibcexported.ClientState) (string, bool) {
	return "", false
}

func (sp *SubstrateProvider) QueryConnection(height int64, connectionid string) (*conntypes.QueryConnectionResponse, error) {
	return nil, nil
}

func (sp *SubstrateProvider) QueryConnections() (conns []*conntypes.IdentifiedConnection, err error) {
	return nil, nil
}

func (sp *SubstrateProvider) QueryConnectionsUsingClient(height int64, clientid string) (*conntypes.QueryConnectionsResponse, error) {
	return nil, nil
}

func (sp *SubstrateProvider) GenerateConnHandshakeProof(height int64, clientId, connId string) (clientState ibcexported.ClientState, clientStateProof []byte, consensusProof []byte, connectionProof []byte, connectionProofHeight ibcexported.Height, err error) {
	return nil, nil, nil, nil, nil, nil
}

func (sp *SubstrateProvider) NewClientState(dstUpdateHeader ibcexported.Header, dstTrustingPeriod, dstUbdPeriod time.Duration, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour bool) (ibcexported.ClientState, error) {
	return nil, nil
}

func (sp *SubstrateProvider) QueryChannel(height int64, channelid, portid string) (chanRes *chantypes.QueryChannelResponse, err error) {
	return nil, nil
}

func (sp *SubstrateProvider) QueryChannelClient(height int64, channelid, portid string) (*clienttypes.IdentifiedClientState, error) {
	return nil, nil
}

func (sp *SubstrateProvider) QueryConnectionChannels(height int64, connectionid string) ([]*chantypes.IdentifiedChannel, error) {
	return nil, nil
}

func (sp *SubstrateProvider) QueryChannels() ([]*chantypes.IdentifiedChannel, error) {
	return nil, nil
}

func (sp *SubstrateProvider) QueryPacketCommitments(height uint64, channelid, portid string) (commitments *chantypes.QueryPacketCommitmentsResponse, err error) {
	qc := chantypes.NewQueryClient(sp)
	c, err := qc.PacketCommitments(context.Background(), &chantypes.QueryPacketCommitmentsRequest{
		PortId:     portid,
		ChannelId:  channelid,
		Pagination: DefaultPageRequest(),
	})
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (sp *SubstrateProvider) QueryPacketAcknowledgements(height uint64, channelid, portid string) (acknowledgements []*chantypes.PacketState, err error) {
	qc := chantypes.NewQueryClient(sp)
	acks, err := qc.PacketAcknowledgements(context.Background(), &chantypes.QueryPacketAcknowledgementsRequest{
		PortId:     portid,
		ChannelId:  channelid,
		Pagination: DefaultPageRequest(),
	})
	if err != nil {
		return nil, err
	}
	return acks.Acknowledgements, nil
}

func (sp *SubstrateProvider) QueryUnreceivedPackets(height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	qc := chantypes.NewQueryClient(sp)
	res, err := qc.UnreceivedPackets(context.Background(), &chantypes.QueryUnreceivedPacketsRequest{
		PortId:                    portid,
		ChannelId:                 channelid,
		PacketCommitmentSequences: seqs,
	})
	if err != nil {
		return nil, err
	}
	return res.Sequences, nil
}

func (sp *SubstrateProvider) QueryUnreceivedAcknowledgements(height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	qc := chantypes.NewQueryClient(sp)
	res, err := qc.UnreceivedAcks(context.Background(), &chantypes.QueryUnreceivedAcksRequest{
		PortId:             portid,
		ChannelId:          channelid,
		PacketAckSequences: seqs,
	})
	if err != nil {
		return nil, err
	}
	return res.Sequences, nil
}

func (sp *SubstrateProvider) QueryNextSeqRecv(height int64, channelid, portid string) (recvRes *chantypes.QueryNextSequenceReceiveResponse, err error) {
	return nil, nil
}

func (sp *SubstrateProvider) QueryPacketCommitment(height int64, channelid, portid string, seq uint64) (comRes *chantypes.QueryPacketCommitmentResponse, err error) {
	return nil, nil
}

func (sp *SubstrateProvider) QueryPacketAcknowledgement(height int64, channelid, portid string, seq uint64) (ackRes *chantypes.QueryPacketAcknowledgementResponse, err error) {
	return nil, nil
}

func (sp *SubstrateProvider) QueryPacketReceipt(height int64, channelid, portid string, seq uint64) (recRes *chantypes.QueryPacketReceiptResponse, err error) {
	return nil, nil
}

func (sp *SubstrateProvider) QueryDenomTrace(denom string) (*transfertypes.DenomTrace, error) {
	transfers, err := transfertypes.NewQueryClient(sp).DenomTrace(context.Background(),
		&transfertypes.QueryDenomTraceRequest{
			Hash: denom,
		})
	if err != nil {
		return nil, err
	}
	return transfers.DenomTrace, nil
}

func (sp *SubstrateProvider) QueryDenomTraces(offset, limit uint64, height int64) ([]transfertypes.DenomTrace, error) {
	transfers, err := transfertypes.NewQueryClient(sp).DenomTraces(context.Background(),
		&transfertypes.QueryDenomTracesRequest{
			Pagination: DefaultPageRequest(),
		})
	if err != nil {
		return nil, err
	}
	return transfers.DenomTraces, nil
}

func DefaultPageRequest() *querytypes.PageRequest {
	return &querytypes.PageRequest{
		Key:        []byte(""),
		Offset:     0,
		Limit:      1000,
		CountTotal: true,
	}
}
