package relayer

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint/types"
	dbm "github.com/tendermint/tm-db"

	retry "github.com/avast/retry-go"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	lite "github.com/tendermint/tendermint/light"
	litep "github.com/tendermint/tendermint/light/provider"
	litehttp "github.com/tendermint/tendermint/light/provider/http"
	dbs "github.com/tendermint/tendermint/light/store/db"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
	"golang.org/x/sync/errgroup"
)

// NOTE: currently we are discarding the very noisy lite client logs
// it would be nice if we could add a setting the chain or otherwise
// that allowed users to enable lite client logging. (maybe as a hidden prop
// on the Chain struct that users could pass in the config??)
var logger = lite.Logger(log.NewTMLogger(log.NewSyncWriter(ioutil.Discard)))

// InjectTrustedFieldsHeaders takes the headers and enriches them
func InjectTrustedFieldsHeaders(src, dst *Chain, srch, dsth *tmclient.Header) (srcho *tmclient.Header, dstho *tmclient.Header, err error) {
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

// UpdatesWithHeaders calls UpdateLiteWithHeader on the passed chains concurrently
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

func liteError(err error) error { return fmt.Errorf("lite client: %w", err) }

// UpdateLiteWithHeader calls client.Update and then .
func (c *Chain) UpdateLiteWithHeader() (*tmclient.Header, error) {
	// create database connection
	db, df, err := c.NewLiteDB()
	if err != nil {
		return nil, liteError(err)
	}
	defer df()

	client, err := c.LiteClient(db)
	if err != nil {
		return nil, liteError(err)
	}

	sh, err := client.Update(time.Now())
	if err != nil {
		return nil, liteError(err)
	}

	if sh == nil {
		sh, err = client.TrustedHeader(0)
		if err != nil {
			return nil, liteError(err)
		}
	}

	vs, _, err := client.TrustedValidatorSet(sh.Height)
	if err != nil {
		return nil, liteError(err)
	}

	protoVal, err := tmtypes.NewValidatorSet(vs.Validators).ToProto()
	if err != nil {
		return nil, err
	}

	return &tmclient.Header{
		SignedHeader: sh.ToProto(),
		ValidatorSet: protoVal,
	}, nil
}

// UpdateLiteWithHeaderHeight updates the lite client database to the given height
func (c *Chain) UpdateLiteWithHeaderHeight(height int64) (*tmclient.Header, error) {
	// create database connection
	db, df, err := c.NewLiteDB()
	if err != nil {
		return nil, err
	}
	defer df()

	client, err := c.LiteClientWithoutTrust(db)
	if err != nil {
		return nil, err
	}

	sh, err := client.VerifyHeaderAtHeight(height, time.Now())
	if err != nil {
		return nil, err
	}

	vs, _, err := client.TrustedValidatorSet(sh.Height)
	if err != nil {
		return nil, err
	}

	protoVal, err := tmtypes.NewValidatorSet(vs.Validators).ToProto()
	if err != nil {
		return nil, err
	}

	return &tmclient.Header{SignedHeader: sh.ToProto(), ValidatorSet: protoVal}, nil
}

// LiteHTTP returns the http client for lite clients
func (c *Chain) LiteHTTP() litep.Provider {
	cl, err := litehttp.New(c.ChainID, c.RPCAddr)
	if err != nil {
		panic(err)
	}
	return cl
}

// LiteClientWithoutTrust querys the latest header from the chain and initializes a new lite client
// database using that header. This should only be called when first initializing the lite client
func (c *Chain) LiteClientWithoutTrust(db dbm.DB) (*lite.Client, error) {
	var (
		height int64
		err    error
	)
	prov := c.LiteHTTP()

	if err := retry.Do(func() error {
		height, err = c.QueryLatestHeight()
		if err != nil || height == 0 {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	header, err := prov.SignedHeader(height)
	if err != nil {
		return nil, err
	}
	return lite.NewClient(
		c.ChainID,
		lite.TrustOptions{
			Period: c.GetTrustingPeriod(),
			Height: height,
			Hash:   header.Hash(),
		},
		prov,
		// TODO: provide actual witnesses!
		// NOTE: This requires adding them to the chain config
		[]litep.Provider{prov},
		dbs.New(db, ""),
		logger)
}

// LiteClientWithTrust takes a header from the chain and attempts to add that header to the lite
// database.
func (c *Chain) LiteClientWithTrust(db dbm.DB, to lite.TrustOptions) (*lite.Client, error) {
	prov := c.LiteHTTP()
	return lite.NewClient(
		c.ChainID,
		to,
		prov,
		// TODO: provide actual witnesses!
		// NOTE: This requires adding them to the chain config
		[]litep.Provider{prov},
		dbs.New(db, ""),
		logger)
}

// LiteClient initializes the lite client for a given chain from the trusted store in the database
// this should be call for all other lite client usage
func (c *Chain) LiteClient(db dbm.DB) (*lite.Client, error) {
	prov := c.LiteHTTP()
	return lite.NewClientFromTrustedStore(
		c.ChainID,
		c.GetTrustingPeriod(),
		prov,
		// TODO: provide actual witnesses!
		// NOTE: This requires adding them to the chain config
		[]litep.Provider{prov},
		dbs.New(db, ""),
		logger,
	)
}

// NewLiteDB returns a new instance of the liteclient database connection
// CONTRACT: must close the database connection when done with it (defer df())
func (c *Chain) NewLiteDB() (db *dbm.GoLevelDB, df func(), err error) {
	if err := retry.Do(func() error {
		db, err = dbm.NewGoLevelDB(c.ChainID, liteDir(c.HomePath))
		if err != nil {
			return fmt.Errorf("can't open lite client database: %w", err)
		}
		return nil
	}); err != nil {
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

// DeleteLiteDB removes the lite client database on disk, forcing re-initialization
func (c *Chain) DeleteLiteDB() error {
	return os.RemoveAll(filepath.Join(liteDir(c.HomePath), fmt.Sprintf("%s.db", c.ChainID)))
}

// TrustOptions returns lite.TrustOptions given a height and hash
func (c *Chain) TrustOptions(height int64, hash []byte) lite.TrustOptions {
	return lite.TrustOptions{
		Period: c.GetTrustingPeriod(),
		Height: height,
		Hash:   hash,
	}
}

// GetLatestLiteHeader returns the header to be used for client creation
func (c *Chain) GetLatestLiteHeader() (*tmclient.Header, error) {
	return c.GetLiteSignedHeaderAtHeight(0)
}

// VerifyProof performs response proof verification.
func (c *Chain) VerifyProof(queryPath string, resp abci.ResponseQuery) error {
	// TODO: write this verify function
	return nil
}

// ValidateTxResult takes a transaction and validates the proof against a stored root of trust
func (c *Chain) ValidateTxResult(resTx *ctypes.ResultTx) (err error) {
	// fetch the header at the height from the ResultTx from the lite database
	check, err := c.GetLiteSignedHeaderAtHeight(resTx.Height - 1)
	if err != nil {
		return
	}

	// validate the proof against that header
	return resTx.Proof.Validate(check.Header.DataHash)
}

// GetLatestLiteHeight uses the CLI utilities to pull the latest height from a given chain
func (c *Chain) GetLatestLiteHeight() (int64, error) {
	db, df, err := c.NewLiteDB()
	if err != nil {
		return -1, err
	}
	defer df()

	client, err := c.LiteClient(db)
	if err != nil {
		return -1, err
	}

	return client.LastTrustedHeight()
}

// GetLiteSignedHeaderAtHeight returns a signed header at a particular height.
func (c *Chain) GetLiteSignedHeaderAtHeight(height int64) (*tmclient.Header, error) {
	// create database connection
	db, df, err := c.NewLiteDB()
	if err != nil {
		return nil, err
	}
	defer df()

	client, err := c.LiteClient(db)
	if err != nil {
		return nil, err
	}

	sh, err := client.TrustedHeader(height)
	if err != nil {
		return nil, err
	}

	vs, _, err := client.TrustedValidatorSet(sh.Height)
	if err != nil {
		return nil, err
	}

	protoVal, err := tmtypes.NewValidatorSet(vs.Validators).ToProto()
	if err != nil {
		return nil, err
	}

	return &tmclient.Header{SignedHeader: sh.ToProto(), ValidatorSet: protoVal}, nil
}

// ErrLiteNotInitialized returns the canonical error for a an uninitialized lite client
var ErrLiteNotInitialized = errors.New("lite client is not initialized")

// ForceInitLite forces initialization of the lite client from the configured node
func (c *Chain) ForceInitLite() error {
	db, df, err := c.NewLiteDB()
	if err != nil {
		return err
	}
	_, err = c.LiteClientWithoutTrust(db)
	if err != nil {
		return err
	}
	df()
	return nil
}
