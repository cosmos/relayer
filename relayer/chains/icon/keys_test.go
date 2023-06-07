package icon

import (
	"fmt"
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

	kwName := "../../../env/godWallet.json"

	pcfg := &IconProviderConfig{
		Keystore:  kwName,
		Password:  "gochain",
		Timeout:   "20s",
		ChainName: "icon",
		BTPHeight: 10,
	}
	p, err := pcfg.NewProvider(zap.NewNop(), "not_correct", false, "icon")
	require.NoError(t, err)
	iconp := p.(*IconProvider)
	fmt.Println(iconp)
	w, err := iconp.RestoreIconKeyStore(kwName, []byte("gochain"))
	require.NoError(t, err)

	assert.Equal(t, w, iconp.wallet)
}
