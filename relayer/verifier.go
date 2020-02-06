package relayer

import (
	"path"
	"time"

	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint"
	abci "github.com/tendermint/tendermint/abci/types"
	lite "github.com/tendermint/tendermint/lite2"
	litehttp "github.com/tendermint/tendermint/lite2/provider/http"
	dbs "github.com/tendermint/tendermint/lite2/store/db"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	dbm "github.com/tendermint/tm-db"
)

// StartUpdatingLiteClient begins a loop that periodically updates the lite database
func (c *Chain) StartUpdatingLiteClient(period time.Duration) {
	ticker := time.NewTicker(period)
	for ; true; <-ticker.C {
		err := c.UpdateLiteDBToLatestHeader()
		if err != nil {
			c.logger.Error(err.Error())
		}
	}
}

// UpdateLiteDBToLatestHeader spins up an instance of the lite client as part of the chain.
func (c *Chain) UpdateLiteDBToLatestHeader() (err error) {
	// create database connection
	db, df, err := c.NewLiteDB()
	if err != nil {
		return
	}
	defer df()

	// initialise Lite Client
	lc, err := c.InitLiteClientWithoutTrust(db)
	if err != nil {
		return
	}

	// sync lite client to the most recent header of the primary provider
	now := time.Now()
	err = lc.Update(now)
	if err != nil {
		return
	}

	// remove expired headers
	lc.RemoveNoLongerTrustedHeaders(now)
	return
}

// InitLiteClientWithoutTrust reads the trusted period off of the chain
func (c *Chain) InitLiteClientWithoutTrust(db *dbm.GoLevelDB) (*lite.Client, error) {
	return c.InitLiteClient(db, c.EmptyTrustOptions())
}

// InitLiteClient initializes the lite client for a given chain
func (c *Chain) InitLiteClient(db *dbm.GoLevelDB, trustOption lite.TrustOptions) (*lite.Client, error) {
	httpProvider, err := litehttp.New(c.ChainID, c.RPCAddr)
	if err != nil {
		return nil, err
	}

	return lite.NewClient(c.ChainID,
		trustOption,
		httpProvider,
		dbs.New(db, c.ChainID))
}

// TrustNodeInitClient trusts the configured node and initializes the lite client
func (c *Chain) TrustNodeInitClient(db *dbm.GoLevelDB) (*lite.Client, error) {
	// fetch latest height from configured node
	height, err := c.QueryLatestHeight()
	if err != nil {
		return nil, err
	}

	// fetch header from configured node
	header, err := c.QueryHeaderAtHeight(height)
	if err != nil {
		return nil, err
	}

	// initialize the lite client database
	out, err := c.InitLiteClient(db, c.TrustOptions(height, header.Hash().Bytes()))
	if err != nil {
		return nil, err
	}

	return out, nil
}

// NewLiteDB returns a new instance of the liteclient database connection
// CONTRACT: must close the database connection when done with it (defer df())
func (c *Chain) NewLiteDB() (db *dbm.GoLevelDB, df func(), err error) {
	db, err = dbm.NewGoLevelDB(c.ChainID, path.Join(c.ChainDir, "db"))
	df = func() {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}
	return
}

// EmptyTrustOptions gets the lite.TrustOptions with c.TrustPeriod set
func (c *Chain) EmptyTrustOptions() lite.TrustOptions {
	return c.TrustOptions(-1, nil)
}

// TrustOptions returns lite.TrustOptions given a height and hash
func (c *Chain) TrustOptions(height int64, hash []byte) lite.TrustOptions {
	dur, err := time.ParseDuration(c.TrustingPeriod)
	if err != nil {
		panic(err)
	}
	return lite.TrustOptions{
		Period: dur,
		Height: height,
		Hash:   hash,
	}
}

// GetLatestLiteHeight uses the CLI utilities to pull the latest height from a given chain
func (c *Chain) GetLatestLiteHeight() (int64, error) {
	db, df, err := c.NewLiteDB()
	if err != nil {
		return -1, err
	}
	defer df()

	store := dbs.New(db, c.ChainID)

	return store.LastSignedHeaderHeight()

}

// GetLatestLiteHeader returns the header to be used for client creation
func (c *Chain) GetLatestLiteHeader() (*tmclient.Header, error) {
	height, err := c.GetLatestLiteHeight()
	if err != nil {
		return nil, err
	}
	return c.GetLiteSignedHeaderAtHeight(height)
}

// VerifyProof performs response proof verification.
func (c *Chain) VerifyProof(queryPath string, resp abci.ResponseQuery) error {
	// TODO: write this verify function
	return nil
}

// ValidateTxResult takes a transaction and validates the proof against a stored root of trust
func (c *Chain) ValidateTxResult(resTx *ctypes.ResultTx) (err error) {
	// fetch the header at the height from the ResultTx from the lite database
	check, err := c.GetLiteSignedHeaderAtHeight(resTx.Height)
	if err != nil {
		return
	}

	// validate the proof against that header
	err = resTx.Proof.Validate(check.Header.DataHash)
	if err != nil {
		return
	}

	return
}

// GetLiteSignedHeaderAtHeight returns a signed header at a particular height
func (c *Chain) GetLiteSignedHeaderAtHeight(height int64) (*tmclient.Header, error) {
	// create database connection
	db, df, err := c.NewLiteDB()
	if err != nil {
		return nil, err
	}
	defer df()

	// QUESTION: Why do we need this store abstration here and not in other lite functions?
	store := dbs.New(db, c.ChainID)

	// Fetch the validator set from the store
	vs, err := store.ValidatorSet(height)
	if err != nil {
		return nil, err
	}

	// Fetch the signed header from the store
	sh, err := store.SignedHeader(height)
	if err != nil {
		return nil, err
	}

	return &tmclient.Header{SignedHeader: *sh, ValidatorSet: vs}, nil
}
