package relayer

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint/types"
	dbm "github.com/tendermint/tm-db"

	retry "github.com/avast/retry-go"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	lite "github.com/tendermint/tendermint/lite2"
	litep "github.com/tendermint/tendermint/lite2/provider"
	litehttp "github.com/tendermint/tendermint/lite2/provider/http"
	dbs "github.com/tendermint/tendermint/lite2/store/db"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

type header struct {
	sync.Mutex
	Map  map[string]*tmclient.Header
	Errs []error
}

func (h *header) err() error {
	var out error
	for _, err := range h.Errs {
		out = fmt.Errorf("err: %w", err)
	}
	return out
}

// UpdatesWithHeaders calls UpdateLiteWithHeader on the passed chains concurrently
func UpdatesWithHeaders(chains ...*Chain) (map[string]*tmclient.Header, error) {
	hs := &header{Map: make(map[string]*tmclient.Header), Errs: []error{}}
	var wg sync.WaitGroup
	for _, chain := range chains {
		wg.Add(1)
		go func(hs *header, wg *sync.WaitGroup, chain *Chain) {
			defer wg.Done()
			header, err := chain.UpdateLiteWithHeader()
			hs.Lock()
			hs.Map[chain.ChainID] = header
			if err != nil {
				hs.Errs = append(hs.Errs, err)
			}
			hs.Unlock()
		}(hs, &wg, chain)
	}
	wg.Wait()
	return hs.Map, hs.err()
}

// UpdateLiteWithHeader calls client.Update and then .
func (c *Chain) UpdateLiteWithHeader() (*tmclient.Header, error) {
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

	sh, err := client.Update(time.Now())
	if err != nil {
		return nil, err
	}

	if sh == nil {
		sh, err = client.TrustedHeader(0)
		if err != nil {
			return nil, err
		}
	}

	vs, _, err := client.TrustedValidatorSet(sh.Height)
	if err != nil {
		return nil, err
	}

	return &tmclient.Header{SignedHeader: *sh, ValidatorSet: vs}, nil
}

// LiteClientWithoutTrust reads the trusted period off of the chain.
func (c *Chain) LiteClientWithoutTrust(db *dbm.GoLevelDB) (*lite.Client, error) {
	httpProvider, err := litehttp.New(c.ChainID, c.RPCAddr)
	if err != nil {
		return nil, err
	}

	// NOTE: currently we are discarding the very noisy lite client logs
	// it would be nice if we could add a setting the chain or otherwise
	// that allowed users to enable lite client logging. (maybe as a hidden prop
	// on the Chain struct that users could pass in the config??)
	logger := log.NewTMLogger(log.NewSyncWriter(ioutil.Discard))

	// TODO: provide actual witnesses!
	return lite.NewClientFromTrustedStore(c.ChainID, c.GetTrustingPeriod(), httpProvider,
		[]litep.Provider{httpProvider}, dbs.New(db, ""),
		lite.Logger(logger))
}

// LiteClient initializes the lite client for a given chain.
func (c *Chain) LiteClient(db *dbm.GoLevelDB, trustOpts lite.TrustOptions) (*lite.Client, error) {
	httpProvider, err := litehttp.New(c.ChainID, c.RPCAddr)
	if err != nil {
		return nil, err
	}

	// NOTE: currently we are discarding the very noisy lite client logs
	// it would be nice if we could add a setting the chain or otherwise
	// that allowed users to enable lite client logging. (maybe as a hidden prop
	// on the Chain struct that users could pass in the config??)
	logger := log.NewTMLogger(log.NewSyncWriter(ioutil.Discard))

	// TODO: provide actual witnesses!
	return lite.NewClient(c.ChainID, trustOpts, httpProvider,
		[]litep.Provider{httpProvider}, dbs.New(db, ""),
		lite.Logger(logger))
}

// InitLiteClient instantantiates the lite client object and calls update
func (c *Chain) InitLiteClient(db *dbm.GoLevelDB, trustOpts lite.TrustOptions) (*lite.Client, error) {
	lc, err := c.LiteClient(db, trustOpts)
	if err != nil {
		return nil, err
	}
	_, err = lc.Update(time.Now())
	if err != nil {
		return nil, err
	}
	return lc, err
}

// TrustNodeInitClient trusts the configured node and initializes the lite client
func (c *Chain) TrustNodeInitClient(db *dbm.GoLevelDB) (*lite.Client, error) {
	// fetch latest height from configured node
	var (
		height int64
		err    error
	)

	if err := retry.Do(func() error {
		height, err = c.QueryLatestHeight()
		if err != nil || height == 0 {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// fetch header from configured node
	header, err := c.QueryHeaderAtHeight(height)
	if err != nil {
		return nil, err
	}

	lc, err := c.LiteClient(db, c.TrustOptions(height, header.Hash().Bytes()))
	if err != nil {
		return nil, err
	}

	_, err = lc.Update(time.Now())
	if err != nil {
		return nil, err
	}

	return lc, nil
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

	client, err := c.LiteClientWithoutTrust(db)
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

	client, err := c.LiteClientWithoutTrust(db)
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

	return &tmclient.Header{SignedHeader: *sh, ValidatorSet: vs}, nil
}

// ErrLiteNotInitialized returns the cannonical error for a an uninitialized lite client
var ErrLiteNotInitialized = errors.New("lite client is not initialized")

// ForceInitLite forces initialization of the lite client from the configured node
func (c *Chain) ForceInitLite() error {
	db, df, err := c.NewLiteDB()
	if err != nil {
		return err
	}
	_, err = c.TrustNodeInitClient(db)
	if err != nil {
		return err
	}
	df()
	return nil
}
