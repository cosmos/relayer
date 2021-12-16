package relayer

import (
	"fmt"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	clienttypes "github.com/cosmos/ibc-go/v2/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v2/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v2/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v2/modules/core/exported"
	tmclient "github.com/cosmos/ibc-go/v2/modules/light-clients/07-tendermint/types"
	"golang.org/x/sync/errgroup"
)

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

// QueryConnectionPair returns a pair of connection responses
func QueryConnectionPair(src, dst *Chain, srcH, dstH int64) (srcConn, dstConn *conntypes.QueryConnectionResponse, err error) {
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

func GetLightSignedHeadersAtHeights(src, dst *Chain, srch, dsth int64) (ibcexported.Header, ibcexported.Header, error) {
	var (
		eg                               = new(errgroup.Group)
		srcUpdateHeader, dstUpdateHeader ibcexported.Header
	)
	eg.Go(func() error {
		var err error
		srcUpdateHeader, err = src.ChainProvider.GetLightSignedHeaderAtHeight(srch)
		return err
	})
	eg.Go(func() error {
		var err error
		dstUpdateHeader, err = dst.ChainProvider.GetLightSignedHeaderAtHeight(dsth)
		return err
	})
	if err := eg.Wait(); err != nil {
		return nil, nil, err
	}
	return srcUpdateHeader, dstUpdateHeader, nil
}

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

//// QueryHistoricalInfo returns historical header data
//func (c *Chain) QueryHistoricalInfo(height clienttypes.Height) (*stakingtypes.QueryHistoricalInfoResponse, error) {
//	//TODO: use epoch number in query once SDK gets updated
//	qc := stakingtypes.NewQueryClient(c.CLIContext(0))
//	return qc.HistoricalInfo(context.Background(), &stakingtypes.QueryHistoricalInfoRequest{
//		Height: int64(height.GetRevisionHeight()),
//	})
//}
//
//// QueryValsetAtHeight returns the validator set at a given height
//func (c *Chain) QueryValsetAtHeight(height clienttypes.Height) (*tmproto.ValidatorSet, error) {
//	res, err := c.QueryHistoricalInfo(height)
//	if err != nil {
//		return nil, fmt.Errorf("chain(%s): %s", c.ChainID, err)
//	}
//
//	// create tendermint ValidatorSet from SDK Validators
//	tmVals, err := c.toTmValidators(res.Hist.Valset)
//	if err != nil {
//		return nil, err
//	}
//
//	sort.Sort(tmtypes.ValidatorsByVotingPower(tmVals))
//	tmValSet := &tmtypes.ValidatorSet{
//		Validators: tmVals,
//	}
//	tmValSet.GetProposer()
//
//	return tmValSet.ToProto()
//}
//
//func (c *Chain) toTmValidators(vals stakingtypes.Validators) ([]*tmtypes.Validator, error) {
//	validators := make([]*tmtypes.Validator, len(vals))
//	var err error
//	for i, val := range vals {
//		validators[i], err = c.toTmValidator(val)
//		if err != nil {
//			return nil, err
//		}
//	}
//
//	return validators, nil
//}
//
//func (c *Chain) toTmValidator(val stakingtypes.Validator) (*tmtypes.Validator, error) {
//	var pk cryptotypes.PubKey
//	if err := c.Encoding.Marshaler.UnpackAny(val.ConsensusPubkey, &pk); err != nil {
//		return nil, err
//	}
//	tmkey, err := cryptocodec.ToTmPubKeyInterface(pk)
//	if err != nil {
//		return nil, fmt.Errorf("pubkey not a tendermint pub key %s", err)
//	}
//	return tmtypes.NewValidator(tmkey, val.ConsensusPower(sdk.DefaultPowerReduction)), nil
//}
