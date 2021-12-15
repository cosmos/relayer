package relayer

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/store/rootmulti"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	clienttypes "github.com/cosmos/ibc-go/v2/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v2/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v2/modules/core/04-channel/types"
	committypes "github.com/cosmos/ibc-go/v2/modules/core/23-commitment/types"
	tmclient "github.com/cosmos/ibc-go/v2/modules/light-clients/07-tendermint/types"
	abci "github.com/tendermint/tendermint/abci/types"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	tmtypes "github.com/tendermint/tendermint/types"
	"golang.org/x/sync/errgroup"
)

// NOTE: This file contains logic for querying the Tendermint RPC port of a configured chain
// All the operations here hit the network and data coming back may be untrusted.
// These functions by convention are named Query*

// TODO: Validate all info coming back from these queries using the verifier

// ////////////////////////////
//    ICS 02 -> CLIENTS     //
// ////////////////////////////

// QueryTMClientState retrevies the latest consensus state for a client in state at a given height
// and unpacks/cast it to tendermint clientstate
func (c *Chain) QueryTMClientState(height int64) (*tmclient.ClientState, error) {
	clientStateRes, err := c.ChainProvider.QueryClientStateResponse(height, c.ClientID())
	if err != nil {
		return &tmclient.ClientState{}, err
	}

	return CastClientStateToTMType(clientStateRes.ClientState)
}

// CastClientStateToTMType casts client state to tendermint type
func CastClientStateToTMType(cs *codectypes.Any) (*tmclient.ClientState, error) {
	clientStateExported, err := clienttypes.UnpackClientState(cs)
	if err != nil {
		return &tmclient.ClientState{}, err
	}

	// cast from interface to concrete type
	clientState, ok := clientStateExported.(*tmclient.ClientState)
	if !ok {
		return &tmclient.ClientState{},
			fmt.Errorf("error when casting exported clientstate to tendermint type")
	}

	return clientState, nil
}

// ////////////////////////////
//  ICS 03 -> CONNECTIONS   //
// ////////////////////////////

// QueryConnectionPair returns a pair of connection responses
func QueryConnectionPair(
	src, dst *Chain,
	srcH, dstH int64) (srcConn, dstConn *conntypes.QueryConnectionResponse, err error) {
	var eg = new(errgroup.Group)
	eg.Go(func() error {
		var err error
		srcConn, err = src.ChainProvider.QueryConnection(srcH, src.ConnectionID())
		return err
	})
	eg.Go(func() error {
		dstConn, err = dst.ChainProvider.QueryConnection(dstH, dst.ConnectionID())
		return err
	})
	err = eg.Wait()
	return
}

// ////////////////////////////
//    ICS 04 -> CHANNEL     //
// ////////////////////////////

// QueryChannelPair returns a pair of channel responses
func QueryChannelPair(src, dst *Chain, srcH, dstH int64) (srcChan, dstChan *chantypes.QueryChannelResponse, err error) {
	var eg = new(errgroup.Group)
	eg.Go(func() error {
		var err error
		srcChan, err = src.ChainProvider.QueryChannel(srcH, src.PathEnd.ChannelID, src.PathEnd.PortID)
		return err
	})
	eg.Go(func() error {
		var err error
		dstChan, err = dst.ChainProvider.QueryChannel(dstH, src.PathEnd.ChannelID, src.PathEnd.PortID)
		return err
	})
	err = eg.Wait()
	return
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
	return tmtypes.NewValidator(tmkey, val.ConsensusPower(sdk.DefaultPowerReduction)), nil
}

// QueryUpgradedClient returns upgraded client info
func (c *Chain) QueryUpgradedClient(height int64) (*codectypes.Any, []byte, clienttypes.Height, error) {
	req := clienttypes.QueryUpgradedClientStateRequest{}

	queryClient := clienttypes.NewQueryClient(c.CLIContext(0))

	res, err := queryClient.UpgradedClientState(context.Background(), &req)
	if err != nil {
		return nil, nil, clienttypes.Height{}, err
	}

	if res == nil || res.UpgradedClientState == nil {
		return nil, nil, clienttypes.Height{},
			fmt.Errorf("upgraded client state plan does not exist at height %d", height)
	}
	client := res.UpgradedClientState

	proof, proofHeight, err := c.QueryUpgradeProof(upgradetypes.UpgradedClientKey(height), uint64(height))
	if err != nil {
		return nil, nil, clienttypes.Height{}, err
	}

	return client, proof, proofHeight, nil
}

// QueryUpgradedConsState returns upgraded consensus state and height of client
func (c *Chain) QueryUpgradedConsState(height int64) (*codectypes.Any, []byte, clienttypes.Height, error) {
	req := clienttypes.QueryUpgradedConsensusStateRequest{}

	queryClient := clienttypes.NewQueryClient(c.CLIContext(height))

	res, err := queryClient.UpgradedConsensusState(context.Background(), &req)
	if err != nil {
		return nil, nil, clienttypes.Height{}, err
	}

	if res == nil || res.UpgradedConsensusState == nil {
		return nil, nil, clienttypes.Height{},
			fmt.Errorf("upgraded consensus state plan does not exist at height %d", height)
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
func (c *Chain) QueryUpgradeProof(key []byte, height uint64) ([]byte, clienttypes.Height, error) {
	res, err := c.QueryABCI(abci.RequestQuery{
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

	proof, err := c.Encoding.Marshaler.Marshal(&merkleProof)
	if err != nil {
		return nil, clienttypes.Height{}, err
	}

	revision := clienttypes.ParseChainID(c.ChainID())

	// proof height + 1 is returned as the proof created corresponds to the height the proof
	// was created in the IAVL tree. Tendermint and subsequently the clients that rely on it
	// have heights 1 above the IAVL tree. Thus we return proof height + 1
	return proof, clienttypes.NewHeight(revision, uint64(res.Height+1)), nil
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

	// TODO: figure out how to verify queries?

	return result.Response, nil
}

// QueryLatestHeights returns the heights of multiple chains at once
func QueryLatestHeights(src, dst *Chain) (srch, dsth int64, err error) {
	var eg = new(errgroup.Group)
	eg.Go(func() error {
		var err error
		srch, err = src.ChainProvider.QueryLatestHeight()
		return err
	})
	eg.Go(func() error {
		var err error
		dsth, err = dst.ChainProvider.QueryLatestHeight()
		return err
	})
	err = eg.Wait()
	return
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
