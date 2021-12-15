package cosmos

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankTypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	transfertypes "github.com/cosmos/ibc-go/v2/modules/apps/transfer/types"
	clientutils "github.com/cosmos/ibc-go/v2/modules/core/02-client/client/utils"
	clienttypes "github.com/cosmos/ibc-go/v2/modules/core/02-client/types"
	connutils "github.com/cosmos/ibc-go/v2/modules/core/03-connection/client/utils"
	conntypes "github.com/cosmos/ibc-go/v2/modules/core/03-connection/types"
	chanutils "github.com/cosmos/ibc-go/v2/modules/core/04-channel/client/utils"
	chantypes "github.com/cosmos/ibc-go/v2/modules/core/04-channel/types"
	committypes "github.com/cosmos/ibc-go/v2/modules/core/23-commitment/types"
	ibcexported "github.com/cosmos/ibc-go/v2/modules/core/exported"
	tmclient "github.com/cosmos/ibc-go/v2/modules/light-clients/07-tendermint/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
	"golang.org/x/sync/errgroup"
	"strings"
	"time"
)

// QueryTx takes a transaction hash and returns the transaction
func (cp *CosmosProvider) QueryTx(hashHex string) (*ctypes.ResultTx, error) {
	hash, err := hex.DecodeString(hashHex)
	if err != nil {
		return &ctypes.ResultTx{}, err
	}

	return cp.Client.Tx(context.Background(), hash, true)
}

// QueryTxs returns an array of transactions given a tag
func (cp *CosmosProvider) QueryTxs(page, limit int, events []string) ([]*ctypes.ResultTx, error) {
	if len(events) == 0 {
		return nil, errors.New("must declare at least one event to search")
	}

	if page <= 0 {
		return nil, errors.New("page must greater than 0")
	}

	if limit <= 0 {
		return nil, errors.New("limit must greater than 0")
	}

	res, err := cp.Client.TxSearch(context.Background(), strings.Join(events, " AND "), true, &page, &limit, "")
	if err != nil {
		return nil, err
	}
	return res.Txs, nil
}

// QueryLatestHeight queries the chain for the latest height and returns it
func (cp *CosmosProvider) QueryLatestHeight() (int64, error) {
	res, err := cp.Client.Status(context.Background())
	if err != nil {
		return -1, err
	} else if res.SyncInfo.CatchingUp {
		return -1, fmt.Errorf("node at %s running chain %s not caught up", cp.Config.RPCAddr, cp.Config.ChainID)
	}

	return res.SyncInfo.LatestBlockHeight, nil
}

// QueryHeaderAtHeight returns the header at a given height
func (cp *CosmosProvider) QueryHeaderAtHeight(height int64) (ibcexported.Header, error) {
	var (
		page    = 1
		perPage = 100000
	)
	if height <= 0 {
		return nil, fmt.Errorf("must pass in valid height, %d not valid", height)
	}

	res, err := cp.Client.Commit(context.Background(), &height)
	if err != nil {
		return nil, err
	}

	val, err := cp.Client.Validators(context.Background(), &height, &page, &perPage)
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

// QueryBalance returns the amount of coins in the relayer account
func (cp *CosmosProvider) QueryBalance(keyName string) (sdk.Coins, error) {
	var addr string
	if keyName == "" {
		addr = cp.MustGetAddress()
	} else {
		info, err := cp.Keybase.Key(keyName)
		if err != nil {
			return nil, err
		}
		done := cp.UseSDKContext()
		addr = info.GetAddress().String()
		done()
	}
	return cp.QueryBalanceWithAddress(addr)
}

// QueryBalanceWithAddress returns the amount of coins in the relayer account with address as input
// TODO add pagination support
func (cp *CosmosProvider) QueryBalanceWithAddress(address string) (sdk.Coins, error) {
	done := cp.UseSDKContext()
	addr, err := sdk.AccAddressFromBech32(address)
	done()
	if err != nil {
		return nil, err
	}

	p := bankTypes.NewQueryAllBalancesRequest(addr, DefaultPageRequest())

	queryClient := bankTypes.NewQueryClient(cp.CLIContext(0))

	res, err := queryClient.AllBalances(context.Background(), p)
	if err != nil {
		return nil, err
	}

	return res.Balances, nil
}

// QueryUnbondingPeriod returns the unbonding period of the chain
func (cp *CosmosProvider) QueryUnbondingPeriod() (time.Duration, error) {
	req := stakingtypes.QueryParamsRequest{}

	queryClient := stakingtypes.NewQueryClient(cp.CLIContext(0))

	res, err := queryClient.Params(context.Background(), &req)
	if err != nil {
		return 0, err
	}

	return res.Params.UnbondingTime, nil
}

// QueryClientStateResponse retrevies the latest consensus state for a client in state at a given height
func (cp *CosmosProvider) QueryClientStateResponse(height int64, srcClientId string) (*clienttypes.QueryClientStateResponse, error) {
	return clientutils.QueryClientStateABCI(cp.CLIContext(height), srcClientId)
}

// QueryClientState retrevies the latest consensus state for a client in state at a given height
// and unpacks it to exported client state interface
func (cp *CosmosProvider) QueryClientState(height int64, clientid string) (ibcexported.ClientState, error) {
	clientStateRes, err := cp.QueryClientStateResponse(height, clientid)
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
func (cp *CosmosProvider) QueryClientConsensusState(chainHeight int64, clientid string, clientHeight ibcexported.Height) (*clienttypes.QueryConsensusStateResponse, error) {
	return clientutils.QueryConsensusStateABCI(
		cp.CLIContext(chainHeight),
		clientid,
		clientHeight,
	)
}

// QueryUpgradedClient returns upgraded client info
func (cp *CosmosProvider) QueryUpgradedClient(height int64) (*clienttypes.QueryClientStateResponse, error) {
	req := clienttypes.QueryUpgradedClientStateRequest{}

	queryClient := clienttypes.NewQueryClient(cp.CLIContext(0))

	res, err := queryClient.UpgradedClientState(context.Background(), &req)
	if err != nil {
		return nil, err
	}

	if res == nil || res.UpgradedClientState == nil {
		return nil, fmt.Errorf("upgraded client state plan does not exist at height %d", height)
	}

	proof, proofHeight, err := cp.QueryUpgradeProof(upgradetypes.UpgradedClientKey(height), uint64(height))
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
func (cp *CosmosProvider) QueryUpgradedConsState(height int64) (*clienttypes.QueryConsensusStateResponse, error) {
	req := clienttypes.QueryUpgradedConsensusStateRequest{}

	queryClient := clienttypes.NewQueryClient(cp.CLIContext(height))

	res, err := queryClient.UpgradedConsensusState(context.Background(), &req)
	if err != nil {
		return nil, err
	}

	if res == nil || res.UpgradedConsensusState == nil {
		return nil, fmt.Errorf("upgraded consensus state plan does not exist at height %d", height)
	}

	proof, proofHeight, err := cp.QueryUpgradeProof(upgradetypes.UpgradedConsStateKey(height), uint64(height))
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
func (cp *CosmosProvider) QueryConsensusState(height int64) (ibcexported.ConsensusState, int64, error) {
	return clientutils.QuerySelfConsensusState(cp.CLIContext(height))
}

// QueryClients queries all the clients!
// TODO add pagination support
func (cp *CosmosProvider) QueryClients() (clienttypes.IdentifiedClientStates, error) {
	qc := clienttypes.NewQueryClient(cp.CLIContext(0))
	state, err := qc.ClientStates(context.Background(), &clienttypes.QueryClientStatesRequest{
		Pagination: DefaultPageRequest(),
	})
	if err != nil {
		return nil, err
	}
	return state.ClientStates, nil
}

// QueryConnection returns the remote end of a given connection
func (cp *CosmosProvider) QueryConnection(height int64, connectionid string) (*conntypes.QueryConnectionResponse, error) {
	res, err := connutils.QueryConnection(cp.CLIContext(height), connectionid, true)
	if err != nil && strings.Contains(err.Error(), "not found") {
		return conntypes.NewQueryConnectionResponse(
			conntypes.NewConnectionEnd(
				conntypes.UNINITIALIZED,
				"client",
				conntypes.NewCounterparty(
					"client",
					"connection",
					committypes.NewMerklePrefix([]byte{}),
				),
				[]*conntypes.Version{},
				0,
			), []byte{}, clienttypes.NewHeight(0, 0)), nil
	} else if err != nil {
		return nil, err
	}
	return res, nil
}

// QueryConnections gets any connections on a chain
// TODO add pagination support
func (cp *CosmosProvider) QueryConnections() (conns []*conntypes.IdentifiedConnection, err error) {
	qc := conntypes.NewQueryClient(cp.CLIContext(0))
	res, err := qc.Connections(context.Background(), &conntypes.QueryConnectionsRequest{
		Pagination: DefaultPageRequest(),
	})
	return res.Connections, err
}

// QueryConnectionsUsingClient gets any connections that exist between chain and counterparty
// TODO add pagination support
func (cp *CosmosProvider) QueryConnectionsUsingClient(height int64, clientid string) (*conntypes.QueryConnectionsResponse, error) {
	qc := conntypes.NewQueryClient(cp.CLIContext(0))
	res, err := qc.Connections(context.Background(), &conntypes.QueryConnectionsRequest{
		Pagination: DefaultPageRequest(),
	})
	return res, err
}

// GenerateConnHandshakeProof generates all the proofs needed to prove the existence of the
// connection state on this chain. A counterparty should use these generated proofs.
func (cp *CosmosProvider) GenerateConnHandshakeProof(height int64, clientId, connId string) (clientState ibcexported.ClientState, clientStateProof []byte, consensusProof []byte, connectionProof []byte, connectionProofHeight ibcexported.Height, err error) {
	var (
		clientStateRes     *clienttypes.QueryClientStateResponse
		consensusStateRes  *clienttypes.QueryConsensusStateResponse
		connectionStateRes *conntypes.QueryConnectionResponse
		eg                 = new(errgroup.Group)
	)

	// query for the client state for the proof and get the height to query the consensus state at.
	clientStateRes, err = cp.QueryClientStateResponse(height, clientId)
	if err != nil {
		return nil, nil, nil, nil, clienttypes.Height{}, err
	}

	clientState, err = clienttypes.UnpackClientState(clientStateRes.ClientState)
	if err != nil {
		return nil, nil, nil, nil, clienttypes.Height{}, err
	}

	eg.Go(func() error {
		var err error
		consensusStateRes, err = cp.QueryClientConsensusState(height, clientId, clientState.GetLatestHeight())
		return err
	})
	eg.Go(func() error {
		var err error
		connectionStateRes, err = cp.QueryConnection(height, connId)
		return err
	})

	if err := eg.Wait(); err != nil {
		return nil, nil, nil, nil, clienttypes.Height{}, err
	}

	return clientState, clientStateRes.Proof, consensusStateRes.Proof, connectionStateRes.Proof, connectionStateRes.ProofHeight, nil
}

// QueryChannel returns the channel associated with a channelID
func (cp *CosmosProvider) QueryChannel(height int64, channelid, portid string) (chanRes *chantypes.QueryChannelResponse, err error) {
	res, err := chanutils.QueryChannel(cp.CLIContext(height), portid, channelid, true)
	if err != nil && strings.Contains(err.Error(), "not found") {
		return chantypes.NewQueryChannelResponse(
			chantypes.NewChannel(
				chantypes.UNINITIALIZED,
				chantypes.UNORDERED,
				chantypes.NewCounterparty(
					"port",
					"channel",
				),
				[]string{},
				"version",
			),
			[]byte{},
			clienttypes.NewHeight(0, 0)), nil
	} else if err != nil {
		return nil, err
	}
	return res, nil
}

// QueryChannelClient returns the client state of the client supporting a given channel
func (cp *CosmosProvider) QueryChannelClient(height int64, channelid, portid string) (*clienttypes.IdentifiedClientState, error) {
	qc := chantypes.NewQueryClient(cp.CLIContext(height))
	cState, err := qc.ChannelClientState(context.Background(), &chantypes.QueryChannelClientStateRequest{
		PortId:    portid,
		ChannelId: channelid,
	})
	if err != nil {
		return nil, err
	}
	return cState.IdentifiedClientState, nil
}

// QueryConnectionChannels queries the channels associated with a connection
func (cp *CosmosProvider) QueryConnectionChannels(height int64, connectionid string) ([]*chantypes.IdentifiedChannel, error) {
	qc := chantypes.NewQueryClient(cp.CLIContext(0))
	chans, err := qc.ConnectionChannels(context.Background(), &chantypes.QueryConnectionChannelsRequest{
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
func (cp *CosmosProvider) QueryChannels() ([]*chantypes.IdentifiedChannel, error) {
	qc := chantypes.NewQueryClient(cp.CLIContext(0))
	res, err := qc.Channels(context.Background(), &chantypes.QueryChannelsRequest{
		Pagination: DefaultPageRequest(),
	})
	if err != nil {
		return nil, err
	}
	return res.Channels, err
}

// QueryPacketCommitments returns an array of packet commitments
// TODO add pagination support
func (cp *CosmosProvider) QueryPacketCommitments(height uint64, channelid, portid string) (commitments *chantypes.QueryPacketCommitmentsResponse, err error) {
	qc := chantypes.NewQueryClient(cp.CLIContext(int64(height)))
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

// QueryPacketAcknowledgements returns an array of packet acks
// TODO add pagination support
func (cp *CosmosProvider) QueryPacketAcknowledgements(height uint64, channelid, portid string) (acknowledgements []*chantypes.PacketState, err error) {
	qc := chantypes.NewQueryClient(cp.CLIContext(int64(height)))
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
func (cp *CosmosProvider) QueryUnreceivedPackets(height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	qc := chantypes.NewQueryClient(cp.CLIContext(int64(height)))
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
func (cp *CosmosProvider) QueryUnreceivedAcknowledgements(height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	qc := chantypes.NewQueryClient(cp.CLIContext(int64(height)))
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
func (cp *CosmosProvider) QueryNextSeqRecv(height int64, channelid, portid string) (recvRes *chantypes.QueryNextSequenceReceiveResponse, err error) {
	return chanutils.QueryNextSequenceReceive(cp.CLIContext(height), portid, channelid, true)
}

// QueryPacketCommitment returns the packet commitment proof at a given height
func (cp *CosmosProvider) QueryPacketCommitment(height int64, channelid, portid string, seq uint64) (comRes *chantypes.QueryPacketCommitmentResponse, err error) {
	return chanutils.QueryPacketCommitment(cp.CLIContext(height), portid, channelid, seq, true)
}

// QueryPacketAcknowledgement returns the packet ack proof at a given height
func (cp *CosmosProvider) QueryPacketAcknowledgement(height int64, channelid, portid string, seq uint64) (ackRes *chantypes.QueryPacketAcknowledgementResponse, err error) {
	return chanutils.QueryPacketAcknowledgement(cp.CLIContext(height), portid, channelid, seq, true)
}

// QueryPacketReceipt returns the packet receipt proof at a given height
func (cp *CosmosProvider) QueryPacketReceipt(height int64, channelid, portid string, seq uint64) (recRes *chantypes.QueryPacketReceiptResponse, err error) {
	return chanutils.QueryPacketReceipt(cp.CLIContext(height), portid, channelid, seq, true)
}

// QueryDenomTrace takes a denom from IBC and queries the information about it
func (cp *CosmosProvider) QueryDenomTrace(denom string) (*transfertypes.DenomTrace, error) {
	transfers, err := transfertypes.NewQueryClient(cp.CLIContext(0)).DenomTrace(context.Background(),
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
func (cp *CosmosProvider) QueryDenomTraces(offset, limit uint64, height int64) ([]transfertypes.DenomTrace, error) {
	transfers, err := transfertypes.NewQueryClient(cp.CLIContext(height)).DenomTraces(context.Background(),
		&transfertypes.QueryDenomTracesRequest{
			Pagination: DefaultPageRequest(),
		})
	if err != nil {
		return nil, err
	}
	return transfers.DenomTraces, nil
}
