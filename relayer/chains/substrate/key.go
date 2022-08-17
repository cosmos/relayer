package substrate

import (
	"github.com/cosmos/relayer/v2/relayer/provider"
)

func (sp *SubstrateProvider) CreateKeystore(path string) error {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) KeystoreCreated(path string) bool {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) AddKey(name string, coinType uint32) (output *provider.KeyOutput, err error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) RestoreKey(name, mnemonic string, coinType uint32) (address string, err error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) ShowAddress(name string) (address string, err error) {
	// TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) ListAddresses() (map[string]string, error) {
	// TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) DeleteKey(name string) error {
	// TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) KeyExists(name string) bool {
	// TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) ExportPrivKeyArmor(keyName string) (armor string, err error) {
	//TODO implement me
	panic("implement me")
}
