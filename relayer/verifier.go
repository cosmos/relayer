package relayer

import (
	"time"

	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint"
	lite "github.com/tendermint/tendermint/lite2"
	litehttp "github.com/tendermint/tendermint/lite2/provider/http"
	dbs "github.com/tendermint/tendermint/lite2/store/db"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
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

// Update spins up an instance of the lite client as part of the chain.
func (c *Chain) Update() error {
	db, err := c.NewLiteDB()
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
	db, err := c.NewLiteDB()
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
func (c *Chain) LatestHeader() (*tmclient.Header, error) {
	height, err := c.LatestHeight()
	if err != nil {
		return nil, err
	}
	return c.SignedHeaderAtHeight(height)
}

// SignedHeaderAtHeight returns a signed header at a particular height
func (c *Chain) SignedHeaderAtHeight(height int64) (*tmclient.Header, error) {
	db, err := c.NewLiteDB()
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

	sh, err := store.SignedHeader(height)
	if err != nil {
		return nil, err
	}

	return headerFromSignedHeader(sh), nil
}

func headerFromSignedHeader(sh *types.SignedHeader) *tmclient.Header {
	return &tmclient.Header{SignedHeader: *sh}
}
