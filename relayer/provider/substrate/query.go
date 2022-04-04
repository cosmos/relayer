package substrate

import (
	"context"
	"fmt"
	"strings"
	"time"

	rpcClientTypes "github.com/ComposableFi/go-substrate-rpc-client/v4/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	querytypes "github.com/cosmos/cosmos-sdk/types/query"
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v3/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	committypes "github.com/cosmos/ibc-go/v3/modules/core/23-commitment/types"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	beefyClientTypes "github.com/cosmos/ibc-go/v3/modules/light-clients/11-beefy/types"
	"github.com/cosmos/relayer/relayer/provider"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"

	"golang.org/x/sync/errgroup"
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
	latestBlockHash, err := sp.RPCClient.RPC.Chain.GetBlockHashLatest()
	if err != nil {
		return nil, err
	}

	c, err := signedCommitment(sp.RPCClient, latestBlockHash)
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

// QueryClients queries all the clients!
// TODO add pagination support
func (sp *SubstrateProvider) QueryClients() (clienttypes.IdentifiedClientStates, error) {
	qc := clienttypes.NewQueryClient(sp)
	state, err := qc.ClientStates(context.Background(), &clienttypes.QueryClientStatesRequest{
		Pagination: DefaultPageRequest(),
	})
	if err != nil {
		return nil, err
	}
	return state.ClientStates, nil
}

func (sp *SubstrateProvider) AutoUpdateClient(dst provider.ChainProvider, _ time.Duration, srcClientId, dstClientId string) (time.Duration, error) {
	srch, err := sp.QueryLatestHeight()
	if err != nil {
		return 0, err
	}
	dsth, err := dst.QueryLatestHeight()
	if err != nil {
		return 0, err
	}

	clientState, err := sp.queryTMClientState(srch, srcClientId)
	if err != nil {
		return 0, err
	}

	// query the latest consensus state of the potential matching client
	consensusStateResp, err := sp.QueryConsensusStateABCI(srcClientId, clientState.GetLatestHeight())
	if err != nil {
		return 0, err
	}

	exportedConsState, err := clienttypes.UnpackConsensusState(consensusStateResp.ConsensusState)
	if err != nil {
		return 0, err
	}

	_, ok := exportedConsState.(*beefyClientTypes.ConsensusState)
	if !ok {
		return 0, fmt.Errorf("consensus state with clientID %s from chain %s is not IBC tendermint type",
			srcClientId, sp.Config.ChainID)
	}

	srcUpdateHeader, err := sp.GetIBCUpdateHeader(srch, dst, dstClientId)
	if err != nil {
		return 0, err
	}

	dstUpdateHeader, err := dst.GetIBCUpdateHeader(dsth, sp, srcClientId)
	if err != nil {
		return 0, err
	}

	updateMsg, err := sp.UpdateClient(srcClientId, dstUpdateHeader)
	if err != nil {
		return 0, err
	}

	msgs := []provider.RelayerMessage{updateMsg}

	res, success, err := sp.SendMessages(msgs)
	if err != nil {
		// cp.LogFailedTx(res, err, CosmosMsgs(msgs...))
		return 0, err
	}
	if !success {
		return 0, fmt.Errorf("tx failed: %s", res.Data)
	}
	sp.Log(fmt.Sprintf("★ Client updated: [%s]client(%s) {%d}->{%d}",
		sp.Config.ChainID,
		srcClientId,
		MustGetHeight(srcUpdateHeader.GetHeight()),
		srcUpdateHeader.GetHeight().GetRevisionHeight(),
	))

	return maxDuration(), nil
}

func maxDuration() time.Duration {
	return 1<<63 - 1
}

func (sp *SubstrateProvider) FindMatchingClient(counterparty provider.ChainProvider, clientState ibcexported.ClientState) (string, bool) {
	clientsResp, err := sp.QueryClients()
	if err != nil {
		if sp.Config.Debug {
			sp.Log(fmt.Sprintf("Error: querying clients on %s failed: %v", sp.Config.ChainID, err))
		}
		return "", false
	}

	for _, identifiedClientState := range clientsResp {
		// unpack any into ibc tendermint client state
		existingClientState, err := castClientStateToBeefyType(identifiedClientState.ClientState)
		if err != nil {
			return "", false
		}

		tmClientState, ok := clientState.(*beefyClientTypes.ClientState)
		if !ok {
			if sp.Config.Debug {
				fmt.Printf("got data of type %T but wanted tmclient.ClientState \n", clientState)
			}
			return "", false
		}

		// check if the client states match
		// NOTE: FrozenHeight.IsZero() is a sanity check, the client to be created should always
		// have a zero frozen height and therefore should never match with a frozen client
		if isMatchingClient(tmClientState, existingClientState) && existingClientState.FrozenHeight == 0 {

			// query the latest consensus state of the potential matching client
			consensusStateResp, err := sp.QueryConsensusStateABCI(identifiedClientState.ClientId, existingClientState.GetLatestHeight())
			if err != nil {
				if sp.Config.Debug {
					sp.Log(fmt.Sprintf("Error: failed to query latest consensus state for existing client on chain %s: %v",
						sp.Config.ChainID, err))
				}
				continue
			}

			//nolint:lll
			header, err := counterparty.GetLightSignedHeaderAtHeight(int64(existingClientState.GetLatestHeight().GetRevisionHeight()))
			if err != nil {
				if sp.Config.Debug {
					sp.Log(fmt.Sprintf("Error: failed to query header for chain %s at height %d: %v",
						counterparty.ChainId(), existingClientState.GetLatestHeight().GetRevisionHeight(), err))
				}
				continue
			}

			exportedConsState, err := clienttypes.UnpackConsensusState(consensusStateResp.ConsensusState)
			if err != nil {
				if sp.Config.Debug {
					sp.Log(fmt.Sprintf("Error: failed to consensus state on chain %s: %v", counterparty.ChainId(), err))
				}
				continue
			}
			existingConsensusState, ok := exportedConsState.(*beefyClientTypes.ConsensusState)
			if !ok {
				if sp.Config.Debug {
					sp.Log(fmt.Sprintf("Error: consensus state is not tendermint type on chain %s", counterparty.ChainId()))
				}
				continue
			}

			tmHeader, ok := header.(*beefyClientTypes.Header)
			if !ok {
				if sp.Config.Debug {
					fmt.Printf("got data of type %T but wanted tmclient.Header \n", header)
				}
				return "", false
			}

			if isMatchingConsensusState(existingConsensusState, tmHeader.ConsensusState()) {
				// found matching client
				return identifiedClientState.ClientId, true
			}
		}
	}
	return "", false
}

func (sp *SubstrateProvider) QueryConnection(height int64, connectionid string) (*conntypes.QueryConnectionResponse, error) {
	res, err := sp.queryConnection(height, connectionid)
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

// TODO: query connection using rpc methods
func (sp *SubstrateProvider) queryConnection(height int64, connectionID string) (*conntypes.QueryConnectionResponse, error) {
	return &conntypes.QueryConnectionResponse{}, nil
}

// QueryConnections gets any connections on a chain
// TODO add pagination support
func (sp *SubstrateProvider) QueryConnections() (conns []*conntypes.IdentifiedConnection, err error) {
	qc := conntypes.NewQueryClient(sp)
	res, err := qc.Connections(context.Background(), &conntypes.QueryConnectionsRequest{
		Pagination: DefaultPageRequest(),
	})
	return res.Connections, err
}

// QueryConnectionsUsingClient gets any connections that exist between chain and counterparty
// TODO add pagination support
func (sp *SubstrateProvider) QueryConnectionsUsingClient(height int64, clientid string) (*conntypes.QueryConnectionsResponse, error) {
	qc := conntypes.NewQueryClient(sp)
	res, err := qc.Connections(context.Background(), &conntypes.QueryConnectionsRequest{
		Pagination: DefaultPageRequest(),
	})
	return res, err
}

func (sp *SubstrateProvider) GenerateConnHandshakeProof(height int64, clientId, connId string) (clientState ibcexported.ClientState, clientStateProof []byte, consensusProof []byte, connectionProof []byte, connectionProofHeight ibcexported.Height, err error) {
	var (
		clientStateRes     *clienttypes.QueryClientStateResponse
		consensusStateRes  *clienttypes.QueryConsensusStateResponse
		connectionStateRes *conntypes.QueryConnectionResponse
		eg                 = new(errgroup.Group)
	)

	// query for the client state for the proof and get the height to query the consensus state at.
	clientStateRes, err = sp.QueryClientStateResponse(height, clientId)
	if err != nil {
		return nil, nil, nil, nil, clienttypes.Height{}, err
	}

	clientState, err = clienttypes.UnpackClientState(clientStateRes.ClientState)
	if err != nil {
		return nil, nil, nil, nil, clienttypes.Height{}, err
	}

	eg.Go(func() error {
		var err error
		consensusStateRes, err = sp.QueryClientConsensusState(height, clientId, clientState.GetLatestHeight())
		return err
	})
	eg.Go(func() error {
		var err error
		connectionStateRes, err = sp.QueryConnection(height, connId)
		return err
	})

	if err := eg.Wait(); err != nil {
		return nil, nil, nil, nil, clienttypes.Height{}, err
	}

	return clientState, clientStateRes.Proof, consensusStateRes.Proof, connectionStateRes.Proof, connectionStateRes.ProofHeight, nil
}

func (sp *SubstrateProvider) NewClientState(
	dstUpdateHeader ibcexported.Header,
	_, _ time.Duration,
	_, _ bool,
) (ibcexported.ClientState, error) {
	dstBeefyHeader, ok := dstUpdateHeader.(*beefyClientTypes.Header)
	if !ok {
		return nil, fmt.Errorf("got data of type %T but wanted  beefyClientType.Header \n", dstUpdateHeader)
	}

	parachainHeader := dstBeefyHeader.ParachainHeaders[0].ParachainHeader
	substrateHeader := &rpcClientTypes.Header{}
	err := Decode(parachainHeader, substrateHeader)
	if err != nil {
		return nil, err
	}

	blockHash, err := sp.RPCClient.RPC.Chain.GetBlockHash(uint64(substrateHeader.Number))
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

func (sp *SubstrateProvider) QueryChannel(height int64, channelid, portid string) (chanRes *chantypes.QueryChannelResponse, err error) {
	res, err := sp.queryChannel(height, portid, channelid)
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

// TODO: query channel using rpc methods
func (sp *SubstrateProvider) queryChannel(height int64, portID, channelID string) (*chantypes.QueryChannelResponse, error) {
	return &chantypes.QueryChannelResponse{}, nil
}

func (sp *SubstrateProvider) QueryChannelClient(height int64, channelid, portid string) (*clienttypes.IdentifiedClientState, error) {
	qc := chantypes.NewQueryClient(sp)
	cState, err := qc.ChannelClientState(context.Background(), &chantypes.QueryChannelClientStateRequest{
		PortId:    portid,
		ChannelId: channelid,
	})
	if err != nil {
		return nil, err
	}
	return cState.IdentifiedClientState, nil
}

func (sp *SubstrateProvider) QueryConnectionChannels(height int64, connectionid string) ([]*chantypes.IdentifiedChannel, error) {
	qc := chantypes.NewQueryClient(sp)
	chans, err := qc.ConnectionChannels(context.Background(), &chantypes.QueryConnectionChannelsRequest{
		Connection: connectionid,
		Pagination: DefaultPageRequest(),
	})
	if err != nil {
		return nil, err
	}
	return chans.Channels, nil
}

func (sp *SubstrateProvider) QueryChannels() ([]*chantypes.IdentifiedChannel, error) {
	qc := chantypes.NewQueryClient(sp)
	res, err := qc.Channels(context.Background(), &chantypes.QueryChannelsRequest{
		Pagination: DefaultPageRequest(),
	})
	if err != nil {
		return nil, err
	}
	return res.Channels, err
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
