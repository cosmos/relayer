package relayer

import (
	"fmt"

	lite "github.com/tendermint/tendermint/lite2"
	litehttp "github.com/tendermint/tendermint/lite2/provider/http"
	dbs "github.com/tendermint/tendermint/lite2/store/db"
	dbm "github.com/tendermint/tm-db"
)

// NewLiteClient returns a new instance of the lite client.
func (c *Chain) NewLiteClient(dbDir string) (*lite.Client, error) {
	httpProvider, err := litehttp.New(c.ChainID, c.RPCAddr)
	if err != nil {
		return nil, err
	}

	db, err := dbm.NewGoLevelDB(fmt.Sprintf("lite-client-%s", c.ChainID), dbDir)
	if err != nil {
		return nil, err
	}

	return lite.NewClient(c.ChainID,
		c.TrustOptions.Get(),
		httpProvider,
		dbs.New(db, c.ChainID),
		lite.UpdatePeriod(c.TrustOptions.Get().Period))
}
