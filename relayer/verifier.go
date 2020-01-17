package relayer

import (
	lite "github.com/tendermint/tendermint/lite2"
	dbs "github.com/tendermint/tendermint/lite2/store/db"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/lite2/provider/http"
)

// NewAutoLiteClient returns a new instance of the auto lite client
func (c *Chain) NewAutoLiteClient(homePath, homeDir string, cache int) (*lite.AutoClient, error) {
	cl, err := c.NewLiteClient(homePath, homeDir, cache)
	if err != nil {
		return &lite.AutoClient{}, err
	}

	return lite.NewAutoClient(cl, c.TrustOptions.Get().Period), nil
}

// NewLiteClient returns a new instance of the lite client
func (c *Chain) NewLiteClient(path, homeDir string, cache int) (*lite.Client, error) {
	var out = &lite.Client{}

	// Create lite.HTTP provider
	p, err := http.New(c.ChainID, c.RPCAddr)
	if err != nil {
		return out, err
	}

	// Create DB backend
	db, err := dbm.NewGoLevelDB(path, homeDir)
	if err != nil {
		return out, err
	}

	// If there is no hash input, grab a hash to intialize the client
	if len(c.TrustOptions.Hash) == 0 && c.TrustOptions.Height == 0 {
		var h = c.TrustOptions.Height + 1
		bl, err := c.Client.Block(&h)
		if err != nil {
			return out, err
		}
		c.TrustOptions.Height = 1
		c.TrustOptions.Hash = bl.Block.Header.Hash().Bytes()
	}

	// Initialize the lite.Client
	cl, err := lite.NewClient(c.ChainID, c.TrustOptions.Get(), p, dbs.New(db, c.ChainID))
	if err != nil {
		return out, err
	}

	return cl, nil
}
