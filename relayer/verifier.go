package relayer

import (
	"fmt"
	lite "github.com/tendermint/tendermint/lite2"
	litehttp "github.com/tendermint/tendermint/lite2/provider/http"
	dbs "github.com/tendermint/tendermint/lite2/store/db"
	dbm "github.com/tendermint/tm-db"
)

// Spins up an instance of the lite client as part of the chain.
func (c *Chain) StartLiteClient(dbDir string) error {
	if c.LiteClient != nil {
		return fmt.Errorf("instance of lite client already running for this chain")
	}

	httpProvider, err := litehttp.New(c.ChainID, c.RPCAddr)
	if err != nil {
		return err
	}

	db, err := dbm.NewGoLevelDB(fmt.Sprintf("lite-client-%s", c.ChainID), dbDir)
	if err != nil {
		return err
	}

	c.LiteClient, err = lite.NewClient(c.ChainID,
		c.TrustOptions.Get(),
		httpProvider,
		dbs.New(db, c.ChainID),
		lite.UpdatePeriod(c.UpdatePeriod))
	return err
}

func (c *Chain) StopLiteClient() error {
	if c.LiteClient != nil {
		c.LiteClient.Stop()
		err := c.LiteClient.Cleanup()
		if err != nil {
			return err
		}
		c.LiteClient = nil
	}
	return nil
}
