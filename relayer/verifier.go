package relayer

import (
	"fmt"
	lite "github.com/tendermint/tendermint/lite2"
	litehttp "github.com/tendermint/tendermint/lite2/provider/http"
	dbs "github.com/tendermint/tendermint/lite2/store/db"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
	"path"
	"time"
)

func (c *Chain) initLiteClientWithoutTrust(db *dbm.GoLevelDB) (*lite.Client, error) {
	return c.InitLiteClient(db, c.GetEmptyTrustOptions())
}

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

// Spins up an instance of the lite client as part of the chain.
func (c *Chain) Update() error {
	// Open connection to the database temporarily
	db, err := dbm.NewGoLevelDB(fmt.Sprintf("lite-%s", c.ChainID), path.Join(c.ChainDir, "db"))
	if err != nil {
		return err
	}
	defer func() {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}()

	// initialise Lite Client
	lc, err := c.initLiteClientWithoutTrust(db)
	if err != nil {
		return err
	}

	now := time.Now()
	// sync lite client to the most recent header of the primary provider
	err = lc.Update(now)
	if err != nil {
		return err
	}

	// also remove expired headers
	lc.RemoveNoLongerTrustedHeaders(now)

	return nil
}

// LatestHeight uses the CLI utilities to pull the latest height from a given chain
func (c *Chain) LatestHeight() (int64, error) {
	db, err := dbm.NewGoLevelDB(fmt.Sprintf("lite-db-%s", c.ChainID), path.Join(c.ChainDir, "db"))
	if err != nil {
		return -1, err
	}
	defer func() {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}()

	store := dbs.New(db, c.ChainID)

	return store.LastSignedHeaderHeight()

}

// LatestHeader returns the header to be used for client creation
func (c *Chain) LatestHeader() (*types.SignedHeader, error) {
	height, err := c.LatestHeight()
	if err != nil {
		return nil, err
	}
	return c.SignedHeaderAtHeight(height)
}

func (c *Chain) SignedHeaderAtHeight(height int64) (*types.SignedHeader, error) {
	db, err := dbm.NewGoLevelDB(fmt.Sprintf("lite-%s", c.ChainID), path.Join(c.ChainDir, "db"))
	if err != nil {
		return nil, err
	}
	defer func() {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}()

	store := dbs.New(db, c.ChainID)

	return store.SignedHeader(height)
}
