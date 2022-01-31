package substrate

import "github.com/cosmos/relayer/relayer/provider"

func (sp *SubstrateProvider) CreateKeystore(path string) error {
	return nil
}

func (sp *SubstrateProvider) KeystoreCreated(path string) bool {
	return false
}

func (sp *SubstrateProvider) AddKey(name string) (output *provider.KeyOutput, err error) {
	return nil, nil
}

func (sp *SubstrateProvider) RestoreKey(name, mnemonic string) (address string, err error) {
	return "", nil
}

func (sp *SubstrateProvider) ShowAddress(name string) (address string, err error) {
	return "", nil
}

func (sp *SubstrateProvider) ListAddresses() (map[string]string, error) {
	return nil, nil
}

func (sp *SubstrateProvider) DeleteKey(name string) error {
	return nil
}

func (sp *SubstrateProvider) KeyExists(name string) bool {
	return false
}

func (sp *SubstrateProvider) ExportPrivKeyArmor(keyName string) (armor string, err error) {
	return "", nil
}
