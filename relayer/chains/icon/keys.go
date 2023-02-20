package icon

import (
	"github.com/cosmos/relayer/v2/relayer/provider"
)

func (cp *IconProvider) CreateKeystore(path string) error {
	return nil
}

func (cp *IconProvider) KeystoreCreated(path string) bool {
	return false
}

func (cp *IconProvider) AddKey(name string, coinType uint32) (output *provider.KeyOutput, err error) {
	return nil, nil
}

func (cp *IconProvider) RestoreKey(name, mnemonic string, coinType uint32) (address string, err error) {
	return "", nil
}

func (cp *IconProvider) ShowAddress(name string) (address string, err error) {
	return "", nil
}

func (cp *IconProvider) ListAddresses() (map[string]string, error) {
	return nil, nil
}

func (cp *IconProvider) DeleteKey(name string) error {
	return nil
}

func (cp *IconProvider) KeyExists(name string) bool {
	return false
}

func (cp *IconProvider) ExportPrivKeyArmor(keyName string) (armor string, err error) {
	return "", nil
}
