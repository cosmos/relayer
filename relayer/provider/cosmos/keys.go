package cosmos

import (
	"path"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/go-bip39"
	"github.com/cosmos/relayer/relayer/provider"
)

// KeysDir returns the filesystem directory where keychain data is stored
func KeysDir(home, chainID string) string {
	return path.Join(home, "keys", chainID)
}

// CreateMnemonic creates a new mnemonic
func CreateMnemonic() (string, error) {
	entropySeed, err := bip39.NewEntropy(256)
	if err != nil {
		return "", err
	}
	mnemonic, err := bip39.NewMnemonic(entropySeed)
	if err != nil {
		return "", err
	}
	return mnemonic, nil
}

// KeyAddOrRestore is a helper function for add key and restores key when mnemonic is passed
func (cp *CosmosProvider) KeyAddOrRestore(keyName string, coinType uint32, mnemonic ...string) (*provider.KeyOutput, error) {
	var mnemonicStr string
	var err error

	if len(mnemonic) > 0 {
		mnemonicStr = mnemonic[0]
	} else {
		mnemonicStr, err = CreateMnemonic()
		if err != nil {
			return &provider.KeyOutput{}, err
		}
	}

	info, err := cp.Keybase.NewAccount(keyName, mnemonicStr, "", hd.CreateHDPath(coinType, 0, 0).String(), hd.Secp256k1)
	if err != nil {
		return &provider.KeyOutput{}, err
	}

	done := cp.UseSDKContext()
	ko := &provider.KeyOutput{Mnemonic: mnemonicStr, Address: info.GetAddress().String()}
	done()

	return ko, nil
}
