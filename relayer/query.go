package relayer

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/store/rootmulti"
	sdk "github.com/cosmos/cosmos-sdk/types"
	querytypes "github.com/cosmos/cosmos-sdk/types/query"
	bankTypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	transfertypes "github.com/cosmos/cosmos-sdk/x/ibc/applications/transfer/types"
	clientutils "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/client/utils"
	clienttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/types"
	connutils "github.com/cosmos/cosmos-sdk/x/ibc/core/03-connection/client/utils"
	conntypes "github.com/cosmos/cosmos-sdk/x/ibc/core/03-connection/types"
	chanutils "github.com/cosmos/cosmos-sdk/x/ibc/core/04-channel/client/utils"
	chantypes "github.com/cosmos/cosmos-sdk/x/ibc/core/04-channel/types"
	committypes "github.com/cosmos/cosmos-sdk/x/ibc/core/23-commitment/types"
	ibcexported "github.com/cosmos/cosmos-sdk/x/ibc/core/exported"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	abci "github.com/tendermint/tendermint/abci/types"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
	"golang.org/x/sync/errgroup"
)

var eventFormat = "{eventType}.{eventAttribute}={value}"

// NOTE: This file contains logic for querying the Tendermint RPC port of a configured chain
// All the operations here hit the network and data coming back may be untrusted.
// These functions by convention are named Query*

// TODO: Validate all info coming back from these queries using the verifier

// QueryBalance returns the amount of coins in the relayer account
func (c *Chain) QueryBalance(keyName string) (sdk.Coins, error) {
	var (
		err  error
		addr sdk.AccAddress
	)
	if keyName == "" {
		addr = c.MustGetAddress()
	} else {
		info, err := c.Keybase.Key(keyName)
		if err != nil {
			return nil, err
		}
		done := c.UseSDKContext()
		addr = info.GetAddress()
		done()
	}

	params := bankTypes.NewQueryAllBalancesRequest(addr, &querytypes.PageRequest{
		Key:        []byte(""),
		Offset:     0,
		Limit:      1000,
		CountTotal: true,
	})

	queryClient := bankTypes.NewQueryClient(c.CLIContext(0))

	res, err := queryClient.AllBalances(context.Background(), params)
	if err != nil {
		return nil, err
	}

	return res.Balances, nil
}

// ////////////////////////////
//    ICS 02 -> CLIENTS     //
// ////////////////////////////

// QueryConsensusState returns a consensus state for a given chain to be used as a
// client in another chain, fetches latest height when passed 0 as arg
func (c *Chain) QueryConsensusState(height int64) (*tmclient.ConsensusState, int64, error) {
	return clientutils.QueryNodeConsensusState(c.CLIContext(height))
}

// QueryClientConsensusState retrevies the latest consensus state for a client in state at a given height
func (c *Chain) QueryClientConsensusState(
	height int64, dstClientConsHeight ibcexported.Height) (*clienttypes.QueryConsensusStateResponse, error) {
	return clientutils.QueryConsensusStateABCI(
		c.CLIContext(height),
		c.PathEnd.ClientID,
		dstClientConsHeight,
	)
}

// QueryClientConsensusStatePair allows for the querying of multiple client states at the same time
func QueryClientConsensusStatePair(
	src, dst *Chain,
	srch, dsth int64, srcClientConsH,
	dstClientConsH ibcexported.Height) (srcCsRes, dstCsRes *clienttypes.QueryConsensusStateResponse, err error) {
	var eg = new(errgroup.Group)
	eg.Go(func() error {
		srcCsRes, err = src.QueryClientConsensusState(srch, srcClientConsH)
		return err
	})
	eg.Go(func() error {
		dstCsRes, err = dst.QueryClientConsensusState(dsth, dstClientConsH)
		return err
	})
	err = eg.Wait()
	return
}

// QueryClientState retrevies the latest consensus state for a client in state at a given height
func (c *Chain) QueryClientState(height int64) (*clienttypes.QueryClientStateResponse, error) {
	return clientutils.QueryClientStateABCI(c.CLIContext(height), c.PathEnd.ClientID)
}

// QueryClientStatePair returns a pair of connection responses
func QueryClientStatePair(
	src, dst *Chain,
	srch, dsth int64) (srcCsRes, dstCsRes *clienttypes.QueryClientStateResponse, err error) {
	var eg = new(errgroup.Group)
	eg.Go(func() error {
		srcCsRes, err = src.QueryClientState(srch)
		return err
	})
	eg.Go(func() error {
		dstCsRes, err = dst.QueryClientState(dsth)
		return err
	})
	err = eg.Wait()
	return
}

// QueryClients queries all the clients!
func (c *Chain) QueryClients(offset, limit uint64) (*clienttypes.QueryClientStatesResponse, error) {
	qc := clienttypes.NewQueryClient(c.CLIContext(0))
	return qc.ClientStates(context.Background(), &clienttypes.QueryClientStatesRequest{
		Pagination: &querytypes.PageRequest{
			Key:        []byte(""),
			Offset:     offset,
			Limit:      limit,
			CountTotal: true,
		},
	})
}

// ////////////////////////////
//  ICS 03 -> CONNECTIONS   //
// ////////////////////////////

// QueryConnections gets any connections on a chain
func (c *Chain) QueryConnections(
	offset, limit uint64) (conns *conntypes.QueryConnectionsResponse, err error) {
	qc := conntypes.NewQueryClient(c.CLIContext(0))
	res, err := qc.Connections(context.Background(), &conntypes.QueryConnectionsRequest{
		Pagination: &querytypes.PageRequest{
			Key:        []byte(""),
			Offset:     offset,
			Limit:      limit,
			CountTotal: true,
		},
	})
	return res, err
}

// QueryConnectionsUsingClient gets any connections that exist between chain and counterparty
func (c *Chain) QueryConnectionsUsingClient(
	height int64) (clientConns *conntypes.QueryClientConnectionsResponse, err error) {
	return connutils.QueryClientConnections(c.CLIContext(height), c.PathEnd.ClientID, true)
}

// QueryConnection returns the remote end of a given connection
func (c *Chain) QueryConnection(height int64) (*conntypes.QueryConnectionResponse, error) {
	res, err := connutils.QueryConnection(c.CLIContext(height), c.PathEnd.ConnectionID, true)
	if err != nil && strings.Contains(err.Error(), "not found") {
		return emptyConnRes, nil
	} else if err != nil {
		return nil, err
	}
	return res, nil
}

var emptyConnRes = conntypes.NewQueryConnectionResponse(
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
	),
	[]byte{},
	clienttypes.NewHeight(0, 0),
)

// QueryConnectionPair returns a pair of connection responses
func QueryConnectionPair(
	src, dst *Chain,
	srcH, dstH int64) (srcConn, dstConn *conntypes.QueryConnectionResponse, err error) {
	var eg = new(errgroup.Group)
	eg.Go(func() error {
		srcConn, err = src.QueryConnection(srcH)
		return err
	})
	eg.Go(func() error {
		dstConn, err = dst.QueryConnection(dstH)
		return err
	})
	err = eg.Wait()
	return
}

// ////////////////////////////
//    ICS 04 -> CHANNEL     //
// ////////////////////////////

// QueryConnectionChannels queries the channels associated with a connection
func (c *Chain) QueryConnectionChannels(
	connectionID string,
	offset, limit uint64) (*chantypes.QueryConnectionChannelsResponse, error) {
	qc := chantypes.NewQueryClient(c.CLIContext(0))
	return qc.ConnectionChannels(context.Background(), &chantypes.QueryConnectionChannelsRequest{
		Connection: connectionID,
		Pagination: &querytypes.PageRequest{
			Key:        []byte(""),
			Offset:     offset,
			Limit:      limit,
			CountTotal: true,
		},
	})
}

// QueryChannel returns the channel associated with a channelID
func (c *Chain) QueryChannel(height int64) (chanRes *chantypes.QueryChannelResponse, err error) {
	res, err := chanutils.QueryChannel(c.CLIContext(height), c.PathEnd.PortID, c.PathEnd.ChannelID, true)
	if err != nil && strings.Contains(err.Error(), "not found") {
		return emptyChannelRes, nil
	} else if err != nil {
		return nil, err
	}
	return res, nil
}

var emptyChannelRes = chantypes.NewQueryChannelResponse(
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
	clienttypes.NewHeight(0, 0),
)

// QueryChannelPair returns a pair of channel responses
func QueryChannelPair(src, dst *Chain, srcH, dstH int64) (srcChan, dstChan *chantypes.QueryChannelResponse, err error) {
	var eg = new(errgroup.Group)
	eg.Go(func() error {
		srcChan, err = src.QueryChannel(srcH)
		return err
	})
	eg.Go(func() error {
		dstChan, err = dst.QueryChannel(dstH)
		return err
	})
	err = eg.Wait()
	return
}

// QueryChannels returns all the channels that are registered on a chain
func (c *Chain) QueryChannels(offset, limit uint64) (*chantypes.QueryChannelsResponse, error) {
	qc := chantypes.NewQueryClient(c.CLIContext(0))
	res, err := qc.Channels(context.Background(), &chantypes.QueryChannelsRequest{
		Pagination: &querytypes.PageRequest{
			Key:        []byte(""),
			Offset:     offset,
			Limit:      limit,
			CountTotal: true,
		},
	})
	return res, err
}

// QueryChannelClient returns the client state of the client supporting a given channel
func (c *Chain) QueryChannelClient(height int64) (*chantypes.QueryChannelClientStateResponse, error) {
	qc := chantypes.NewQueryClient(c.CLIContext(height))
	return qc.ChannelClientState(context.Background(), &chantypes.QueryChannelClientStateRequest{
		PortId:    c.PathEnd.PortID,
		ChannelId: c.PathEnd.ChannelID,
	})
}

/////////////////////////////////////
//    TRANSFER -> Denoms           //
/////////////////////////////////////

// QueryDenomTrace takes a denom from IBC and queries the information about it
func (c *Chain) QueryDenomTrace(denom string) (*transfertypes.QueryDenomTraceResponse, error) {
	return transfertypes.NewQueryClient(c.CLIContext(0)).DenomTrace(context.Background(), &transfertypes.QueryDenomTraceRequest{
		Hash: denom,
	})
}

// QueryDenomTraces returns all the denom traces from a given chain
func (c *Chain) QueryDenomTraces(offset, limit uint64, height int64) (*transfertypes.QueryDenomTracesResponse, error) {
	return transfertypes.NewQueryClient(c.CLIContext(height)).DenomTraces(context.Background(), &transfertypes.QueryDenomTracesRequest{
		Pagination: &querytypes.PageRequest{
			Key:        []byte(""),
			Offset:     offset,
			Limit:      limit,
			CountTotal: true,
		},
	})
}

/////////////////////////////////////
//    STAKING -> HistoricalInfo     //
/////////////////////////////////////

// QueryHistoricalInfo returns historical header data
func (c *Chain) QueryHistoricalInfo(height clienttypes.Height) (*stakingtypes.QueryHistoricalInfoResponse, error) {
	//TODO: use epoch number in query once SDK gets updated
	qc := stakingtypes.NewQueryClient(c.CLIContext(0))
	return qc.HistoricalInfo(context.Background(), &stakingtypes.QueryHistoricalInfoRequest{
		Height: int64(height.GetRevisionHeight()),
	})
}

// QueryValsetAtHeight returns the validator set at a given height
func (c *Chain) QueryValsetAtHeight(height clienttypes.Height) (*tmproto.ValidatorSet, error) {
	res, err := c.QueryHistoricalInfo(height)
	if err != nil {
		return nil, fmt.Errorf("chain(%s): %s", c.ChainID, err)
	}

	// create tendermint ValidatorSet from SDK Validators
	tmVals, err := c.toTmValidators(res.Hist.Valset)
	if err != nil {
		return nil, err
	}

	sort.Sort(tmtypes.ValidatorsByVotingPower(tmVals))
	tmValSet := &tmtypes.ValidatorSet{
		Validators: tmVals,
	}
	tmValSet.GetProposer()

	return tmValSet.ToProto()
}

func (c *Chain) toTmValidators(vals stakingtypes.Validators) ([]*tmtypes.Validator, error) {
	validators := make([]*tmtypes.Validator, len(vals))
	var err error
	for i, val := range vals {
		validators[i], err = c.toTmValidator(val)
		if err != nil {
			return nil, err
		}
	}

	return validators, nil
}

func (c *Chain) toTmValidator(val stakingtypes.Validator) (*tmtypes.Validator, error) {
	var pk cryptotypes.PubKey
	if err := c.Encoding.Marshaler.UnpackAny(val.ConsensusPubkey, &pk); err != nil {
		return nil, err
	}
	tmkey, err := cryptocodec.ToTmPubKeyInterface(pk)
	if err != nil {
		return nil, fmt.Errorf("pubkey not a tendermint pub key %s", err)
	}
	return tmtypes.NewValidator(tmkey, val.ConsensusPower()), nil
}

// QueryUnbondingPeriod returns the unbonding period of the chain
func (c *Chain) QueryUnbondingPeriod() (time.Duration, error) {
	req := stakingtypes.QueryParamsRequest{}

	queryClient := stakingtypes.NewQueryClient(c.CLIContext(0))

	res, err := queryClient.Params(context.Background(), &req)
	if err != nil {
		return 0, err
	}

	return res.Params.UnbondingTime, nil
}

/////////////////////////////////////
//          UPGRADES               //
/////////////////////////////////////

func (c *Chain) QueryUpgradedClient(height int64) (*codectypes.Any, []byte, clienttypes.Height, error) {
	req := upgradetypes.QueryCurrentPlanRequest{}

	queryClient := upgradetypes.NewQueryClient(c.CLIContext(0))

	res, err := queryClient.CurrentPlan(context.Background(), &req)
	if err != nil {
		return nil, nil, clienttypes.Height{}, err
	}

	if res == nil || res.Plan == nil || res.Plan.UpgradedClientState == nil {
		return nil, nil, clienttypes.Height{}, fmt.Errorf("upgraded client state plan does not exist at height %d", height)
	}
	client := res.Plan.UpgradedClientState

	proof, proofHeight, err := c.QueryUpgradeProof(upgradetypes.UpgradedClientKey(height), uint64(height))
	if err != nil {
		return nil, nil, clienttypes.Height{}, err
	}

	return client, proof, proofHeight, nil
}

func (c *Chain) QueryUpgradedConsState(height int64) (*codectypes.Any, []byte, clienttypes.Height, error) {
	req := upgradetypes.QueryUpgradedConsensusStateRequest{
		LastHeight: height,
	}

	queryClient := upgradetypes.NewQueryClient(c.CLIContext(0))

	res, err := queryClient.UpgradedConsensusState(context.Background(), &req)
	if err != nil {
		return nil, nil, clienttypes.Height{}, err
	}

	if res == nil || res.UpgradedConsensusState == nil {
		return nil, nil, clienttypes.Height{}, fmt.Errorf("upgraded consensus state plan does not exist at height %d", height)
	}
	consState := res.UpgradedConsensusState

	proof, proofHeight, err := c.QueryUpgradeProof(upgradetypes.UpgradedConsStateKey(height), uint64(height))
	if err != nil {
		return nil, nil, clienttypes.Height{}, err
	}

	return consState, proof, proofHeight, nil
}

// QueryUpgradeProof performs an abci query with the given key and returns the proto encoded merkle proof
// for the query and the height at which the proof will succeed on a tendermint verifier.
func (chain *Chain) QueryUpgradeProof(key []byte, height uint64) ([]byte, clienttypes.Height, error) {
	res, err := chain.QueryABCI(abci.RequestQuery{
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

	proof, err := chain.Encoding.Marshaler.MarshalBinaryBare(&merkleProof)
	if err != nil {
		return nil, clienttypes.Height{}, err
	}

	revision := clienttypes.ParseChainID(chain.ChainID)

	// proof height + 1 is returned as the proof created corresponds to the height the proof
	// was created in the IAVL tree. Tendermint and subsequently the clients that rely on it
	// have heights 1 above the IAVL tree. Thus we return proof height + 1
	return proof, clienttypes.NewHeight(revision, uint64(res.Height+1)), nil
}

// WaitForNBlocks blocks until the next block on a given chain
func (c *Chain) WaitForNBlocks(n int64) error {
	var initial int64
	h, err := c.Client.Status(context.Background())
	if err != nil {
		return err
	}
	if h.SyncInfo.CatchingUp {
		return fmt.Errorf("chain catching up")
	}
	initial = h.SyncInfo.LatestBlockHeight
	for {
		h, err = c.Client.Status(context.Background())
		if err != nil {
			return err
		}
		if h.SyncInfo.LatestBlockHeight > initial+n {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// QueryNextSeqRecv returns the next seqRecv for a configured channel
func (c *Chain) QueryNextSeqRecv(height int64) (recvRes *chantypes.QueryNextSequenceReceiveResponse, err error) {
	return chanutils.QueryNextSequenceReceive(c.CLIContext(height), c.PathEnd.PortID, c.PathEnd.ChannelID, true)
}

// QueryPacketCommitment returns the packet commitment proof at a given height
func (c *Chain) QueryPacketCommitment(
	height int64, seq uint64) (comRes *chantypes.QueryPacketCommitmentResponse, err error) {
	return chanutils.QueryPacketCommitment(c.CLIContext(height), c.PathEnd.PortID, c.PathEnd.ChannelID, seq, true)
}

// QueryPacketAcknowledgement returns the packet ack proof at a given height
func (c *Chain) QueryPacketAcknowledgement(height int64, seq uint64) (ackRes *chantypes.QueryPacketAcknowledgementResponse, err error) {
	return chanutils.QueryPacketAcknowledgement(c.CLIContext(height), c.PathEnd.PortID, c.PathEnd.ChannelID, seq, true)
}

// QueryPacketReciept returns the packet reciept proof at a given height
func (c *Chain) QueryPacketReciept(height int64, seq uint64) (recRes *chantypes.QueryPacketReceiptResponse, err error) {
	return chanutils.QueryPacketReceipt(c.CLIContext(height), c.PathEnd.PortID, c.PathEnd.ChannelID, seq, true)
}

// QueryPacketCommitments returns an array of packet commitments
func (c *Chain) QueryPacketCommitments(
	offset, limit, height uint64) (comRes *chantypes.QueryPacketCommitmentsResponse, err error) {
	qc := chantypes.NewQueryClient(c.CLIContext(int64(height)))
	return qc.PacketCommitments(context.Background(), &chantypes.QueryPacketCommitmentsRequest{
		PortId:    c.PathEnd.PortID,
		ChannelId: c.PathEnd.ChannelID,
		Pagination: &querytypes.PageRequest{
			Offset:     offset,
			Limit:      limit,
			CountTotal: true,
		},
	})
}

// QueryPacketAcknowledgements returns an array of packet acks
func (c *Chain) QueryPacketAcknowledgements(offset, limit, height uint64) (comRes *chantypes.QueryPacketAcknowledgementsResponse, err error) {
	qc := chantypes.NewQueryClient(c.CLIContext(int64(height)))
	return qc.PacketAcknowledgements(context.Background(), &chantypes.QueryPacketAcknowledgementsRequest{
		PortId:    c.PathEnd.PortID,
		ChannelId: c.PathEnd.ChannelID,
		Pagination: &querytypes.PageRequest{
			Offset:     offset,
			Limit:      limit,
			CountTotal: true,
		},
	})
}

// QueryUnrecievedPackets returns a list of unrelayed packet commitments
func (c *Chain) QueryUnrecievedPackets(height uint64, seqs []uint64) ([]uint64, error) {
	qc := chantypes.NewQueryClient(c.CLIContext(int64(height)))
	res, err := qc.UnreceivedPackets(context.Background(), &chantypes.QueryUnreceivedPacketsRequest{
		PortId:                    c.PathEnd.PortID,
		ChannelId:                 c.PathEnd.ChannelID,
		PacketCommitmentSequences: seqs,
	})
	return res.Sequences, err
}

// QueryUnrecievedAcknowledgements returns a list of unrelayed packet acks
func (c *Chain) QueryUnrecievedAcknowledgements(height uint64, seqs []uint64) ([]uint64, error) {
	qc := chantypes.NewQueryClient(c.CLIContext(int64(height)))
	res, err := qc.UnreceivedAcks(context.Background(), &chantypes.QueryUnreceivedAcksRequest{
		PortId:             c.PathEnd.PortID,
		ChannelId:          c.PathEnd.ChannelID,
		PacketAckSequences: seqs,
	})
	return res.Sequences, err
}

// QueryTx takes a transaction hash and returns the transaction
func (c *Chain) QueryTx(hashHex string) (*ctypes.ResultTx, error) {
	hash, err := hex.DecodeString(hashHex)
	if err != nil {
		return &ctypes.ResultTx{}, err
	}

	return c.Client.Tx(context.Background(), hash, true)
}

// QueryTxs returns an array of transactions given a tag
func (c *Chain) QueryTxs(height uint64, page, limit int, events []string) ([]*ctypes.ResultTx, error) {
	if len(events) == 0 {
		return nil, errors.New("must declare at least one event to search")
	}

	if page <= 0 {
		return nil, errors.New("page must greater than 0")
	}

	if limit <= 0 {
		return nil, errors.New("limit must greater than 0")
	}

	res, err := c.Client.TxSearch(context.Background(), strings.Join(events, " AND "), true, &page, &limit, "")
	if err != nil {
		return nil, err
	}
	return res.Txs, nil
}

// QueryABCI is an affordance for querying the ABCI server associated with a chain
// Similar to cliCtx.QueryABCI
func (c *Chain) QueryABCI(req abci.RequestQuery) (res abci.ResponseQuery, err error) {
	opts := rpcclient.ABCIQueryOptions{
		Height: req.GetHeight(),
		Prove:  req.Prove,
	}

	result, err := c.Client.ABCIQueryWithOptions(context.Background(), req.Path, req.Data, opts)
	if err != nil {
		// retry queries on EOF
		if strings.Contains(err.Error(), "EOF") {
			if c.debug {
				c.Error(err)
			}
			return c.QueryABCI(req)
		}
		return res, err
	}

	if !result.Response.IsOK() {
		return res, errors.New(result.Response.Log)
	}

	// data from trusted node or subspace query doesn't need verification
	if !isQueryStoreWithProof(req.Path) {
		return result.Response, nil
	}

	if err = c.VerifyProof(req.Path, result.Response); err != nil {
		return res, err
	}

	return result.Response, nil
}

// QueryWithData satisfies auth.NodeQuerier interface and used for fetching account details
func (c *Chain) QueryWithData(p string, d []byte) (byt []byte, i int64, err error) {
	var res abci.ResponseQuery
	if res, err = c.QueryABCI(abci.RequestQuery{Path: p, Height: 0, Data: d}); err != nil {
		return byt, i, err
	}

	return res.Value, res.Height, nil
}

// QueryLatestHeight queries the chain for the latest height and returns it
func (c *Chain) QueryLatestHeight() (int64, error) {
	res, err := c.Client.Status(context.Background())
	if err != nil {
		return -1, err
	} else if res.SyncInfo.CatchingUp {
		return -1, fmt.Errorf("node at %s running chain %s not caught up", c.RPCAddr, c.ChainID)
	}

	return res.SyncInfo.LatestBlockHeight, nil
}

// QueryLatestHeights returns the heights of multiple chains at once
func QueryLatestHeights(src, dst *Chain) (srch, dsth int64, err error) {
	var eg = new(errgroup.Group)
	eg.Go(func() error {
		srch, err = src.QueryLatestHeight()
		return err
	})
	eg.Go(func() error {
		dsth, err = dst.QueryLatestHeight()
		return err
	})
	err = eg.Wait()
	return
}

// QueryLatestHeader returns the latest header from the chain
func (c *Chain) QueryLatestHeader() (out *tmclient.Header, err error) {
	var h int64
	if h, err = c.QueryLatestHeight(); err != nil {
		return nil, err
	}
	return c.QueryHeaderAtHeight(h)
}

// QueryHeaderAtHeight returns the header at a given height
func (c *Chain) QueryHeaderAtHeight(height int64) (*tmclient.Header, error) {
	var (
		page    int = 1
		perPage int = 100000
	)
	if height <= 0 {
		return nil, fmt.Errorf("must pass in valid height, %d not valid", height)
	}

	res, err := c.Client.Commit(context.Background(), &height)
	if err != nil {
		return nil, err
	}

	val, err := c.Client.Validators(context.Background(), &height, &page, &perPage)
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

// isQueryStoreWithProof expects a format like /<queryType>/<storeName>/<subpath>
// queryType must be "store" and subpath must be "key" to require a proof.
func isQueryStoreWithProof(path string) bool {
	if !strings.HasPrefix(path, "/") {
		return false
	}

	paths := strings.SplitN(path[1:], "/", 3)
	switch {
	case len(paths) != 3:
		return false
	case paths[0] != "store":
		return false
	case rootmulti.RequireProof("/" + paths[2]):
		return true
	}

	return false
}

// ParseEvents takes events in the query format and reutrns
func ParseEvents(e string) ([]string, error) {
	eventsStr := strings.Trim(e, "'")
	var events []string
	if strings.Contains(eventsStr, "&") {
		events = strings.Split(eventsStr, "&")
	} else {
		events = append(events, eventsStr)
	}

	var tmEvents = make([]string, len(events))

	for i, event := range events {
		if !strings.Contains(event, "=") {
			return []string{}, fmt.Errorf("invalid event; event %s should be of the format: %s", event, eventFormat)
		} else if strings.Count(event, "=") > 1 {
			return []string{}, fmt.Errorf("invalid event; event %s should be of the format: %s", event, eventFormat)
		}

		tokens := strings.Split(event, "=")
		if tokens[0] == tmtypes.TxHeightKey {
			event = fmt.Sprintf("%s=%s", tokens[0], tokens[1])
		} else {
			event = fmt.Sprintf("%s='%s'", tokens[0], tokens[1])
		}

		tmEvents[i] = event
	}
	return tmEvents, nil
}
