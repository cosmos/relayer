package substrate

import (
	"context"
	"fmt"
	"strings"
	"time"

	rpcClientTypes "github.com/ComposableFi/go-substrate-rpc-client/v4/types"
	beefyClientTypes "github.com/cosmos/ibc-go/v3/modules/light-clients/11-beefy/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v3/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	committypes "github.com/cosmos/ibc-go/v3/modules/core/23-commitment/types"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/provider"

	"golang.org/x/sync/errgroup"
)

// QueryTx takes a transaction hash and returns the transaction
func (sp *SubstrateProvider) QueryTx(ctx context.Context, hashHex string) (*provider.RelayerTxResponse, error) {
	return nil, nil
}

// QueryTxs returns an array of transactions given a tag
func (cc *SubstrateProvider) QueryTxs(ctx context.Context, page, limit int, events []string) ([]*provider.RelayerTxResponse, error) {
	return nil, nil
}

func (sp *SubstrateProvider) QueryLatestHeight(ctx context.Context) (int64, error) {
	signedBlock, err := sp.RPCClient.RPC.Chain.GetBlockLatest()
	if err != nil {
		return 0, err
	}

	return int64(signedBlock.Block.Header.Number), nil
}

func (sp *SubstrateProvider) QueryHeaderAtHeight(ctx context.Context, height int64) (ibcexported.ClientMessage, error) {
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

func (sp *SubstrateProvider) QueryBalanceWithAddress(ctx context.Context, addr string) (sdk.Coins, error) {
	// TODO: addr might need to be passed as byte not string
	res, err := sp.RPCClient.RPC.IBC.QueryBalanceWithAddress([]byte(addr))
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (sp *SubstrateProvider) QueryUnbondingPeriod(ctx context.Context) (time.Duration, error) {
	return 0, nil
}

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
	//blockHash, err := sp.RPCClient.RPC.Chain.GetBlockHash(uint64(height))
	//if err != nil {
	//	return nil, err
	//}
	//
	//commitment, err := signedCommitment(sp.RPCClient, blockHash)
	//if err != nil {
	//	return nil, err
	//}
	//
	//cs, err := clientState(sp.RPCClient, commitment)
	//if err != nil {
	//	return nil, err
	//}
	//
	//return cs, nil
}

func (sp *SubstrateProvider) QueryClientStateResponse(ctx context.Context, height int64, srcClientId string) (*clienttypes.QueryClientStateResponse, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryClientStateResponse(height, srcClientId)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (sp *SubstrateProvider) QueryClientConsensusState(ctx context.Context, chainHeight int64, clientid string, clientHeight ibcexported.Height) (*clienttypes.QueryConsensusStateResponse, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryClientConsensusState(clientid,
		clientHeight.GetRevisionHeight(), clientHeight.GetRevisionNumber(), false)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (sp *SubstrateProvider) QueryUpgradedClient(ctx context.Context, height int64) (*clienttypes.QueryClientStateResponse, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryUpgradedClient(height)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (sp *SubstrateProvider) QueryUpgradedConsState(ctx context.Context, height int64) (*clienttypes.QueryConsensusStateResponse, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryUpgradedConsState(height)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (sp *SubstrateProvider) QueryConsensusState(ctx context.Context, height int64) (ibcexported.ConsensusState, int64, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryConsensusState(uint32(height))
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
	res, err := sp.RPCClient.RPC.IBC.QueryClients()
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (sp *SubstrateProvider) AutoUpdateClient(ctx context.Context, dst provider.ChainProvider, _ time.Duration, srcClientId, dstClientId string) (time.Duration, error) {
	srch, err := sp.QueryLatestHeight(ctx)
	if err != nil {
		return 0, err
	}
	dsth, err := dst.QueryLatestHeight(ctx)
	if err != nil {
		return 0, err
	}

	clientState, err := sp.queryTMClientState(ctx, srch, srcClientId)
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

	srcUpdateHeader, err := sp.GetIBCUpdateHeader(ctx, srch, dst, dstClientId)
	if err != nil {
		return 0, err
	}

	dstUpdateHeader, err := dst.GetIBCUpdateHeader(ctx, dsth, sp, srcClientId)
	if err != nil {
		return 0, err
	}

	updateMsg, err := sp.UpdateClient(srcClientId, dstUpdateHeader)
	if err != nil {
		return 0, err
	}

	msgs := []provider.RelayerMessage{updateMsg}

	res, success, err := sp.SendMessages(ctx, msgs)
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

func (sp *SubstrateProvider) FindMatchingClient(ctx context.Context, counterparty provider.ChainProvider, clientState ibcexported.ClientState) (string, bool) {
	clientsResp, err := sp.QueryClients(ctx)
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
			//todo: return finalized block here ?
			header, err := counterparty.GetLightSignedHeaderAtHeight(ctx, int64(existingClientState.GetLatestHeight().GetRevisionHeight()))
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

func (sp *SubstrateProvider) QueryConnection(ctx context.Context, height int64, connectionid string) (*conntypes.QueryConnectionResponse, error) {
	res, err := sp.queryConnection(ctx, height, connectionid)
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

func (sp *SubstrateProvider) queryConnection(ctx context.Context, height int64, connectionID string) (*conntypes.QueryConnectionResponse, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryConnection(height, connectionID)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// QueryConnections gets any connections on a chain
// TODO add pagination support
func (sp *SubstrateProvider) QueryConnections(ctx context.Context) (conns []*conntypes.IdentifiedConnection, err error) {
	res, err := sp.RPCClient.RPC.IBC.QueryConnections()
	if err != nil {
		return nil, err
	}

	return res.Connections, nil
}

// QueryConnectionsUsingClient gets any connections that exist between chain and counterparty
// TODO add pagination support
func (sp *SubstrateProvider) QueryConnectionsUsingClient(ctx context.Context, height int64, clientid string) (*conntypes.QueryConnectionsResponse, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryConnectionsUsingClient(height, clientid)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (sp *SubstrateProvider) GenerateConnHandshakeProof(ctx context.Context, height int64, clientId, connId string) (clientState ibcexported.ClientState, clientStateProof []byte, consensusProof []byte, connectionProof []byte, connectionProofHeight ibcexported.Height, err error) {
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

func (sp *SubstrateProvider) NewClientState(
	dstUpdateHeader ibcexported.ClientMessage,
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

func (sp *SubstrateProvider) QueryChannel(ctx context.Context, height int64, channelid, portid string) (chanRes *chantypes.QueryChannelResponse, err error) {
	res, err := sp.queryChannel(ctx, height, portid, channelid)
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
func (sp *SubstrateProvider) queryChannel(ctx context.Context, height int64, portID, channelID string) (*chantypes.QueryChannelResponse, error) {
	return &chantypes.QueryChannelResponse{}, nil
}

func (sp *SubstrateProvider) QueryChannelClient(ctx context.Context, height int64, channelid, portid string) (*clienttypes.IdentifiedClientState, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryChannelClient(uint32(height), channelid, portid)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (sp *SubstrateProvider) QueryConnectionChannels(ctx context.Context, height int64, connectionid string) ([]*chantypes.IdentifiedChannel, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryConnectionChannels(uint32(height), connectionid)
	if err != nil {
		return nil, err
	}

	return res.Channels, nil
}

func (sp *SubstrateProvider) QueryChannels(ctx context.Context) ([]*chantypes.IdentifiedChannel, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryChannels()
	if err != nil {
		return nil, err
	}

	return res.Channels, err
}

func (sp *SubstrateProvider) QueryPacketCommitments(ctx context.Context, height uint64, channelid, portid string) (commitments *chantypes.QueryPacketCommitmentsResponse, err error) {
	res, err := sp.RPCClient.RPC.IBC.QueryPacketCommitments(height, channelid, portid)
	if err != nil {
		return nil, err
	}

	return res, err
}

func (sp *SubstrateProvider) QueryPacketAcknowledgements(ctx context.Context, height uint64, channelid, portid string) (acknowledgements []*chantypes.PacketState, err error) {
	res, err := sp.RPCClient.RPC.IBC.QueryPacketAcknowledgements(uint32(height), channelid, portid)
	if err != nil {
		return nil, err
	}

	return res.Acknowledgements, err
}

func (sp *SubstrateProvider) QueryUnreceivedPackets(ctx context.Context, height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	packets, err := sp.RPCClient.RPC.IBC.QueryUnreceivedPackets(uint32(height), channelid, portid, seqs)
	if err != nil {
		return nil, err
	}

	return packets, err
}

func (sp *SubstrateProvider) QueryUnreceivedAcknowledgements(ctx context.Context, height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	var ack []uint64
	ack, err := sp.RPCClient.RPC.IBC.QueryUnreceivedAcknowledgements(uint32(height), channelid, portid, seqs)
	if err != nil {
		return nil, err
	}

	return ack, err
}

func (sp *SubstrateProvider) QueryNextSeqRecv(ctx context.Context, height int64, channelid, portid string) (recvRes *chantypes.QueryNextSequenceReceiveResponse, err error) {
	recvRes, err = sp.RPCClient.RPC.IBC.QueryNextSeqRecv(uint32(height), channelid, portid)
	if err != nil {
		return nil, err
	}
	return
}

func (sp *SubstrateProvider) QueryPacketCommitment(ctx context.Context, height int64, channelid, portid string, seq uint64) (comRes *chantypes.QueryPacketCommitmentResponse, err error) {
	comRes, err = sp.RPCClient.RPC.IBC.QueryPacketCommitment(height, channelid, portid)
	if err != nil {
		return nil, err
	}
	return
}

func (sp *SubstrateProvider) QueryPacketAcknowledgement(ctx context.Context, height int64, channelid, portid string, seq uint64) (ackRes *chantypes.QueryPacketAcknowledgementResponse, err error) {
	ackRes, err = sp.RPCClient.RPC.IBC.QueryPacketAcknowledgement(uint32(height), channelid, portid, seq)
	if err != nil {
		return nil, err
	}
	return
}

func (sp *SubstrateProvider) QueryPacketReceipt(ctx context.Context, height int64, channelid, portid string, seq uint64) (recRes *chantypes.QueryPacketReceiptResponse, err error) {
	recRes, err = sp.RPCClient.RPC.IBC.QueryPacketReceipt(uint32(height), channelid, portid, seq)
	if err != nil {
		return nil, err
	}
	return
}

func (sp *SubstrateProvider) QueryDenomTrace(ctx context.Context, denom string) (*transfertypes.DenomTrace, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryDenomTrace(denom)
	if err != nil {
		return nil, err
	}

	return res.DenomTrace, err
}

func (sp *SubstrateProvider) QueryDenomTraces(ctx context.Context, offset, limit uint64, height int64) ([]transfertypes.DenomTrace, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryDenomTraces(offset, limit, uint32(height))
	if err != nil {
		return nil, err
	}

	return res.DenomTraces, err
}
