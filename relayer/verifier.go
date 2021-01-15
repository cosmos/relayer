package relayer

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
	dbm "github.com/tendermint/tm-db"

	retry "github.com/avast/retry-go"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	light "github.com/tendermint/tendermint/light"
	lightp "github.com/tendermint/tendermint/light/provider"
	lighthttp "github.com/tendermint/tendermint/light/provider/http"
	dbs "github.com/tendermint/tendermint/light/store/db"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
	"golang.org/x/sync/errgroup"
)

// NOTE: currently we are discarding the very noisy light client logs
// it would be nice if we could add a setting the chain or otherwise
// that allowed users to enable light client logging. (maybe as a hidden prop
// on the Chain struct that users could pass in the config??)
var logger = light.Logger(log.NewTMLogger(log.NewSyncWriter(ioutil.Discard)))

// InjectTrustedFieldsHeaders takes the headers and enriches them
func InjectTrustedFieldsHeaders(
	src, dst *Chain,
	srch, dsth *tmclient.Header) (srcho *tmclient.Header, dstho *tmclient.Header, err error) {
	var eg = new(errgroup.Group)
	eg.Go(func() error {
		srcho, err = InjectTrustedFields(src, dst, srch)
		return err
	})
	eg.Go(func() error {
		dstho, err = InjectTrustedFields(dst, src, dsth)
		return err
	})
	if err = eg.Wait(); err != nil {
		return
	}
	return
}

// UpdatesWithHeaders calls UpdateLightWithHeader on the passed chains concurrently
func UpdatesWithHeaders(src, dst *Chain) (srch, dsth *tmclient.Header, err error) {
	var eg = new(errgroup.Group)
	eg.Go(func() error {
		srch, err = src.QueryLatestHeader()
		return err
	})
	eg.Go(func() error {
		dsth, err = dst.QueryLatestHeader()
		return err
	})
	if err := eg.Wait(); err != nil {
		return nil, nil, err
	}
	return srch, dsth, nil
}

func lightError(err error) error { return fmt.Errorf("light client: %w", err) }

// UpdateLightWithHeader calls client.Update and then .
func (c *Chain) UpdateLightWithHeader() (*tmclient.Header, error) {
	// create database connection
	db, df, err := c.NewLightDB()
	if err != nil {
		return nil, lightError(err)
	}
	defer df()

	client, err := c.LightClient(db)
	if err != nil {
		return nil, lightError(err)
	}

	sh, err := client.Update(context.Background(), time.Now())
	if err != nil {
		return nil, lightError(err)
	}

	if sh == nil {
		sh, err = client.TrustedLightBlock(0)
		if err != nil {
			return nil, lightError(err)
		}
	}

	protoVal, err := tmtypes.NewValidatorSet(sh.ValidatorSet.Validators).ToProto()
	if err != nil {
		return nil, err
	}

	return &tmclient.Header{
		SignedHeader: sh.SignedHeader.ToProto(),
		ValidatorSet: protoVal,
	}, nil
}

// LightHTTP returns the http client for light clients
func (c *Chain) LightHTTP() lightp.Provider {
	cl, err := lighthttp.New(c.ChainID, c.RPCAddr)
	if err != nil {
		panic(err)
	}
	return cl
}

// LightClientWithoutTrust querys the latest header from the chain and initializes a new light client
// database using that header. This should only be called when first initializing the light client
func (c *Chain) LightClientWithoutTrust(db dbm.DB) (*light.Client, error) {
	var (
		height int64
		err    error
	)
	prov := c.LightHTTP()

	if err := retry.Do(func() error {
		height, err = c.QueryLatestHeight()
		switch {
		case err != nil:
			return err
		case height == 0:
			return fmt.Errorf("shouldn't be here")
		default:
			return nil
		}
	}, rtyAtt, rtyDel, rtyErr); err != nil {
		return nil, err
	}

	lb, err := prov.LightBlock(context.Background(), height)
	if err != nil {
		return nil, err
	}
	return light.NewClient(
		context.Background(),
		c.ChainID,
		light.TrustOptions{
			Period: c.GetTrustingPeriod(),
			Height: height,
			Hash:   lb.SignedHeader.Hash(),
		},
		prov,
		// TODO: provide actual witnesses!
		// NOTE: This requires adding them to the chain config
		[]lightp.Provider{prov},
		dbs.New(db, ""),
		logger)
}

// LightClientWithTrust takes a header from the chain and attempts to add that header to the light
// database.
func (c *Chain) LightClientWithTrust(db dbm.DB, to light.TrustOptions) (*light.Client, error) {
	prov := c.LightHTTP()
	return light.NewClient(
		context.Background(),
		c.ChainID,
		to,
		prov,
		// TODO: provide actual witnesses!
		// NOTE: This requires adding them to the chain config
		[]lightp.Provider{prov},
		dbs.New(db, ""),
		logger)
}

// LightClient initializes the light client for a given chain from the trusted store in the database
// this should be call for all other light client usage
func (c *Chain) LightClient(db dbm.DB) (*light.Client, error) {
	prov := c.LightHTTP()
	return light.NewClientFromTrustedStore(
		c.ChainID,
		c.GetTrustingPeriod(),
		prov,
		// TODO: provide actual witnesses!
		// NOTE: This requires adding them to the chain config
		[]lightp.Provider{prov},
		dbs.New(db, ""),
		logger,
	)
}

// NewLightDB returns a new instance of the lightclient database connection
// CONTRACT: must close the database connection when done with it (defer df())
func (c *Chain) NewLightDB() (db *dbm.GoLevelDB, df func(), err error) {
	if err := retry.Do(func() error {
		db, err = dbm.NewGoLevelDB(c.ChainID, lightDir(c.HomePath))
		if err != nil {
			return fmt.Errorf("can't open light client database: %w", err)
		}
		return nil
	}, rtyAtt, rtyDel, rtyErr); err != nil {
		return nil, nil, err
	}

	df = func() {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}

	return
}

// DeleteLightDB removes the light client database on disk, forcing re-initialization
func (c *Chain) DeleteLightDB() error {
	return os.RemoveAll(filepath.Join(lightDir(c.HomePath), fmt.Sprintf("%s.db", c.ChainID)))
}

// TrustOptions returns light.TrustOptions given a height and hash
func (c *Chain) TrustOptions(height int64, hash []byte) light.TrustOptions {
	return light.TrustOptions{
		Period: c.GetTrustingPeriod(),
		Height: height,
		Hash:   hash,
	}
}

// GetLatestLightHeader returns the header to be used for client creation
func (c *Chain) GetLatestLightHeader() (*tmclient.Header, error) {
	return c.GetLightSignedHeaderAtHeight(0)
}

// VerifyProof performs response proof verification.
func (c *Chain) VerifyProof(queryPath string, resp abci.ResponseQuery) error {
	// TODO: write this verify function
	return nil
}

// ValidateTxResult takes a transaction and validates the proof against a stored root of trust
func (c *Chain) ValidateTxResult(resTx *ctypes.ResultTx) (err error) {
	// fetch the header at the height from the ResultTx from the light database
	check, err := c.GetLightSignedHeaderAtHeight(resTx.Height - 1)
	if err != nil {
		return
	}

	// validate the proof against that header
	return resTx.Proof.Validate(check.Header.DataHash)
}

// GetLatestLightHeights returns both the src and dst latest height in the local client
func GetLatestLightHeights(src, dst *Chain) (srch int64, dsth int64, err error) {
	var eg = new(errgroup.Group)
	eg.Go(func() error {
		srch, err = src.GetLatestLightHeight()
		return err
	})
	eg.Go(func() error {
		dsth, err = dst.GetLatestLightHeight()
		return err
	})
	if err = eg.Wait(); err != nil {
		return
	}
	return
}

// GetLatestLightHeight uses the CLI utilities to pull the latest height from a given chain
func (c *Chain) GetLatestLightHeight() (int64, error) {
	db, df, err := c.NewLightDB()
	if err != nil {
		return -1, err
	}
	defer df()

	client, err := c.LightClient(db)
	if err != nil {
		return -1, err
	}

	return client.LastTrustedHeight()
}

// GetLightSignedHeaderAtHeight returns a signed header at a particular height.
func (c *Chain) GetLightSignedHeaderAtHeight(height int64) (*tmclient.Header, error) {
	// create database connection
	db, df, err := c.NewLightDB()
	if err != nil {
		return nil, err
	}
	defer df()

	client, err := c.LightClient(db)
	if err != nil {
		return nil, err
	}

	sh, err := client.TrustedLightBlock(height)
	if err != nil {
		return nil, err
	}

	protoVal, err := tmtypes.NewValidatorSet(sh.ValidatorSet.Validators).ToProto()
	if err != nil {
		return nil, err
	}

	return &tmclient.Header{SignedHeader: sh.SignedHeader.ToProto(), ValidatorSet: protoVal}, nil
}

// ErrLightNotInitialized returns the canonical error for a an uninitialized light client
var ErrLightNotInitialized = errors.New("light client is not initialized")

// ForceInitLight forces initialization of the light client from the configured node
func (c *Chain) ForceInitLight() error {
	db, df, err := c.NewLightDB()
	if err != nil {
		return err
	}
	_, err = c.LightClientWithoutTrust(db)
	if err != nil {
		return err
	}
	df()
	return nil
}

// ValidateLightInitialized returns an error if the light client isn't initialized or there is a problem
// interacting with the light client.
func (c *Chain) ValidateLightInitialized() error {
	height, err := c.GetLatestLightHeight()
	if err != nil {
		return fmt.Errorf("encountered issue with off chain light client for chain (%s): %v", c.ChainID, err)
	}

	// height will return -1 when the client has not been initialized
	if height == -1 {
		return fmt.Errorf("please initialize an off chain light client for chain (%s)", c.ChainID)
	}

	return nil
}
