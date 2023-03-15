package icon

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const (
	ENDPOINT = "https://ctz.solidwallet.io/api/v3/"
)

func TestCreateKeystore(t *testing.T) {
	kwName := "testWallet.json"
	p := &IconProvider{
		client: NewClient(ENDPOINT, &zap.Logger{}),
	}
	err := p.CreateKeystore(kwName)
	require.NoError(t, err)
}

func TestAddIconKeyStore(t *testing.T) {
	kwName := "testWallet.json"
	p := &IconProvider{
		client: NewClient(ENDPOINT, &zap.Logger{}),
	}
	w, err := p.AddIconKey(kwName, []byte("gochain"))
	require.NoError(t, err, "err creating keystore with password")

	assert.Equal(t, w.Address(), p.wallet.Address())
	assert.Equal(t, w, p.wallet)
}

func TestRestoreIconKeyStore(t *testing.T) {

	kwName := "/home/lilixac/keystore/godWallet.json"
	p := &IconProvider{
		client: NewClient(ENDPOINT, &zap.Logger{}),
	}

	w, err := generateKeystoreWithPassword(kwName, []byte("gochain"))
	require.NoError(t, err)

	assert.Equal(t, w, p.wallet)
}
