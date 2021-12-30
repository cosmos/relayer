package cosmos

import (
	"errors"
	ckeys "github.com/cosmos/cosmos-sdk/client/keys"
	keys "github.com/cosmos/cosmos-sdk/crypto/keyring"
	"os"
	"path"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/go-bip39"
	"github.com/cosmos/relayer/relayer/provider"
)

// CreateKeystore creates the on disk file for the keystore and attaches it to the provider object
func (cp *CosmosProvider) CreateKeystore(homePath string) error {
	kb, err := keys.New(cp.Config.ChainID, "test", KeysDir(homePath, cp.Config.ChainID), nil)
	if err != nil {
		return err
	}
	cp.Keybase = kb
	return nil
}

// KeystoreCreated returns false if either files aren't on disk as expected or the keystore isn't set on the provider
func (cp *CosmosProvider) KeystoreCreated(homePath string) bool {
	if _, err := os.Stat(KeysDir(homePath, cp.Config.ChainID)); errors.Is(err, os.ErrNotExist) {
		return false
	} else if cp.Keybase == nil {
		return false
	}
	return true
}

// AddKey adds a key to the keystore and generate and return a mnemonic for it
func (cp *CosmosProvider) AddKey(name string) (*provider.KeyOutput, error) {
	ko, err := cp.KeyAddOrRestore(name, 118)
	if err != nil {
		return nil, err
	}
	return ko, nil
}

// RestoreKey restores a key from a mnemonic to the keystore at ta given name
func (cp *CosmosProvider) RestoreKey(name, mnemonic string) (string, error) {
	ko, err := cp.KeyAddOrRestore(name, 118, mnemonic)
	if err != nil {
		return "", err
	}
	return ko.Address, nil
}

// KeyExists returns true if there is a specified key in provider's keybase
func (cp *CosmosProvider) KeyExists(name string) bool {
	k, err := cp.Keybase.Key(name)
	if err != nil {
		return false
	}

	return k.GetName() == name
}

// ShowAddress shows the address for a key from the store
func (cp *CosmosProvider) ShowAddress(name string) (address string, err error) {
	info, err := cp.Keybase.Key(name)
	if err != nil {
		return "", err
	}
	done := cp.UseSDKContext()
	address = info.GetAddress().String()
	done()
	return address, nil
}

// ListAddresses lists the addresses in the keystore and their assoicated names
func (cp *CosmosProvider) ListAddresses() (map[string]string, error) {
	out := map[string]string{}
	info, err := cp.Keybase.List()
	if err != nil {
		return nil, err
	}
	done := cp.UseSDKContext()
	for _, k := range info {
		out[k.GetName()] = k.GetAddress().String()
	}
	done()
	return out, nil
}

// DeleteKey deletes a key tracked by the store
func (cp *CosmosProvider) DeleteKey(name string) error {
	if err := cp.Keybase.Delete(name); err != nil {
		return err
	}
	return nil
}

// ExportPrivKeyArmor exports a privkey from the keychain associated with a particular keyName
func (cp *CosmosProvider) ExportPrivKeyArmor(keyName string) (armor string, err error) {
	return cp.Keybase.ExportPrivKeyArmor(keyName, ckeys.DefaultKeyPass)
}

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
