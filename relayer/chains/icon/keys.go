package icon

import (
	"fmt"
	"log"
	"os"

	"github.com/cosmos/relayer/v2/relayer/provider"
	glcrypto "github.com/icon-project/goloop/common/crypto"
	"github.com/icon-project/goloop/common/wallet"
	"github.com/icon-project/goloop/module"
)

func (cp *IconProvider) CreateKeystore(path string) error {
	_, e := generateKeystoreWithPassword(path, []byte("gochain"))
	return e
}

func (cp *IconProvider) KeystoreCreated(path string) bool {
	return false
}

func (cp *IconProvider) AddKey(name string, coinType uint32) (output *provider.KeyOutput, err error) {
	return nil, fmt.Errorf("Not implemented on icon")
}

func (cp *IconProvider) RestoreKey(name, mnemonic string, coinType uint32) (address string, err error) {
	return "", fmt.Errorf("Not implemented on icon")
}

func (cp *IconProvider) ShowAddress(name string) (address string, err error) {
	return cp.wallet.Address().String(), nil
}

func (cp *IconProvider) ListAddresses() (map[string]string, error) {
	return nil, fmt.Errorf("Not implemented on icon")
}

func (cp *IconProvider) DeleteKey(name string) error {
	ok := cp.KeyExists(name)
	if !ok {
		return fmt.Errorf("Wallet does not exist")
	}
	cp.wallet = nil
	return nil
}

func (cp *IconProvider) KeyExists(name string) bool {
	return cp.wallet != nil
}

func (cp *IconProvider) ExportPrivKeyArmor(keyName string) (armor string, err error) {
	return "", fmt.Errorf("Not implemented on icon")
}

func (cp *IconProvider) AddIconKey(name string, password []byte) (module.Wallet, error) {
	w, err := generateKeystoreWithPassword(name, password)
	if err != nil {
		return nil, err
	}
	cp.AddWallet(w)
	return w, nil
}

func (cp *IconProvider) RestoreIconKeyStore(path string, password []byte) (module.Wallet, error) {
	ksByte, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	w, err := wallet.NewFromKeyStore(ksByte, password)
	if err != nil {
		return nil, err
	}
	cp.AddWallet(w)
	return w, nil
}

func (cp *IconProvider) RestoreIconPrivateKey(pk []byte) (module.Wallet, error) {
	pKey, err := glcrypto.ParsePrivateKey(pk)
	if err != nil {
		return nil, err
	}
	w, err := wallet.NewFromPrivateKey(pKey)
	if err != nil {
		return nil, err
	}
	cp.AddWallet(w)
	return w, nil
}

func (cp *IconProvider) KeyExistsIcon() bool {
	return cp.wallet != nil
}

func (cp *IconProvider) DeleteKeyIcon() error {
	ok := cp.KeyExistsIcon()
	if !ok {
		return fmt.Errorf("Wallet does not exist")
	}
	cp.wallet = nil
	return nil
}

func (cp *IconProvider) ShowAddressIcon() (address string, err error) {
	ok := cp.KeyExistsIcon()
	if !ok {
		return "", fmt.Errorf("Wallet does not exist")
	}
	return cp.wallet.Address().String(), nil
}

func generateKeystoreWithPassword(path string, password []byte) (module.Wallet, error) {
	w := wallet.New()
	_, err := wallet.KeyStoreFromWallet(w, password)
	if err != nil {
		log.Panicf("Failed to generate keystore. Err %+v", err)
		return nil, err
	}
	return w, nil
}
