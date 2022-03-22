package cosmos

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	querytypes "github.com/cosmos/cosmos-sdk/types/query"
	bankTypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v3/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	commitmenttypes "github.com/cosmos/ibc-go/v3/modules/core/23-commitment/types"
	committypes "github.com/cosmos/ibc-go/v3/modules/core/23-commitment/types"
	host "github.com/cosmos/ibc-go/v3/modules/core/24-host"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	ibctmtypes "github.com/cosmos/ibc-go/v3/modules/light-clients/07-tendermint/types"
	tmclient "github.com/cosmos/ibc-go/v3/modules/light-clients/07-tendermint/types"
	"github.com/pkg/errors"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/light"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
	"golang.org/x/sync/errgroup"
)

// QueryTx takes a transaction hash and returns the transaction
func (cc *CosmosProvider) QueryTx(ctx context.Context, hashHex string) (*ctypes.ResultTx, error) {
	hash, err := hex.DecodeString(hashHex)
	if err != nil {
		return nil, err
	}

	return cc.RPCClient.Tx(ctx, hash, true)
}

// QueryTxs returns an array of transactions given a tag
func (cc *CosmosProvider) QueryTxs(ctx context.Context, page, limit int, events []string) ([]*ctypes.ResultTx, error) {
	if len(events) == 0 {
		return nil, errors.New("must declare at least one event to search")
	}

	if page <= 0 {
		return nil, errors.New("page must greater than 0")
	}

	if limit <= 0 {
		return nil, errors.New("limit must greater than 0")
	}

	res, err := cc.RPCClient.TxSearch(ctx, strings.Join(events, " AND "), true, &page, &limit, "")
	if err != nil {
		return nil, err
	}
	return res.Txs, nil
}

// QueryBalance returns the amount of coins in the relayer account
func (cc *CosmosProvider) QueryBalance(ctx context.Context, keyName string) (sdk.Coins, error) {
	if keyName != "" {
		cc.PCfg.Key = keyName
	}
	addr, err := cc.Address()
	if err != nil {
		return nil, err
	}

	return cc.QueryBalanceWithAddress(ctx, addr)
}

// QueryBalanceWithAddress returns the amount of coins in the relayer account with address as input
// TODO add pagination support
func (cc *CosmosProvider) QueryBalanceWithAddress(ctx context.Context, address string) (sdk.Coins, error) {
	p := &bankTypes.QueryAllBalancesRequest{Address: address, Pagination: DefaultPageRequest()}
	queryClient := bankTypes.NewQueryClient(cc)

	res, err := queryClient.AllBalances(ctx, p)
	if err != nil {
		return nil, err
	}

	return res.Balances, nil
}

// QueryUnbondingPeriod returns the unbonding period of the chain
func (cc *CosmosProvider) QueryUnbondingPeriod(ctx context.Context) (time.Duration, error) {
	req := stakingtypes.QueryParamsRequest{}
	queryClient := stakingtypes.NewQueryClient(cc)

	res, err := queryClient.Params(ctx, &req)
	if err != nil {
		return 0, err
	}

	return res.Params.UnbondingTime, nil
}

// QueryTendermintProof performs an ABCI query with the given key and returns
// the value of the query, the proto encoded merkle proof, and the height of
// the Tendermint block containing the state root. The desired tendermint height
// to perform the query should be set in the client context. The query will be
// performed at one below this height (at the IAVL version) in order to obtain
// the correct merkle proof. Proof queries at height less than or equal to 2 are
// not supported. Queries with a client context height of 0 will perform a query
// at the lastest state available.
// Issue: https://github.com/cosmos/cosmos-sdk/issues/6567
func (cc *CosmosProvider) QueryTendermintProof(height int64, key []byte) ([]byte, []byte, clienttypes.Height, error) {
	// ABCI queries at heights 1, 2 or less than or equal to 0 are not supported.
	// Base app does not support queries for height less than or equal to 1.
	// Therefore, a query at height 2 would be equivalent to a query at height 3.
	// A height of 0 will query with the lastest state.
	if height != 0 && height <= 2 {
		return nil, nil, clienttypes.Height{}, fmt.Errorf("proof queries at height <= 2 are not supported")
	}

	// Use the IAVL height if a valid tendermint height is passed in.
	// A height of 0 will query with the latest state.
	if height != 0 {
		height--
	}

	req := abci.RequestQuery{
		Path:   fmt.Sprintf("store/%s/key", host.StoreKey),
		Height: height,
		Data:   key,
		Prove:  true,
	}

	res, err := cc.QueryABCI(req)
	if err != nil {
		return nil, nil, clienttypes.Height{}, err
	}

	merkleProof, err := commitmenttypes.ConvertProofs(res.ProofOps)
	if err != nil {
		return nil, nil, clienttypes.Height{}, err
	}

	cdc := codec.NewProtoCodec(cc.Codec.InterfaceRegistry)

	proofBz, err := cdc.Marshal(&merkleProof)
	if err != nil {
		return nil, nil, clienttypes.Height{}, err
	}

	revision := clienttypes.ParseChainID(cc.PCfg.ChainID)
	return res.Value, proofBz, clienttypes.NewHeight(revision, uint64(res.Height)+1), nil
}

// QueryClientStateResponse retrieves the latest consensus state for a client in state at a given height
func (cc *CosmosProvider) QueryClientStateResponse(height int64, srcClientId string) (*clienttypes.QueryClientStateResponse, error) {
	key := host.FullClientStateKey(srcClientId)

	value, proofBz, proofHeight, err := cc.QueryTendermintProof(height, key)
	if err != nil {
		return nil, err
	}

	// check if client exists
	if len(value) == 0 {
		return nil, sdkerrors.Wrap(clienttypes.ErrClientNotFound, srcClientId)
	}

	cdc := codec.NewProtoCodec(cc.Codec.InterfaceRegistry)

	clientState, err := clienttypes.UnmarshalClientState(cdc, value)
	if err != nil {
		return nil, err
	}

	anyClientState, err := clienttypes.PackClientState(clientState)
	if err != nil {
		return nil, err
	}

	return &clienttypes.QueryClientStateResponse{
		ClientState: anyClientState,
		Proof:       proofBz,
		ProofHeight: proofHeight,
	}, nil
}

// QueryClientState retrieves the latest consensus state for a client in state at a given height
// and unpacks it to exported client state interface
func (cc *CosmosProvider) QueryClientState(height int64, clientid string) (ibcexported.ClientState, error) {
	clientStateRes, err := cc.QueryClientStateResponse(height, clientid)
	if err != nil {
		return nil, err
	}

	clientStateExported, err := clienttypes.UnpackClientState(clientStateRes.ClientState)
	if err != nil {
		return nil, err
	}

	return clientStateExported, nil
}

// QueryClientConsensusState retrieves the latest consensus state for a client in state at a given height
func (cc *CosmosProvider) QueryClientConsensusState(chainHeight int64, clientid string, clientHeight ibcexported.Height) (*clienttypes.QueryConsensusStateResponse, error) {
	key := host.FullConsensusStateKey(clientid, clientHeight)

	value, proofBz, proofHeight, err := cc.QueryTendermintProof(chainHeight, key)
	if err != nil {
		return nil, err
	}

	// check if consensus state exists
	if len(value) == 0 {
		return nil, sdkerrors.Wrap(clienttypes.ErrConsensusStateNotFound, clientid)
	}

	cdc := codec.NewProtoCodec(cc.Codec.InterfaceRegistry)

	cs, err := clienttypes.UnmarshalConsensusState(cdc, value)
	if err != nil {
		return nil, err
	}

	anyConsensusState, err := clienttypes.PackConsensusState(cs)
	if err != nil {
		return nil, err
	}

	return &clienttypes.QueryConsensusStateResponse{
		ConsensusState: anyConsensusState,
		Proof:          proofBz,
		ProofHeight:    proofHeight,
	}, nil
}

//DefaultUpgradePath is the default IBC upgrade path set for an on-chain light client
var defaultUpgradePath = []string{"upgrade", "upgradedIBCState"}

func (cc *CosmosProvider) NewClientState(dstUpdateHeader ibcexported.Header, dstTrustingPeriod, dstUbdPeriod time.Duration, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour bool) (ibcexported.ClientState, error) {
	dstTmHeader, ok := dstUpdateHeader.(*tmclient.Header)
	if !ok {
		return nil, fmt.Errorf("got data of type %T but wanted tmclient.Header", dstUpdateHeader)
	}

	// Create the ClientState we want on 'c' tracking 'dst'
	return &tmclient.ClientState{
		ChainId:                      dstTmHeader.GetHeader().GetChainID(),
		TrustLevel:                   tmclient.NewFractionFromTm(light.DefaultTrustLevel),
		TrustingPeriod:               dstTrustingPeriod,
		UnbondingPeriod:              dstUbdPeriod,
		MaxClockDrift:                time.Minute * 10,
		FrozenHeight:                 clienttypes.ZeroHeight(),
		LatestHeight:                 dstUpdateHeader.GetHeight().(clienttypes.Height),
		ProofSpecs:                   committypes.GetSDKSpecs(),
		UpgradePath:                  defaultUpgradePath,
		AllowUpdateAfterExpiry:       allowUpdateAfterExpiry,
		AllowUpdateAfterMisbehaviour: allowUpdateAfterMisbehaviour,
	}, nil
}

// QueryUpgradeProof performs an abci query with the given key and returns the proto encoded merkle proof
// for the query and the height at which the proof will succeed on a tendermint verifier.
func (cc *CosmosProvider) QueryUpgradeProof(key []byte, height uint64) ([]byte, clienttypes.Height, error) {
	res, err := cc.QueryABCI(abci.RequestQuery{
		Path:   "store/upgrade/key",
		Height: int64(height - 1),
		Data:   key,
		Prove:  true,
	})
	if err != nil {
		return nil, clienttypes.Height{}, err
	}

	merkleProof, err := committypes.ConvertProofs(res.ProofOps)
	if err != nil {
		return nil, clienttypes.Height{}, err
	}

	proof, err := cc.Codec.Marshaler.Marshal(&merkleProof)
	if err != nil {
		return nil, clienttypes.Height{}, err
	}

	revision := clienttypes.ParseChainID(cc.PCfg.ChainID)

	// proof height + 1 is returned as the proof created corresponds to the height the proof
	// was created in the IAVL tree. Tendermint and subsequently the clients that rely on it
	// have heights 1 above the IAVL tree. Thus we return proof height + 1
	return proof, clienttypes.Height{
		RevisionNumber: revision,
		RevisionHeight: uint64(res.Height + 1),
	}, nil
}

// QueryUpgradedClient returns upgraded client info
func (cc *CosmosProvider) QueryUpgradedClient(ctx context.Context, height int64) (*clienttypes.QueryClientStateResponse, error) {
	req := clienttypes.QueryUpgradedClientStateRequest{}

	queryClient := clienttypes.NewQueryClient(cc)

	res, err := queryClient.UpgradedClientState(ctx, &req)
	if err != nil {
		return nil, err
	}

	if res == nil || res.UpgradedClientState == nil {
		return nil, fmt.Errorf("upgraded client state plan does not exist at height %d", height)
	}

	proof, proofHeight, err := cc.QueryUpgradeProof(upgradetypes.UpgradedClientKey(height), uint64(height))
	if err != nil {
		return nil, err
	}

	return &clienttypes.QueryClientStateResponse{
		ClientState: res.UpgradedClientState,
		Proof:       proof,
		ProofHeight: proofHeight,
	}, nil
}

// QueryUpgradedConsState returns upgraded consensus state and height of client
func (cc *CosmosProvider) QueryUpgradedConsState(ctx context.Context, height int64) (*clienttypes.QueryConsensusStateResponse, error) {
	req := clienttypes.QueryUpgradedConsensusStateRequest{}

	queryClient := clienttypes.NewQueryClient(cc)

	res, err := queryClient.UpgradedConsensusState(ctx, &req)
	if err != nil {
		return nil, err
	}

	if res == nil || res.UpgradedConsensusState == nil {
		return nil, fmt.Errorf("upgraded consensus state plan does not exist at height %d", height)
	}

	proof, proofHeight, err := cc.QueryUpgradeProof(upgradetypes.UpgradedConsStateKey(height), uint64(height))
	if err != nil {
		return nil, err
	}

	return &clienttypes.QueryConsensusStateResponse{
		ConsensusState: res.UpgradedConsensusState,
		Proof:          proof,
		ProofHeight:    proofHeight,
	}, nil
}

// QueryConsensusState returns a consensus state for a given chain to be used as a
// client in another chain, fetches latest height when passed 0 as arg
func (cc *CosmosProvider) QueryConsensusState(ctx context.Context, height int64) (ibcexported.ConsensusState, int64, error) {
	commit, err := cc.RPCClient.Commit(ctx, &height)
	if err != nil {
		return &ibctmtypes.ConsensusState{}, 0, err
	}

	page := 1
	count := 10_000

	nextHeight := height + 1
	nextVals, err := cc.RPCClient.Validators(ctx, &nextHeight, &page, &count)
	if err != nil {
		return &ibctmtypes.ConsensusState{}, 0, err
	}

	state := &ibctmtypes.ConsensusState{
		Timestamp:          commit.Time,
		Root:               commitmenttypes.NewMerkleRoot(commit.AppHash),
		NextValidatorsHash: tmtypes.NewValidatorSet(nextVals.Validators).Hash(),
	}

	return state, height, nil
}

// QueryClients queries all the clients!
// TODO add pagination support
func (cc *CosmosProvider) QueryClients(ctx context.Context) (clienttypes.IdentifiedClientStates, error) {
	qc := clienttypes.NewQueryClient(cc)
	state, err := qc.ClientStates(ctx, &clienttypes.QueryClientStatesRequest{
		Pagination: DefaultPageRequest(),
	})
	if err != nil {
		return nil, err
	}
	return state.ClientStates, nil
}

// QueryConnection returns the remote end of a given connection
func (cc *CosmosProvider) QueryConnection(height int64, connectionid string) (*conntypes.QueryConnectionResponse, error) {
	res, err := cc.queryConnectionABCI(height, connectionid)
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

func (cc *CosmosProvider) queryConnectionABCI(height int64, connectionID string) (*conntypes.QueryConnectionResponse, error) {
	key := host.ConnectionKey(connectionID)

	value, proofBz, proofHeight, err := cc.QueryTendermintProof(height, key)
	if err != nil {
		return nil, err
	}

	// check if connection exists
	if len(value) == 0 {
		return nil, sdkerrors.Wrap(conntypes.ErrConnectionNotFound, connectionID)
	}

	cdc := codec.NewProtoCodec(cc.Codec.InterfaceRegistry)

	var connection conntypes.ConnectionEnd
	if err := cdc.Unmarshal(value, &connection); err != nil {
		return nil, err
	}

	return &conntypes.QueryConnectionResponse{
		Connection:  &connection,
		Proof:       proofBz,
		ProofHeight: proofHeight,
	}, nil
}

// QueryConnections gets any connections on a chain
// TODO add pagination support
func (cc *CosmosProvider) QueryConnections(ctx context.Context) (conns []*conntypes.IdentifiedConnection, err error) {
	qc := conntypes.NewQueryClient(cc)
	res, err := qc.Connections(ctx, &conntypes.QueryConnectionsRequest{
		Pagination: DefaultPageRequest(),
	})
	if err != nil || res == nil {
		return nil, err
	}
	return res.Connections, err
}

// QueryConnectionsUsingClient gets any connections that exist between chain and counterparty
// TODO add pagination support
func (cc *CosmosProvider) QueryConnectionsUsingClient(ctx context.Context, height int64, clientid string) (*conntypes.QueryConnectionsResponse, error) {
	qc := conntypes.NewQueryClient(cc)
	res, err := qc.Connections(ctx, &conntypes.QueryConnectionsRequest{
		Pagination: DefaultPageRequest(),
	})
	return res, err
}

// GenerateConnHandshakeProof generates all the proofs needed to prove the existence of the
// connection state on this chain. A counterparty should use these generated proofs.
func (cc *CosmosProvider) GenerateConnHandshakeProof(height int64, clientId, connId string) (clientState ibcexported.ClientState, clientStateProof []byte, consensusProof []byte, connectionProof []byte, connectionProofHeight ibcexported.Height, err error) {
	var (
		clientStateRes     *clienttypes.QueryClientStateResponse
		consensusStateRes  *clienttypes.QueryConsensusStateResponse
		connectionStateRes *conntypes.QueryConnectionResponse
		eg                 = new(errgroup.Group)
	)

	// query for the client state for the proof and get the height to query the consensus state at.
	clientStateRes, err = cc.QueryClientStateResponse(height, clientId)
	if err != nil {
		return nil, nil, nil, nil, clienttypes.Height{}, err
	}

	clientState, err = clienttypes.UnpackClientState(clientStateRes.ClientState)
	if err != nil {
		return nil, nil, nil, nil, clienttypes.Height{}, err
	}

	eg.Go(func() error {
		var err error
		consensusStateRes, err = cc.QueryClientConsensusState(height, clientId, clientState.GetLatestHeight())
		return err
	})
	eg.Go(func() error {
		var err error
		connectionStateRes, err = cc.QueryConnection(height, connId)
		return err
	})

	if err := eg.Wait(); err != nil {
		return nil, nil, nil, nil, clienttypes.Height{}, err
	}

	return clientState, clientStateRes.Proof, consensusStateRes.Proof, connectionStateRes.Proof, connectionStateRes.ProofHeight, nil
}

// QueryChannel returns the channel associated with a channelID
func (cc *CosmosProvider) QueryChannel(height int64, channelid, portid string) (chanRes *chantypes.QueryChannelResponse, err error) {
	res, err := cc.queryChannelABCI(height, portid, channelid)
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

func (cc *CosmosProvider) queryChannelABCI(height int64, portID, channelID string) (*chantypes.QueryChannelResponse, error) {
	key := host.ChannelKey(portID, channelID)

	value, proofBz, proofHeight, err := cc.QueryTendermintProof(height, key)
	if err != nil {
		return nil, err
	}

	// check if channel exists
	if len(value) == 0 {
		return nil, sdkerrors.Wrapf(chantypes.ErrChannelNotFound, "portID (%s), channelID (%s)", portID, channelID)
	}

	cdc := codec.NewProtoCodec(cc.Codec.InterfaceRegistry)

	var channel chantypes.Channel
	if err := cdc.Unmarshal(value, &channel); err != nil {
		return nil, err
	}

	return &chantypes.QueryChannelResponse{
		Channel:     &channel,
		Proof:       proofBz,
		ProofHeight: proofHeight,
	}, nil
}

// QueryChannelClient returns the client state of the client supporting a given channel
func (cc *CosmosProvider) QueryChannelClient(ctx context.Context, height int64, channelid, portid string) (*clienttypes.IdentifiedClientState, error) {
	qc := chantypes.NewQueryClient(cc)
	cState, err := qc.ChannelClientState(ctx, &chantypes.QueryChannelClientStateRequest{
		PortId:    portid,
		ChannelId: channelid,
	})
	if err != nil {
		return nil, err
	}
	return cState.IdentifiedClientState, nil
}

// QueryConnectionChannels queries the channels associated with a connection
func (cc *CosmosProvider) QueryConnectionChannels(ctx context.Context, height int64, connectionid string) ([]*chantypes.IdentifiedChannel, error) {
	qc := chantypes.NewQueryClient(cc)
	chans, err := qc.ConnectionChannels(ctx, &chantypes.QueryConnectionChannelsRequest{
		Connection: connectionid,
		Pagination: DefaultPageRequest(),
	})
	if err != nil {
		return nil, err
	}
	return chans.Channels, nil
}

// QueryChannels returns all the channels that are registered on a chain
// TODO add pagination support
func (cc *CosmosProvider) QueryChannels(ctx context.Context) ([]*chantypes.IdentifiedChannel, error) {
	qc := chantypes.NewQueryClient(cc)
	res, err := qc.Channels(ctx, &chantypes.QueryChannelsRequest{
		Pagination: DefaultPageRequest(),
	})
	if err != nil {
		return nil, err
	}
	return res.Channels, nil
}

// QueryPacketCommitments returns an array of packet commitments
// TODO add pagination support
func (cc *CosmosProvider) QueryPacketCommitments(ctx context.Context, height uint64, channelid, portid string) (commitments *chantypes.QueryPacketCommitmentsResponse, err error) {
	qc := chantypes.NewQueryClient(cc)
	c, err := qc.PacketCommitments(ctx, &chantypes.QueryPacketCommitmentsRequest{
		PortId:     portid,
		ChannelId:  channelid,
		Pagination: DefaultPageRequest(),
	})
	if err != nil {
		return nil, err
	}
	return c, nil
}

// QueryPacketAcknowledgements returns an array of packet acks
// TODO add pagination support
func (cc *CosmosProvider) QueryPacketAcknowledgements(height uint64, channelid, portid string) (acknowledgements []*chantypes.PacketState, err error) {
	qc := chantypes.NewQueryClient(cc)
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

// QueryUnreceivedPackets returns a list of unrelayed packet commitments
func (cc *CosmosProvider) QueryUnreceivedPackets(height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	qc := chantypes.NewQueryClient(cc)
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

// QueryUnreceivedAcknowledgements returns a list of unrelayed packet acks
func (cc *CosmosProvider) QueryUnreceivedAcknowledgements(height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	qc := chantypes.NewQueryClient(cc)
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

// QueryNextSeqRecv returns the next seqRecv for a configured channel
func (cc *CosmosProvider) QueryNextSeqRecv(height int64, channelid, portid string) (recvRes *chantypes.QueryNextSequenceReceiveResponse, err error) {
	key := host.NextSequenceRecvKey(portid, channelid)

	value, proofBz, proofHeight, err := cc.QueryTendermintProof(height, key)
	if err != nil {
		return nil, err
	}

	// check if next sequence receive exists
	if len(value) == 0 {
		return nil, sdkerrors.Wrapf(chantypes.ErrChannelNotFound, "portID (%s), channelID (%s)", portid, channelid)
	}

	sequence := binary.BigEndian.Uint64(value)

	return &chantypes.QueryNextSequenceReceiveResponse{
		NextSequenceReceive: sequence,
		Proof:               proofBz,
		ProofHeight:         proofHeight,
	}, nil
}

// QueryPacketCommitment returns the packet commitment proof at a given height
func (cc *CosmosProvider) QueryPacketCommitment(height int64, channelid, portid string, seq uint64) (comRes *chantypes.QueryPacketCommitmentResponse, err error) {
	key := host.PacketCommitmentKey(portid, channelid, seq)

	value, proofBz, proofHeight, err := cc.QueryTendermintProof(height, key)
	if err != nil {
		return nil, err
	}

	// check if packet commitment exists
	if len(value) == 0 {
		return nil, sdkerrors.Wrapf(chantypes.ErrPacketCommitmentNotFound, "portID (%s), channelID (%s), sequence (%d)", portid, channelid, seq)
	}

	return &chantypes.QueryPacketCommitmentResponse{
		Commitment:  value,
		Proof:       proofBz,
		ProofHeight: proofHeight,
	}, nil
}

// QueryPacketAcknowledgement returns the packet ack proof at a given height
func (cc *CosmosProvider) QueryPacketAcknowledgement(height int64, channelid, portid string, seq uint64) (ackRes *chantypes.QueryPacketAcknowledgementResponse, err error) {
	key := host.PacketAcknowledgementKey(portid, channelid, seq)

	value, proofBz, proofHeight, err := cc.QueryTendermintProof(height, key)
	if err != nil {
		return nil, err
	}

	if len(value) == 0 {
		return nil, sdkerrors.Wrapf(chantypes.ErrInvalidAcknowledgement, "portID (%s), channelID (%s), sequence (%d)", portid, channelid, seq)
	}

	return &chantypes.QueryPacketAcknowledgementResponse{
		Acknowledgement: value,
		Proof:           proofBz,
		ProofHeight:     proofHeight,
	}, nil
}

// QueryPacketReceipt returns the packet receipt proof at a given height
func (cc *CosmosProvider) QueryPacketReceipt(height int64, channelid, portid string, seq uint64) (recRes *chantypes.QueryPacketReceiptResponse, err error) {
	key := host.PacketReceiptKey(portid, channelid, seq)

	value, proofBz, proofHeight, err := cc.QueryTendermintProof(height, key)
	if err != nil {
		return nil, err
	}

	return &chantypes.QueryPacketReceiptResponse{
		Received:    value != nil,
		Proof:       proofBz,
		ProofHeight: proofHeight,
	}, nil
}

func (cc *CosmosProvider) QueryLatestHeight() (int64, error) {
	stat, err := cc.RPCClient.Status(context.Background())
	if err != nil {
		return -1, err
	} else if stat.SyncInfo.CatchingUp {
		return -1, fmt.Errorf("node at %s running chain %s not caught up", cc.PCfg.RPCAddr, cc.PCfg.ChainID)
	}
	return stat.SyncInfo.LatestBlockHeight, nil
}

// QueryHeaderAtHeight returns the header at a given height
func (cc *CosmosProvider) QueryHeaderAtHeight(height int64) (ibcexported.Header, error) {
	var (
		page    = 1
		perPage = 100000
	)
	if height <= 0 {
		return nil, fmt.Errorf("must pass in valid height, %d not valid", height)
	}

	res, err := cc.RPCClient.Commit(context.Background(), &height)
	if err != nil {
		return nil, err
	}

	val, err := cc.RPCClient.Validators(context.Background(), &height, &page, &perPage)
	if err != nil {
		return nil, err
	}

	protoVal, err := tmtypes.NewValidatorSet(val.Validators).ToProto()
	if err != nil {
		return nil, err
	}

	return &tmclient.Header{
		// NOTE: This is not a SignedHeader
		// We are missing a light.Commit type here
		SignedHeader: res.SignedHeader.ToProto(),
		ValidatorSet: protoVal,
	}, nil
}

// QueryDenomTrace takes a denom from IBC and queries the information about it
func (cc *CosmosProvider) QueryDenomTrace(denom string) (*transfertypes.DenomTrace, error) {
	transfers, err := transfertypes.NewQueryClient(cc).DenomTrace(context.Background(),
		&transfertypes.QueryDenomTraceRequest{
			Hash: denom,
		})
	if err != nil {
		return nil, err
	}
	return transfers.DenomTrace, nil
}

// QueryDenomTraces returns all the denom traces from a given chain
// TODO add pagination support
func (cc *CosmosProvider) QueryDenomTraces(offset, limit uint64, height int64) ([]transfertypes.DenomTrace, error) {
	transfers, err := transfertypes.NewQueryClient(cc).DenomTraces(context.Background(),
		&transfertypes.QueryDenomTracesRequest{
			Pagination: DefaultPageRequest(),
		})
	if err != nil {
		return nil, err
	}
	return transfers.DenomTraces, nil
}

func (cc *CosmosProvider) QueryStakingParams(ctx context.Context) (*stakingtypes.Params, error) {
	res, err := stakingtypes.NewQueryClient(cc).Params(ctx, &stakingtypes.QueryParamsRequest{})
	if err != nil {
		return nil, err
	}
	return &res.Params, nil
}

func DefaultPageRequest() *querytypes.PageRequest {
	return &querytypes.PageRequest{
		Key:        []byte(""),
		Offset:     0,
		Limit:      1000,
		CountTotal: true,
	}
}
