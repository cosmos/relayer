package substrate

import (
	"os"

	"github.com/cosmos/go-bip39"
	"github.com/cosmos/relayer/v2/relayer/chains/substrate/keystore"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

func (sp *SubstrateProvider) CreateKeystore(path string) error {
	keybase, err := keystore.New(sp.PCfg.ChainID, sp.PCfg.KeyringBackend, sp.PCfg.KeyDirectory, sp.Input)
	if err != nil {
		return err
	}
	sp.Keybase = keybase
	return nil
}

func (sp *SubstrateProvider) KeystoreCreated(path string) bool {
	if _, err := os.Stat(path); err != nil || sp.Keybase == nil {
		return false
	}
	return true
}

func (sp *SubstrateProvider) AddKey(name string, coinType uint32) (output *provider.KeyOutput, err error) {
	ko, err := sp.KeyAddOrRestore(name, coinType)
	if err != nil {
		return nil, err
	}

	return ko, nil
}

func (sp *SubstrateProvider) RestoreKey(name, mnemonic string, coinType uint32) (address string, err error) {
	ko, err := sp.KeyAddOrRestore(name, coinType, mnemonic)
	if err != nil {
		return "", err
	}
	return ko.Address, nil
}

func (sp *SubstrateProvider) ShowAddress(name string) (address string, err error) {

	info, err := sp.Keybase.Key(name)
	if err != nil {
		return "", err
	}
	return info.GetAddress(), nil
}

func (sp *SubstrateProvider) ListAddresses() (map[string]string, error) {

	out := map[string]string{}
	info, err := sp.Keybase.List()
	if err != nil {
		return nil, err
	}
	for _, k := range info {
		addr := k.GetAddress()
		out[k.GetName()] = addr
	}
	return out, nil
}

func (sp *SubstrateProvider) DeleteKey(name string) error {

	if err := sp.Keybase.Delete(name); err != nil {
		return err
	}

	return nil
}

func (sp *SubstrateProvider) KeyExists(name string) bool {
	if sp.Keybase == nil {
		return false
	}

	k, err := sp.Keybase.Key(name)
	if err != nil {
		return false
	}
	return k.GetName() == name
}

func (sp *SubstrateProvider) ExportPrivKeyArmor(keyName string) (armor string, err error) {
	return "", nil
}

func (sp *SubstrateProvider) KeyAddOrRestore(keyName string, coinType uint32, mnemonic ...string) (*provider.KeyOutput, error) {

	var mnemonicStr string
	var err error

	if len(mnemonic) > 0 {
		mnemonicStr = mnemonic[0]
	} else {
		mnemonicStr, err = CreateMnemonic()
		if err != nil {
			return nil, err
		}
	}

	info, err := sp.Keybase.NewAccount(keyName, mnemonicStr, sp.PCfg.Network)
	if err != nil {
		return nil, err
	}

	return &provider.KeyOutput{Mnemonic: mnemonicStr, Address: info.GetAddress()}, nil
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
