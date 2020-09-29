package relayer

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/store/rootmulti"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	bankTypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	xferTypes "github.com/cosmos/cosmos-sdk/x/ibc-transfer/types"
	clientUtils "github.com/cosmos/cosmos-sdk/x/ibc/02-client/client/utils"
	clientTypes "github.com/cosmos/cosmos-sdk/x/ibc/02-client/types"
	connUtils "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/client/utils"
	connTypes "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/types"
	chanUtils "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/client/utils"
	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint/types"
	commitTypes "github.com/cosmos/cosmos-sdk/x/ibc/23-commitment/types"
	stakingTypes "github.com/cosmos/cosmos-sdk/x/staking/types"
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
		addr = info.GetAddress()
	}

	params := bankTypes.NewQueryAllBalancesRequest(addr, &query.PageRequest{
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
	return clientUtils.QueryNodeConsensusState(c.CLIContext(height))
}

// QueryClientConsensusState retrevies the latest consensus state for a client in state at a given height
func (c *Chain) QueryClientConsensusState(
	height, dstClientConsHeight int64) (*clientTypes.QueryConsensusStateResponse, error) {
	return clientUtils.QueryConsensusStateABCI(
		c.CLIContext(height),
		c.PathEnd.ClientID,
		clientTypes.NewHeight(0, uint64(dstClientConsHeight)),
	)
}

// QueryClientConsensusStatePair allows for the querying of multiple client states at the same time
func QueryClientConsensusStatePair(
	src, dst *Chain,
	srch, dsth, srcClientConsH,
	dstClientConsH int64) (srcCsRes, dstCsRes *clientTypes.QueryConsensusStateResponse, err error) {
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
func (c *Chain) QueryClientState(height int64) (*clientTypes.QueryClientStateResponse, error) {
	return clientUtils.QueryClientStateABCI(c.CLIContext(height), c.PathEnd.ClientID)
}

// QueryClientStatePair returns a pair of connection responses
func QueryClientStatePair(
	src, dst *Chain,
	srch, dsth int64) (srcCsRes, dstCsRes *clientTypes.QueryClientStateResponse, err error) {
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
func (c *Chain) QueryClients(offset, limit uint64) (*clientTypes.QueryClientStatesResponse, error) {
	qc := clientTypes.NewQueryClient(c.CLIContext(0))
	res, err := qc.ClientStates(context.Background(), &clientTypes.QueryClientStatesRequest{
		Pagination: &query.PageRequest{
			Key:        []byte(""),
			Offset:     offset,
			Limit:      limit,
			CountTotal: true,
		},
	})
	return res, err
}

// ////////////////////////////
//  ICS 03 -> CONNECTIONS   //
// ////////////////////////////

// QueryConnections gets any connections on a chain
func (c *Chain) QueryConnections(
	offset, limit uint64) (conns *connTypes.QueryConnectionsResponse, err error) {
	qc := connTypes.NewQueryClient(c.CLIContext(0))
	res, err := qc.Connections(context.Background(), &connTypes.QueryConnectionsRequest{
		Pagination: &query.PageRequest{
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
	height int64) (clientConns *connTypes.QueryClientConnectionsResponse, err error) {
	return connUtils.QueryClientConnections(c.CLIContext(height), c.PathEnd.ClientID, true)
}

// QueryConnection returns the remote end of a given connection
func (c *Chain) QueryConnection(height int64) (*connTypes.QueryConnectionResponse, error) {
	res, err := connUtils.QueryConnection(c.CLIContext(height), c.PathEnd.ConnectionID, true)
	if err != nil && strings.Contains(err.Error(), "not found") {
		return emptyConnRes, nil
	} else if err != nil {
		return nil, err
	}
	return res, nil
}

var emptyConnRes = connTypes.NewQueryConnectionResponse(
	"uninitialized",
	connTypes.NewConnectionEnd(
		connTypes.UNINITIALIZED,
		"client",
		connTypes.NewCounterparty(
			"client",
			"connection",
			commitTypes.NewMerklePrefix([]byte{}),
		),
		[]string{},
	),
	[]byte{},
	clientTypes.NewHeight(0, 0),
)

// QueryConnectionPair returns a pair of connection responses
func QueryConnectionPair(
	src, dst *Chain,
	srcH, dstH int64) (srcConn, dstConn *connTypes.QueryConnectionResponse, err error) {
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
	offset, limit uint64) (*chanTypes.QueryConnectionChannelsResponse, error) {
	qc := chanTypes.NewQueryClient(c.CLIContext(0))
	return qc.ConnectionChannels(context.Background(), &chanTypes.QueryConnectionChannelsRequest{
		Connection: connectionID,
		Pagination: &query.PageRequest{
			Key:        []byte(""),
			Offset:     offset,
			Limit:      limit,
			CountTotal: true,
		},
	})
}

// QueryChannel returns the channel associated with a channelID
func (c *Chain) QueryChannel(height int64) (chanRes *chanTypes.QueryChannelResponse, err error) {
	res, err := chanUtils.QueryChannel(c.CLIContext(height), c.PathEnd.PortID, c.PathEnd.ChannelID, true)
	if err != nil && strings.Contains(err.Error(), "not found") {
		return emptyChannelRes, nil
	} else if err != nil {
		return nil, err
	}
	return res, nil
}

var emptyChannelRes = chanTypes.NewQueryChannelResponse(
	"port",
	"channel",
	chanTypes.NewChannel(
		chanTypes.UNINITIALIZED,
		chanTypes.UNORDERED,
		chanTypes.NewCounterparty(
			"port",
			"channel",
		),
		[]string{},
		"version",
	),
	[]byte{},
	clientTypes.NewHeight(0, 0),
)

// QueryChannelPair returns a pair of channel responses
func QueryChannelPair(src, dst *Chain, srcH, dstH int64) (srcChan, dstChan *chanTypes.QueryChannelResponse, err error) {
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
func (c *Chain) QueryChannels(offset, limit uint64) (*chanTypes.QueryChannelsResponse, error) {
	qc := chanTypes.NewQueryClient(c.CLIContext(0))
	res, err := qc.Channels(context.Background(), &chanTypes.QueryChannelsRequest{
		Pagination: &query.PageRequest{
			Key:        []byte(""),
			Offset:     offset,
			Limit:      limit,
			CountTotal: true,
		},
	})
	return res, err
}

// QueryChannelClient returns the client state of the client supporting a given channel
func (c *Chain) QueryChannelClient() (*chanTypes.QueryChannelClientStateResponse, error) {
	qc := chanTypes.NewQueryClient(c.CLIContext(0))
	return qc.ChannelClientState(context.Background(), &chanTypes.QueryChannelClientStateRequest{
		PortId:    c.PathEnd.PortID,
		ChannelId: c.PathEnd.ChannelID,
	})
}

/////////////////////////////////////
//    TRANSFER -> Denoms           //
/////////////////////////////////////

// QueryDenomTrace takes a denom from IBC and queries the information about it
func (c *Chain) QueryDenomTrace(denom string) (*xferTypes.QueryDenomTraceResponse, error) {
	return xferTypes.NewQueryClient(c.CLIContext(0)).DenomTrace(context.Background(), &xferTypes.QueryDenomTraceRequest{
		Hash: denom,
	})
}

// QueryDenomTraces returns all the denom traces from a given chain
func (c *Chain) QueryDenomTraces(offset, limit uint64) (*xferTypes.QueryDenomTracesResponse, error) {
	return xferTypes.NewQueryClient(c.CLIContext(0)).DenomTraces(context.Background(), &xferTypes.QueryDenomTracesRequest{
		Pagination: &query.PageRequest{
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
func (c *Chain) QueryHistoricalInfo(height clientTypes.Height) (*stakingTypes.QueryHistoricalInfoResponse, error) {
	//TODO: use epoch number in query once SDK gets updated
	qc := stakingTypes.NewQueryClient(c.CLIContext(int64(height.EpochHeight)))
	return qc.HistoricalInfo(context.Background(), &stakingTypes.QueryHistoricalInfoRequest{
		Height: int64(height.EpochHeight),
	})
}

// QueryValsetAtHeight returns the validator set at a given height
func (c *Chain) QueryValsetAtHeight(height clientTypes.Height) (*tmproto.ValidatorSet, error) {
	res, err := c.QueryHistoricalInfo(height)
	if err != nil {
		return nil, err
	}

	// create tendermint ValidatorSet from SDK Validators
	tmVals := stakingTypes.Validators(res.Hist.Valset).ToTmValidators()
	tmValSet := &tmtypes.ValidatorSet{
		Validators: tmVals,
		Proposer:   tmVals[0],
	}

	return tmValSet.ToProto()
}

// QueryUnbondingPeriod returns the unbonding period of the chain
func (c *Chain) QueryUnbondingPeriod() (time.Duration, error) {
	req := stakingTypes.QueryParamsRequest{}

	queryClient := stakingTypes.NewQueryClient(c.CLIContext(0))

	res, err := queryClient.Params(context.Background(), &req)
	if err != nil {
		return 0, err
	}

	return res.Params.UnbondingTime, nil
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
func (c *Chain) QueryNextSeqRecv(height int64) (recvRes *chanTypes.QueryNextSequenceReceiveResponse, err error) {
	return chanUtils.QueryNextSequenceReceive(c.CLIContext(height), c.PathEnd.PortID, c.PathEnd.ChannelID, true)
}

// QueryPacketCommitment returns the packet commitment proof at a given height
func (c *Chain) QueryPacketCommitment(
	height int64, seq uint64) (comRes *chanTypes.QueryPacketCommitmentResponse, err error) {
	return chanUtils.QueryPacketCommitment(c.CLIContext(height), c.PathEnd.PortID, c.PathEnd.ChannelID, seq, true)
}

// QueryPacketCommitments returns an array of packet commitment proofs
func (c *Chain) QueryPacketCommitments(
	offset, limit, height uint64) (comRes *chanTypes.QueryPacketCommitmentsResponse, err error) {
	qc := chanTypes.NewQueryClient(c.CLIContext(int64(height)))
	return qc.PacketCommitments(context.Background(), &chanTypes.QueryPacketCommitmentsRequest{
		PortId:    c.PathEnd.PortID,
		ChannelId: c.PathEnd.ChannelID,
		Pagination: &query.PageRequest{
			Offset:     offset,
			Limit:      limit,
			CountTotal: true,
		},
	})
	// return res.Commitments, err
}

// QueryUnrecievedPackets returns a list of unrelayed packet commitments
func (c *Chain) QueryUnrecievedPackets(height uint64, seqs []uint64) ([]uint64, error) {
	qc := chanTypes.NewQueryClient(c.CLIContext(int64(height)))
	res, err := qc.UnreceivedPackets(context.Background(), &chanTypes.QueryUnreceivedPacketsRequest{
		PortId:                    c.PathEnd.PortID,
		ChannelId:                 c.PathEnd.ChannelID,
		PacketCommitmentSequences: seqs,
	})
	return res.Sequences, err
}

// QueryUnrelayedAcks returns a list of unrelayed packet acks
func (c *Chain) QueryUnrelayedAcks(height uint64, seqs []uint64) ([]uint64, error) {
	qc := chanTypes.NewQueryClient(c.CLIContext(int64(height)))
	res, err := qc.UnrelayedAcks(context.Background(), &chanTypes.QueryUnrelayedAcksRequest{
		PortId:                    c.PathEnd.PortID,
		ChannelId:                 c.PathEnd.ChannelID,
		PacketCommitmentSequences: seqs,
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
