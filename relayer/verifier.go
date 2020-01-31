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

func (c *Chain) initLiteClient(db *dbm.GoLevelDB) (*lite.Client, error) {
	httpProvider, err := litehttp.New(c.ChainID, c.RPCAddr)
	if err != nil {
		return nil, err
	}

	return lite.NewClient(c.ChainID,
		c.TrustOptions.Get(),
		httpProvider,
		dbs.New(db, c.ChainID))
}

// Spins up an instance of the lite client as part of the chain.
func (c *Chain) Update() error {
	db, err := dbm.NewGoLevelDB(fmt.Sprintf("lite-%s", c.ChainID), path.Join(c.dir, "db"))
	if err != nil {
		return err
	}
	defer db.Close()

	lc, err := c.initLiteClient(db)
	if err != nil {
		return err
	}

	err = lc.Update(time.Now())
	if err != nil {
		return err
	}

	return nil
}

// LatestHeight uses the CLI utilities to pull the latest height from a given chain
func (c *Chain) LatestHeight() (int64, error) {
	db, err := dbm.NewGoLevelDB(fmt.Sprintf("lite-db-%s", c.ChainID), path.Join(c.dir, "db"))
	if err != nil {
		return -1, err
	}
	defer db.Close()

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
	db, err := dbm.NewGoLevelDB(fmt.Sprintf("lite-%s", c.ChainID), path.Join(c.dir, "db"))
	if err != nil {
		return nil, err
	}
	defer db.Close()

	store := dbs.New(db, c.ChainID)

	return store.SignedHeader(height)
}
