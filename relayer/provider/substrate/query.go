package substrate

import (
	"context"
	"fmt"
	"strings"
	"time"

	rpcClientTypes "github.com/ComposableFi/go-substrate-rpc-client/v4/types"
	beefyClientTypes "github.com/cosmos/ibc-go/v3/modules/light-clients/11-beefy/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	querytypes "github.com/cosmos/cosmos-sdk/types/query"
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v3/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	committypes "github.com/cosmos/ibc-go/v3/modules/core/23-commitment/types"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
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
	var res *clienttypes.QueryClientStateResponse
	// TODO: addr might need to be passed as byte not string
	err := sp.RPCClient.Client.Call(&res, "ibc.queryBalanceWithAddress", addr)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (sp *SubstrateProvider) QueryUnbondingPeriod() (time.Duration, error) {
	return 0, nil
}

func (sp *SubstrateProvider) QueryClientState(height int64, clientid string) (ibcexported.ClientState, error) {
	res, err := sp.QueryClientStateResponse(height, clientid)
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

func (sp *SubstrateProvider) QueryClientStateResponse(height int64, srcClientId string) (*clienttypes.QueryClientStateResponse, error) {
	var res *clienttypes.QueryClientStateResponse
	err := sp.RPCClient.Client.Call(&res, "ibc.queryClientState", height, srcClientId)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (sp *SubstrateProvider) QueryClientConsensusState(chainHeight int64, clientid string, clientHeight ibcexported.Height) (*clienttypes.QueryConsensusStateResponse, error) {
	var res *clienttypes.QueryConsensusStateResponse
	err := sp.RPCClient.Client.Call(&res, "ibc.queryClientConsensusState", clientid,
		clientHeight.GetRevisionHeight(), clientHeight.GetRevisionNumber(), false)
	if err != nil  {
		return nil, err
	}
	return res, nil
}

func (sp *SubstrateProvider) QueryUpgradedClient(height int64) (*clienttypes.QueryClientStateResponse, error) {
	var res *clienttypes.QueryClientStateResponse
	err := sp.RPCClient.Client.Call(&res, "ibc.queryUpgradedClient", height)
	if err != nil  {
		return nil, err
	}
	return res, nil
}

func (sp *SubstrateProvider) QueryUpgradedConsState(height int64) (*clienttypes.QueryConsensusStateResponse, error) {
	var res *clienttypes.QueryConsensusStateResponse
	err := sp.RPCClient.Client.Call(&res, "ibc.queryUpgradedConnectionState", height)
	if err != nil  {
		return nil, err
	}
	return res, nil
}

func (sp *SubstrateProvider) QueryConsensusState(height int64) (ibcexported.ConsensusState, int64, error) {
	var res *clienttypes.QueryConsensusStateResponse
	err := sp.RPCClient.Client.Call(&res, "ibc.queryConsensusState", height)
	if err != nil  {
		return nil, 0, err
	}

	consensusStateExported, err := clienttypes.UnpackConsensusState(res.ConsensusState)
	if err != nil  {
		return nil, 0, err
	}

	return consensusStateExported, height, nil
}

// QueryClients queries all the clients!
// TODO add pagination support
func (sp *SubstrateProvider) QueryClients() (clienttypes.IdentifiedClientStates, error) {
	var res clienttypes.IdentifiedClientStates
	err := sp.RPCClient.Client.Call(&res, "ibc.queryClients")
	if err != nil  {
		return nil, err
	}

	return res, nil
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

func (sp *SubstrateProvider) queryConnection(height int64, connectionID string) (*conntypes.QueryConnectionResponse, error) {
	var res *conntypes.QueryConnectionResponse
	err := sp.RPCClient.Client.Call(&res, "ibc.queryConnection", height, connectionID)
	if err != nil  {
		return nil, err
	}

	return res, nil
}

// QueryConnections gets any connections on a chain
// TODO add pagination support
func (sp *SubstrateProvider) QueryConnections() (conns []*conntypes.IdentifiedConnection, err error) {
	var res *conntypes.QueryConnectionsResponse
	err = sp.RPCClient.Client.Call(&res, "ibc.queryConnections")
	if err != nil  {
		return nil, err
	}

	return res.Connections, nil
}

// QueryConnectionsUsingClient gets any connections that exist between chain and counterparty
// TODO add pagination support
func (sp *SubstrateProvider) QueryConnectionsUsingClient(height int64, clientid string) (*conntypes.QueryConnectionsResponse, error) {
	var res *conntypes.QueryConnectionsResponse
	err := sp.RPCClient.Client.Call(&res, "ibc_queryConnectionUsingClient")
	if err != nil  {
		return nil, err
	}

	return res, nil
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
	var res *clienttypes.IdentifiedClientState
	err := sp.RPCClient.Client.Call(&res, "ibc.queryChannelClient", height, channelid, portid)
	if err != nil  {
		return nil, err
	}

	return res, nil
}

func (sp *SubstrateProvider) QueryConnectionChannels(height int64, connectionid string) ([]*chantypes.IdentifiedChannel, error) {
	var res *chantypes.QueryConnectionChannelsResponse
	err := sp.RPCClient.Client.Call(&res, "ibc.queryConnectionChannels", height, connectionid)
	if err != nil  {
		return nil, err
	}

	return res.Channels, nil
}

func (sp *SubstrateProvider) QueryChannels() ([]*chantypes.IdentifiedChannel, error) {
	var res *chantypes.QueryChannelsResponse
	err := sp.RPCClient.Client.Call(&res, "ibc.queryChannels")
	if err != nil  {
		return nil, err
	}

	return res.Channels, err
}

func (sp *SubstrateProvider) QueryPacketCommitments(height uint64, channelid, portid string) (commitments *chantypes.QueryPacketCommitmentsResponse, err error) {
	var res *chantypes.QueryPacketCommitmentsResponse
	err = sp.RPCClient.Client.Call(&res, "ibc.queryPacketCommitments", height, channelid, portid)
	if err != nil  {
		return nil, err
	}

	return res, err
}

func (sp *SubstrateProvider) QueryPacketAcknowledgements(height uint64, channelid, portid string) (acknowledgements []*chantypes.PacketState, err error) {
	var res *chantypes.QueryPacketAcknowledgementsResponse
	err = sp.RPCClient.Client.Call(&res, "ibc.queryPacketAcknowledgements", height, channelid, portid)
	if err != nil  {
		return nil, err
	}

	return res.Acknowledgements, err
}

func (sp *SubstrateProvider) QueryUnreceivedPackets(height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	var packets []uint64
	err := sp.RPCClient.Client.Call(&packets, "ibc.queryUnreceivedPackets", height, channelid, portid, seqs)
	if err != nil  {
		return nil, err
	}

	return packets, err
}

func (sp *SubstrateProvider) QueryUnreceivedAcknowledgements(height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	var ack []uint64
	err := sp.RPCClient.Client.Call(&ack, "ibc.queryUnreceivedAcknowledgement", height, channelid, portid, seqs)
	if err != nil  {
		return nil, err
	}

	return ack, err
}

func (sp *SubstrateProvider) QueryNextSeqRecv(height int64, channelid, portid string) (recvRes *chantypes.QueryNextSequenceReceiveResponse, err error) {
	err = sp.RPCClient.Client.Call(&recvRes, "ibc.queryNextSeqRecv", height, channelid, portid)
	if err != nil  {
		return nil, err
	}
	return
}

func (sp *SubstrateProvider) QueryPacketCommitment(height int64, channelid, portid string, seq uint64) (comRes *chantypes.QueryPacketCommitmentResponse, err error) {
	err = sp.RPCClient.Client.Call(&comRes, "ibc.queryPacketCommitment", height, channelid, portid)
	if err != nil  {
		return nil, err
	}
	return
}

func (sp *SubstrateProvider) QueryPacketAcknowledgement(height int64, channelid, portid string, seq uint64) (ackRes *chantypes.QueryPacketAcknowledgementResponse, err error) {
	err = sp.RPCClient.Client.Call(&ackRes, "ibc.queryPacketAcknowledgement", height, channelid, portid, seq)
	if err != nil  {
		return nil, err
	}
	return
}

func (sp *SubstrateProvider) QueryPacketReceipt(height int64, channelid, portid string, seq uint64) (recRes *chantypes.QueryPacketReceiptResponse, err error) {
	err = sp.RPCClient.Client.Call(&recRes, "ibc.queryPacketReceipt", height, channelid, portid, seq)
	if err != nil  {
		return nil, err
	}
	return
}

func (sp *SubstrateProvider) QueryDenomTrace(denom string) (*transfertypes.DenomTrace, error) {
	var res *transfertypes.QueryDenomTraceResponse
	err := sp.RPCClient.Client.Call(&res, "ibc.queryDenomTrace", denom)
	if err != nil  {
		return nil, err
	}

	return res.DenomTrace, err
}

func (sp *SubstrateProvider) QueryDenomTraces(offset, limit uint64, height int64) ([]transfertypes.DenomTrace, error) {
	var res *transfertypes.QueryDenomTracesResponse
	err := sp.RPCClient.Client.Call(&res, "ibc.queryDenomTraces", offset, limit, height)
	if err != nil  {
		return nil, err
	}

	return res.DenomTraces, err
}
