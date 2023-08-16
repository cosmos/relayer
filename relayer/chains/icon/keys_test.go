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
		PCfg: &IconProviderConfig{
			KeyDirectory: "../../../env",
			ChainID:      "ibc-icon",
		},
	}
	err := p.CreateKeystore(kwName)
	require.NoError(t, err)
}

func TestAddIconKeyStore(t *testing.T) {
	kwName := "testWallet"
	p := &IconProvider{
		client: NewClient(ENDPOINT, &zap.Logger{}),
		PCfg: &IconProviderConfig{
			KeyDirectory: "../../../env",
			ChainID:      "ibc-icon",
		},
	}
	w, err := p.AddKey(kwName, 0, "", "gochain")
	require.NoError(t, err, "err creating keystore with password")

	addr, err := p.ShowAddress(kwName)
	assert.NoError(t, err)

	// assert.Equal(t, w.Address, p.wallet.Address())
	assert.Equal(t, w.Address, addr)
}

// func TestRestoreIconKeyStore(t *testing.T) {

// 	pcfg := &IconProviderConfig{
// 		KeyDirectory:      "../../../env",
// 		Keystore:          "testWallet",
// 		Password:          "gochain",
// 		Timeout:           "20s",
// 		ChainName:         "icon",
// 		StartHeight:       10,
// 		IbcHandlerAddress: "cxb6b5791be0b5ef67063b3c10b840fb81514db2fd",
// 		BlockInterval:     2000,
// 	}
// 	p, err := pcfg.NewProvider(zap.NewNop(), "not_correct", false, "icon")
// 	require.NoError(t, err)
// 	iconp := p.(*IconProvider)
// 	_, err = iconp.RestoreIconKeyStore("testWallet", []byte("gochain"))
// 	require.NoError(t, err)

// }
