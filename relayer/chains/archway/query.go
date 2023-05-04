package archway

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	abci "github.com/cometbft/cometbft/abci/types"
	tmtypes "github.com/cometbft/cometbft/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	tmclient "github.com/cosmos/ibc-go/v7/modules/light-clients/07-tendermint"

	querytypes "github.com/cosmos/cosmos-sdk/types/query"
	bankTypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	commitmenttypes "github.com/cosmos/ibc-go/v7/modules/core/23-commitment/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/chains/archway/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

const PaginationDelay = 10 * time.Millisecond

func (ap *ArchwayProvider) QueryTx(ctx context.Context, hashHex string) (*provider.RelayerTxResponse, error) {
	hash, err := hex.DecodeString(hashHex)
	if err != nil {
		return nil, err
	}

	resp, err := ap.RPCClient.Tx(ctx, hash, true)
	if err != nil {
		return nil, err
	}

	events := parseEventsFromResponseDeliverTx(resp.TxResult)

	return &provider.RelayerTxResponse{
		Height: resp.Height,
		TxHash: string(hash),
		Code:   resp.TxResult.Code,
		Data:   string(resp.TxResult.Data),
		Events: events,
	}, nil

}
func (ap *ArchwayProvider) QueryTxs(ctx context.Context, page, limit int, events []string) ([]*provider.RelayerTxResponse, error) {
	if len(events) == 0 {
		return nil, errors.New("must declare at least one event to search")
	}

	if page <= 0 {
		return nil, errors.New("page must greater than 0")
	}

	if limit <= 0 {
		return nil, errors.New("limit must greater than 0")
	}

	res, err := ap.RPCClient.TxSearch(ctx, strings.Join(events, " AND "), true, &page, &limit, "")
	if err != nil {
		return nil, err
	}

	// Currently, we only call QueryTxs() in two spots and in both of them we are expecting there to only be,
	// at most, one tx in the response. Because of this we don't want to initialize the slice with an initial size.
	var txResps []*provider.RelayerTxResponse
	for _, tx := range res.Txs {
		relayerEvents := parseEventsFromResponseDeliverTx(tx.TxResult)
		txResps = append(txResps, &provider.RelayerTxResponse{
			Height: tx.Height,
			TxHash: string(tx.Hash),
			Code:   tx.TxResult.Code,
			Data:   string(tx.TxResult.Data),
			Events: relayerEvents,
		})
	}
	return txResps, nil

}

// parseEventsFromResponseDeliverTx parses the events from a ResponseDeliverTx and builds a slice
// of provider.RelayerEvent's.
func parseEventsFromResponseDeliverTx(resp abci.ResponseDeliverTx) []provider.RelayerEvent {
	var events []provider.RelayerEvent

	for _, event := range resp.Events {
		attributes := make(map[string]string)
		for _, attribute := range event.Attributes {
			attributes[string(attribute.Key)] = string(attribute.Value)
		}
		events = append(events, provider.RelayerEvent{
			EventType:  event.Type,
			Attributes: attributes,
		})
	}
	return events
}

func (ap *ArchwayProvider) QueryLatestHeight(ctx context.Context) (int64, error) {

	stat, err := ap.RPCClient.Status(ctx)
	if err != nil {
		return -1, err
	} else if stat.SyncInfo.CatchingUp {
		return -1, fmt.Errorf("node at %s running chain %s not caught up", ap.PCfg.RPCAddr, ap.PCfg.ChainID)
	}
	return stat.SyncInfo.LatestBlockHeight, nil
}

// QueryIBCHeader returns the IBC compatible block header at a specific height.
func (ap *ArchwayProvider) QueryIBCHeader(ctx context.Context, h int64) (provider.IBCHeader, error) {
	if h == 0 {
		return nil, fmt.Errorf("No header at height 0")
	}
	lightBlock, err := ap.LightProvider.LightBlock(ctx, h)
	if err != nil {
		return nil, err
	}
	return provider.TendermintIBCHeader{
		SignedHeader: lightBlock.SignedHeader,
		ValidatorSet: lightBlock.ValidatorSet,
	}, nil

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
	addr, err := ap.ShowAddress(keyName)
	if err != nil {
		return nil, err
	}

	return ap.QueryBalanceWithAddress(ctx, addr)
}

func (ap *ArchwayProvider) QueryBalanceWithAddress(ctx context.Context, address string) (sdk.Coins, error) {
	qc := bankTypes.NewQueryClient(ap)
	p := DefaultPageRequest()
	coins := sdk.Coins{}

	for {
		res, err := qc.AllBalances(ctx, &bankTypes.QueryAllBalancesRequest{
			Address:    address,
			Pagination: p,
		})
		if err != nil {
			return nil, err
		}

		coins = append(coins, res.Balances...)
		next := res.GetPagination().GetNextKey()
		if len(next) == 0 {
			break
		}

		time.Sleep(PaginationDelay)
		p.Key = next
	}
	return coins, nil
}

func DefaultPageRequest() *querytypes.PageRequest {
	return &querytypes.PageRequest{
		Key:        []byte(""),
		Offset:     0,
		Limit:      1000,
		CountTotal: false,
	}
}

// staking
func (ap *ArchwayProvider) QueryUnbondingPeriod(context.Context) (time.Duration, error) {
	return 0, nil
}

// ics 02 - client
func (ap *ArchwayProvider) QueryClientState(ctx context.Context, height int64, clientid string) (ibcexported.ClientState, error) {
	clientStateRes, err := ap.QueryClientStateResponse(ctx, height, clientid)
	if err != nil {
		return nil, err
	}
	// TODO
	fmt.Println(clientStateRes)
	return nil, nil
}

func (ap *ArchwayProvider) QueryClientStateResponse(ctx context.Context, height int64, srcClientId string) (*clienttypes.QueryClientStateResponse, error) {
	clientStateParam := types.NewClientState(srcClientId)

	param, err := json.Marshal(clientStateParam)
	if err != nil {
		return nil, err
	}

	clientState, err := ap.QueryClient.SmartContractState(ctx, &wasmtypes.QuerySmartContractStateRequest{
		Address:   ap.PCfg.IbcHandlerAddress,
		QueryData: param,
	})
	if err != nil {
		return nil, err
	}
	// TODO
	fmt.Println(clientState)
	return nil, nil
}

func (ap *ArchwayProvider) QueryClientConsensusState(ctx context.Context, chainHeight int64, clientid string, clientHeight ibcexported.Height) (*clienttypes.QueryConsensusStateResponse, error) {
	consensusStateParam := types.NewConsensusState(clientid, uint64(chainHeight))

	param, err := json.Marshal(consensusStateParam)
	if err != nil {
		return nil, err
	}

	consensusState, err := ap.QueryClient.SmartContractState(ctx, &wasmtypes.QuerySmartContractStateRequest{
		Address:   ap.PCfg.IbcHandlerAddress,
		QueryData: param,
	})
	if err != nil {
		return nil, err
	}

	// TODO
	fmt.Println(consensusState)

	return nil, nil
}
func (ap *ArchwayProvider) QueryUpgradedClient(ctx context.Context, height int64) (*clienttypes.QueryClientStateResponse, error) {
	return nil, fmt.Errorf("Not implemented for Archway")
}
func (ap *ArchwayProvider) QueryUpgradedConsState(ctx context.Context, height int64) (*clienttypes.QueryConsensusStateResponse, error) {
	return nil, fmt.Errorf("Not implemented for Archway")
}
func (ap *ArchwayProvider) QueryConsensusState(ctx context.Context, height int64) (ibcexported.ConsensusState, int64, error) {

	commit, err := ap.RPCClient.Commit(ctx, &height)
	if err != nil {
		return &tmclient.ConsensusState{}, 0, err
	}

	page := 1
	count := 10_000

	nextHeight := height + 1
	nextVals, err := ap.RPCClient.Validators(ctx, &nextHeight, &page, &count)
	if err != nil {
		return &tmclient.ConsensusState{}, 0, err
	}

	state := &tmclient.ConsensusState{
		Timestamp:          commit.Time,
		Root:               commitmenttypes.NewMerkleRoot(commit.AppHash),
		NextValidatorsHash: tmtypes.NewValidatorSet(nextVals.Validators).Hash(),
	}

	return state, height, nil
}
func (ap *ArchwayProvider) QueryClients(ctx context.Context) (clienttypes.IdentifiedClientStates, error) {
	return nil, fmt.Errorf("Not implemented for Archway")
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
	return nil, fmt.Errorf("Not implemented for Archway")
}
func (ap *ArchwayProvider) QueryDenomTraces(ctx context.Context, offset, limit uint64, height int64) ([]transfertypes.DenomTrace, error) {
	return nil, fmt.Errorf("Not implemented for Archway")
}
