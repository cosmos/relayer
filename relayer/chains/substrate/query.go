package substrate

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	beefyclienttypes "github.com/ComposableFi/ics11-beefy/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v5/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v5/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
	committypes "github.com/cosmos/ibc-go/v5/modules/core/23-commitment/types"
	ibcexported "github.com/cosmos/ibc-go/v5/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"golang.org/x/sync/errgroup"
)

var _ provider.QueryProvider = &SubstrateProvider{}

// QueryTx takes a transaction hash and returns the transaction
func (sp *SubstrateProvider) QueryTx(ctx context.Context, hashHex string) (*provider.RelayerTxResponse, error) {
	return &provider.RelayerTxResponse{}, errors.New(ErrTextSubstrateDoesNotHaveQueryForTransactions)
}

// QueryTxs returns an array of transactions given a tag
func (sp *SubstrateProvider) QueryTxs(ctx context.Context, page, limit int, events []string) ([]*provider.RelayerTxResponse, error) {
	return []*provider.RelayerTxResponse{}, errors.New(ErrTextSubstrateDoesNotHaveQueryForTransactions)
}

// QueryBalance returns the amount of coins in the relayer account
func (sp *SubstrateProvider) QueryBalance(ctx context.Context, keyName string) (sdk.Coins, error) {
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
	return sp.QueryBalanceWithAddress(ctx, addr)
}

// QueryBalanceWithAddress returns the amount of coins in the relayer account with address as input
// TODO add pagination support
func (sp *SubstrateProvider) QueryBalanceWithAddress(ctx context.Context, address string) (sdk.Coins, error) {
	// TODO: addr might need to be passed as byte not string
	res, err := sp.RPCClient.RPC.IBC.QueryBalanceWithAddress(ctx, []byte(address))
	if err != nil {
		return nil, err
	}

	return res, nil
}

// QueryUnbondingPeriod returns the unbonding period of the chain
func (sp *SubstrateProvider) QueryUnbondingPeriod(ctx context.Context) (time.Duration, error) {
	return 0, nil
}

// QueryClientStateResponse retrieves the latest consensus state for a client in state at a given height
func (sp *SubstrateProvider) QueryClientStateResponse(ctx context.Context, height int64, srcClientId string) (*clienttypes.QueryClientStateResponse, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryClientStateResponse(ctx, height, srcClientId)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

// QueryClientState retrieves the latest consensus state for a client in state at a given height
// and unpacks it to exported client state interface
func (sp *SubstrateProvider) QueryClientState(ctx context.Context, height int64, clientid string) (ibcexported.ClientState, error) {
	res, err := sp.QueryClientStateResponse(ctx, height, clientid)
	if err != nil {
		return nil, err
	}

	clientStateExported, err := clienttypes.UnpackClientState(res.ClientState)
	if err != nil {
		return nil, err
	}

	return clientStateExported, nil
}

// QueryClientConsensusState retrieves the latest consensus state for a client in state at a given height
func (sp *SubstrateProvider) QueryClientConsensusState(ctx context.Context, chainHeight int64, clientid string, clientHeight ibcexported.Height) (*clienttypes.QueryConsensusStateResponse, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryClientConsensusState(ctx,
		uint32(chainHeight), clientid, clientHeight.GetRevisionNumber(), clientHeight.GetRevisionHeight(), false)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// QueryUpgradeProof performs an abci query with the given key and returns the proto encoded merkle proof
// for the query and the height at which the proof will succeed on a tendermint verifier.
func (sp *SubstrateProvider) QueryUpgradeProof(ctx context.Context, key []byte, height uint64) ([]byte, clienttypes.Height, error) {
	return nil, clienttypes.Height{}, nil
}

// QueryUpgradedClient returns upgraded client info
func (sp *SubstrateProvider) QueryUpgradedClient(ctx context.Context, height int64) (*clienttypes.QueryClientStateResponse, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryUpgradedClient(ctx, height)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// QueryUpgradedConsState returns upgraded consensus state and height of client
func (sp *SubstrateProvider) QueryUpgradedConsState(ctx context.Context, height int64) (*clienttypes.QueryConsensusStateResponse, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryUpgradedConsState(ctx, height)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// QueryConsensusState returns a consensus state for a given chain to be used as a
// client in another chain, fetches latest height when passed 0 as arg
func (sp *SubstrateProvider) QueryConsensusState(ctx context.Context, height int64) (ibcexported.ConsensusState, int64, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryConsensusState(ctx, uint32(height))
	if err != nil {
		return nil, 0, err
	}

	consensusStateExported, err := clienttypes.UnpackConsensusState(res.ConsensusState)
	if err != nil {
		return nil, 0, err
	}

	return consensusStateExported, height, nil
}

// QueryClients queries all the clients!
// TODO add pagination support
func (sp *SubstrateProvider) QueryClients(ctx context.Context) (clienttypes.IdentifiedClientStates, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryClients(ctx)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// QueryConnection returns the remote end of a given connection
func (sp *SubstrateProvider) QueryConnection(ctx context.Context, height int64, connectionid string) (*conntypes.QueryConnectionResponse, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryConnection(ctx, height, connectionid)
	if err != nil {
		return nil, err
	}

	if err != nil && strings.Contains(err.Error(), "not found") {
		return &conntypes.QueryConnectionResponse{
			Connection: &conntypes.ConnectionEnd{
				ClientId: "client",
				Versions: []*conntypes.Version{},
				State:    conntypes.UNINITIALIZED,
				Counterparty: conntypes.Counterparty{
					ClientId:     "client",
					ConnectionId: "connection",
					Prefix:       committypes.MerklePrefix{KeyPrefix: []byte{}},
				},
				DelayPeriod: 0,
			},
			Proof:       []byte{},
			ProofHeight: clienttypes.Height{RevisionNumber: 0, RevisionHeight: 0},
		}, nil
	} else if err != nil {
		return nil, err
	}
	return res, nil
}

// QueryConnections gets any connections on a chain
// TODO add pagination support
func (sp *SubstrateProvider) QueryConnections(ctx context.Context) (conns []*conntypes.IdentifiedConnection, err error) {
	res, err := sp.RPCClient.RPC.IBC.QueryConnections(ctx)
	if err != nil {
		return nil, err
	}

	return res.Connections, nil
}

// QueryConnectionsUsingClient gets any connections that exist between chain and counterparty
// TODO add pagination support
func (sp *SubstrateProvider) QueryConnectionsUsingClient(ctx context.Context, height int64, clientid string) (*conntypes.QueryConnectionsResponse, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryConnectionsUsingClient(ctx, height, clientid)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// GenerateConnHandshakeProof generates all the proofs needed to prove the existence of the
// connection state on this chain. A counterparty should use these generated proofs.
func (sp *SubstrateProvider) GenerateConnHandshakeProof(ctx context.Context, height int64, clientId, connId string) (
	clientState ibcexported.ClientState, clientStateProof []byte, consensusProof []byte, connectionProof []byte, connectionProofHeight ibcexported.Height, err error,
) {
	var (
		clientStateRes     *clienttypes.QueryClientStateResponse
		consensusStateRes  *clienttypes.QueryConsensusStateResponse
		connectionStateRes *conntypes.QueryConnectionResponse
		eg                 = new(errgroup.Group)
	)

	// query for the client state for the proof and get the height to query the consensus state at.
	clientStateRes, err = sp.QueryClientStateResponse(ctx, height, clientId)
	if err != nil {
		return nil, nil, nil, nil, clienttypes.Height{}, err
	}

	clientState, err = clienttypes.UnpackClientState(clientStateRes.ClientState)
	if err != nil {
		return nil, nil, nil, nil, clienttypes.Height{}, err
	}

	eg.Go(func() error {
		var err error
		consensusStateRes, err = sp.QueryClientConsensusState(ctx, height, clientId, clientState.GetLatestHeight())
		return err
	})
	eg.Go(func() error {
		var err error
		connectionStateRes, err = sp.QueryConnection(ctx, height, connId)
		return err
	})

	if err := eg.Wait(); err != nil {
		return nil, nil, nil, nil, clienttypes.Height{}, err
	}

	return clientState, clientStateRes.Proof, consensusStateRes.Proof, connectionStateRes.Proof, connectionStateRes.ProofHeight, nil
}

// QueryChannel returns the channel associated with a channelID
func (sp *SubstrateProvider) QueryChannel(ctx context.Context, height int64, channelid, portid string) (chanRes *chantypes.QueryChannelResponse, err error) {

	res, err := sp.RPCClient.RPC.IBC.QueryChannel(ctx, uint32(height), channelid, portid)
	if err != nil {
		return nil, err
	}

	// TODO check if how can be the "not found" result from composable node
	if err != nil && strings.Contains(err.Error(), "not found") {
		return &chantypes.QueryChannelResponse{
			Channel: &chantypes.Channel{
				State:    chantypes.UNINITIALIZED,
				Ordering: chantypes.UNORDERED,
				Counterparty: chantypes.Counterparty{
					PortId:    "port",
					ChannelId: "channel",
				},
				ConnectionHops: []string{},
				Version:        "version",
			},
			Proof: []byte{},
			ProofHeight: clienttypes.Height{
				RevisionNumber: 0,
				RevisionHeight: 0,
			},
		}, nil
	} else if err != nil {
		return nil, err
	}
	return res, nil
}

// QueryChannelClient returns the client state of the client supporting a given channel
func (sp *SubstrateProvider) QueryChannelClient(ctx context.Context, height int64, channelid, portid string) (*clienttypes.IdentifiedClientState, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryChannelClient(ctx, uint32(height), channelid, portid)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// QueryConnectionChannels queries the channels associated with a connection
func (sp *SubstrateProvider) QueryConnectionChannels(ctx context.Context, height int64, connectionid string) ([]*chantypes.IdentifiedChannel, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryConnectionChannels(ctx, uint32(height), connectionid)
	if err != nil {
		return nil, err
	}

	return res.Channels, nil
}

// QueryChannels returns all the channels that are registered on a chain
// TODO add pagination support
func (sp *SubstrateProvider) QueryChannels(ctx context.Context) ([]*chantypes.IdentifiedChannel, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryChannels(ctx)
	if err != nil {
		return nil, err
	}

	return res.Channels, err
}

// QueryPacketCommitments returns an array of packet commitments
// TODO add pagination support
func (sp *SubstrateProvider) QueryPacketCommitments(ctx context.Context, height uint64, channelid, portid string) (commitments *chantypes.QueryPacketCommitmentsResponse, err error) {
	res, err := sp.RPCClient.RPC.IBC.QueryPacketCommitments(ctx, height, channelid, portid)
	if err != nil {
		return nil, err
	}

	return res, err
}

// QueryPacketAcknowledgements returns an array of packet acks
// TODO add pagination support
func (sp *SubstrateProvider) QueryPacketAcknowledgements(ctx context.Context, height uint64, channelid, portid string) (acknowledgements []*chantypes.PacketState, err error) {
	res, err := sp.RPCClient.RPC.IBC.QueryPacketAcknowledgements(ctx, uint32(height), channelid, portid)
	if err != nil {
		return nil, err
	}

	return res.Acknowledgements, err
}

// QueryUnreceivedPackets returns a list of unrelayed packet commitments
func (sp *SubstrateProvider) QueryUnreceivedPackets(ctx context.Context, height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	packets, err := sp.RPCClient.RPC.IBC.QueryUnreceivedPackets(ctx, uint32(height), channelid, portid, seqs)
	if err != nil {
		return nil, err
	}

	return packets, err
}

func (sp *SubstrateProvider) QuerySendPacket(
	ctx context.Context,
	srcChanID,
	srcPortID string,
	sequence uint64,
) (provider.PacketInfo, error) {
	packets, err := sp.RPCClient.RPC.IBC.QuerySendPackets(ctx, srcChanID, srcPortID, []uint64{sequence})
	if err != nil {
		return provider.PacketInfo{}, err
	}
	packet := packets[0]
	return provider.PacketInfo{
		Height:           packet.Height,
		Sequence:         packet.Sequence,
		SourceChannel:    packet.SourceChannel,
		SourcePort:       packet.SourcePort,
		DestChannel:      packet.DestinationChannel,
		DestPort:         packet.DestinationPort,
		ChannelOrder:     packet.ChannelOrder,
		Data:             packet.Data,
		Ack:              packet.Ack,
		TimeoutHeight:    packet.TimeoutHeight,
		TimeoutTimestamp: packet.TimeoutTimestamp,
	}, nil
}

func (sp *SubstrateProvider) QueryRecvPacket(
	ctx context.Context,
	dstChanID,
	dstPortID string,
	sequence uint64,
) (provider.PacketInfo, error) {
	packets, err := sp.RPCClient.RPC.IBC.QueryRecvPackets(ctx, dstChanID, dstPortID, []uint64{sequence})
	if err != nil {
		return provider.PacketInfo{}, err
	}
	packet := packets[0]
	return provider.PacketInfo{
		Height:           packet.Height,
		Sequence:         packet.Sequence,
		SourceChannel:    packet.SourceChannel,
		SourcePort:       packet.SourcePort,
		DestChannel:      packet.DestinationChannel,
		DestPort:         packet.DestinationPort,
		ChannelOrder:     packet.ChannelOrder,
		Data:             packet.Data,
		Ack:              packet.Ack,
		TimeoutHeight:    packet.TimeoutHeight,
		TimeoutTimestamp: packet.TimeoutTimestamp,
	}, nil
}

// QueryUnreceivedAcknowledgements returns a list of unrelayed packet acks
func (sp *SubstrateProvider) QueryUnreceivedAcknowledgements(ctx context.Context, height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	var ack []uint64
	ack, err := sp.RPCClient.RPC.IBC.QueryUnreceivedAcknowledgements(ctx, uint32(height), channelid, portid, seqs)
	if err != nil {
		return nil, err
	}

	return ack, err
}

// QueryNextSeqRecv returns the next seqRecv for a configured channel
func (sp *SubstrateProvider) QueryNextSeqRecv(ctx context.Context, height int64, channelid, portid string) (recvRes *chantypes.QueryNextSequenceReceiveResponse, err error) {
	recvRes, err = sp.RPCClient.RPC.IBC.QueryNextSeqRecv(ctx, uint32(height), channelid, portid)
	if err != nil {
		return nil, err
	}
	return
}

// QueryPacketCommitment returns the packet commitment proof at a given height
func (sp *SubstrateProvider) QueryPacketCommitment(ctx context.Context, height int64, channelid, portid string, seq uint64) (comRes *chantypes.QueryPacketCommitmentResponse, err error) {
	comRes, err = sp.RPCClient.RPC.IBC.QueryPacketCommitment(ctx, height, channelid, portid)
	if err != nil {
		return nil, err
	}
	return
}

// QueryPacketAcknowledgement returns the packet ack proof at a given height
func (sp *SubstrateProvider) QueryPacketAcknowledgement(ctx context.Context, height int64, channelid, portid string, seq uint64) (ackRes *chantypes.QueryPacketAcknowledgementResponse, err error) {
	ackRes, err = sp.RPCClient.RPC.IBC.QueryPacketAcknowledgement(ctx, uint32(height), channelid, portid, seq)
	if err != nil {
		return nil, err
	}
	return
}

// QueryPacketReceipt returns the packet receipt proof at a given height
func (sp *SubstrateProvider) QueryPacketReceipt(ctx context.Context, height int64, channelid, portid string, seq uint64) (recRes *chantypes.QueryPacketReceiptResponse, err error) {
	recRes, err = sp.RPCClient.RPC.IBC.QueryPacketReceipt(ctx, uint32(height), channelid, portid, seq)
	if err != nil {
		return nil, err
	}
	return
}

func (sp *SubstrateProvider) QueryLatestHeight(ctx context.Context) (int64, error) {
	signedHash, err := sp.RelayChainRPCClient.RPC.Beefy.GetFinalizedHead()
	if err != nil {
		return 0, err
	}

	block, err := sp.RelayChainRPCClient.RPC.Chain.GetBlock(signedHash)
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

	sp.LatestQueriedRelayChainHeight = int64(block.Block.Header.Number)
	return int64(decodedHeader.Number), nil
}

// QueryHeaderAtHeight returns the header at a given height
func (sp *SubstrateProvider) QueryHeaderAtHeight(ctx context.Context, height int64) (ibcexported.Header, error) {
	latestBlockHash, err := sp.RPCClient.RPC.Chain.GetBlockHashLatest()
	if err != nil {
		return nil, err
	}

	c, err := sp.signedCommitment(latestBlockHash)
	if err != nil {
		return nil, err
	}

	if int64(c.Commitment.BlockNumber) < height {
		return nil, fmt.Errorf("queried block is not finalized")
	}

	blockHash, err := sp.RPCClient.RPC.Chain.GetBlockHash(uint64(height))
	if err != nil {
		return nil, err
	}

	return sp.constructBeefyHeader(blockHash, nil)
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

	return sp.QueryLatestIBCHeader()
}

// QueryLatestIBCHeader returns the IBCHeader at the latest finalized relay chain height. It uses the relay chain height
// which is set in the SubstrateProvider when the QueryLatestHeight method is called.
func (sp *SubstrateProvider) QueryLatestIBCHeader() (provider.IBCHeader, error) {
	if sp.LatestQueriedRelayChainHeight == 0 {
		return nil, fmt.Errorf("latest finalized parachain height needs to be queried first")
	}

	relayChainHeight := uint64(sp.LatestQueriedRelayChainHeight)
	blockHash, err := sp.RelayChainRPCClient.RPC.Chain.GetBlockHash(relayChainHeight)
	if err != nil {
		return nil, err
	}

	header, err := sp.constructBeefyHeader(blockHash, nil)
	if err != nil {
		return nil, err
	}

	// reset latest queried relay height after IBC Header is queried
	sp.LatestQueriedRelayChainHeight = 0
	return SubstrateIBCHeader{
		height:       uint64(header.MMRUpdateProof.SignedCommitment.Commitment.BlockNumber),
		SignedHeader: header,
	}, nil
}

func (sp *SubstrateProvider) QueryIBCHeaderOverBlocks(finalizedHeight, previouslyFinalized uint64) (provider.IBCHeader, error) {
	finalizedHash, err := sp.RelayChainRPCClient.RPC.Chain.GetBlockHash(finalizedHeight)
	if err != nil {
		return nil, err
	}

	previouslyFinalizedHash, err := sp.RelayChainRPCClient.RPC.Chain.GetBlockHash(previouslyFinalized)
	if err != nil {
		return nil, err
	}

	header, err := sp.constructBeefyHeader(finalizedHash, &previouslyFinalizedHash)
	if err != nil {
		return nil, err
	}

	return SubstrateIBCHeader{
		height:       uint64(header.MMRUpdateProof.SignedCommitment.Commitment.BlockNumber),
		SignedHeader: header,
	}, nil
}

// QueryDenomTrace takes a denom from IBC and queries the information about it
func (sp *SubstrateProvider) QueryDenomTrace(ctx context.Context, denom string) (*transfertypes.DenomTrace, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryDenomTrace(ctx, denom)
	if err != nil {
		return nil, err
	}

	return res.DenomTrace, err
}

// QueryDenomTraces returns all the denom traces from a given chain
// TODO add pagination support
func (sp *SubstrateProvider) QueryDenomTraces(ctx context.Context, offset, limit uint64, height int64) ([]transfertypes.DenomTrace, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryDenomTraces(ctx, offset, limit, uint32(height))
	if err != nil {
		return nil, err
	}

	return res.DenomTraces, err
}

func (sp *SubstrateProvider) QueryConsensusStateABCI(ctx context.Context, clientID string, height ibcexported.Height) (*clienttypes.QueryConsensusStateResponse, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryConsensusState(ctx, uint32(height.GetRevisionHeight()))
	if err != nil {
		return nil, err
	}

	// check if consensus state exists
	if len(res.Proof) == 0 {
		return nil, fmt.Errorf(ErrTextConsensusStateNotFound, clientID)
	}

	return &clienttypes.QueryConsensusStateResponse{
		ConsensusState: res.ConsensusState,
		Proof:          res.Proof,
		ProofHeight:    res.ProofHeight,
	}, nil
}
