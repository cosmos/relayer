package icon

import (
	"github.com/cosmos/relayer/v2/relayer/provider"
)

func (cp *IconProvider) CreateKeystore(path string) error

func (cp *IconProvider) KeystoreCreated(path string) bool

func (cp *IconProvider) AddKey(name string, coinType uint32) (output *provider.KeyOutput, err error)

func (cp *IconProvider) RestoreKey(name, mnemonic string, coinType uint32) (address string, err error)

func (cp *IconProvider) ShowAddress(name string) (address string, err error)

func (cp *IconProvider) ListAddresses() (map[string]string, error)

func (cp *IconProvider) DeleteKey(name string) error

func (cp *IconProvider) KeyExists(name string) bool

func (cp *IconProvider) ExportPrivKeyArmor(keyName string) (armor string, err error)
