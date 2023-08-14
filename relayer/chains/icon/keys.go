package icon

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"github.com/cosmos/relayer/v2/relayer/provider"
	glcrypto "github.com/icon-project/goloop/common/crypto"
	"github.com/icon-project/goloop/common/wallet"
	"github.com/icon-project/goloop/module"
)

func (cp *IconProvider) CreateKeystore(path string) error {
	_, e := cp.generateKeystoreWithPassword(path, "gochain")
	return e
}

func (cp *IconProvider) KeystoreCreated(path string) bool {
	return false
}

func (cp *IconProvider) AddKey(name string, coinType uint32, signingAlgorithm string, password string) (output *provider.KeyOutput, err error) {
	w, err := cp.generateKeystoreWithPassword(name, password)
	if err != nil {
		return nil, err
	}
	return &provider.KeyOutput{
		Address:  w.Address().String(),
		Mnemonic: "",
	}, nil
}

func (cp *IconProvider) RestoreKey(name, mnemonic string, coinType uint32, signingAlgorithm string) (address string, err error) {
	return "", fmt.Errorf("not implemented on icon")
}

func (cp *IconProvider) ShowAddress(name string) (address string, err error) {
	dirPath := path.Join(cp.PCfg.KeyDirectory, cp.ChainId(), fmt.Sprintf("%s.json", name))
	return getAddrFromKeystore(dirPath)
}

func (cp *IconProvider) ListAddresses() (map[string]string, error) {
	dirPath := path.Join(cp.PCfg.KeyDirectory, cp.ChainId())
	dirEntry, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	addrMap := make(map[string]string)
	for _, file := range dirEntry {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			ksFile := path.Join(dirPath, file.Name())
			addr, err := getAddrFromKeystore(ksFile)
			if err != nil {
				continue
			}
			addrMap[strings.TrimSuffix(file.Name(), ".json")] = addr
		}
	}
	return addrMap, nil
}

func (cp *IconProvider) DeleteKey(name string) error {
	ok := cp.KeyExists(name)
	if !ok {
		return fmt.Errorf("wallet does not exist")
	}

	dirPath := path.Join(cp.PCfg.KeyDirectory, cp.ChainId(), fmt.Sprintf("%s.json", name))
	_, err := os.Stat(dirPath)
	if err == nil {
		if err := os.Remove(dirPath); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("fail to delete wallet")
}

func (cp *IconProvider) KeyExists(name string) bool {
	walletPath := path.Join(cp.PCfg.KeyDirectory, cp.ChainId(), fmt.Sprintf("%s.json", name))
	_, err := os.ReadFile(walletPath)
	if err != nil && os.IsNotExist(err) {
		return false
	} else if err != nil {
		panic("key does not exist")
	}
	return true
}

func (cp *IconProvider) ExportPrivKeyArmor(keyName string) (armor string, err error) {
	return "", fmt.Errorf("not implemented on icon")
}

func (cp *IconProvider) RestoreIconKeyStore(name string, password []byte) (module.Wallet, error) {
	walletPath := path.Join(cp.PCfg.KeyDirectory, cp.ChainId(), fmt.Sprintf("%s.json", name))
	ksByte, err := os.ReadFile(walletPath)
	if err != nil {
		return nil, err
	}
	w, err := wallet.NewFromKeyStore(ksByte, password)
	if err != nil {
		return nil, err
	}
	return w, nil
}

// This method does not save keystore
func (cp *IconProvider) RestoreFromPrivateKey(name string, pk []byte) (module.Wallet, error) {
	pKey, err := glcrypto.ParsePrivateKey(pk)
	if err != nil {
		return nil, err
	}
	w, err := wallet.NewFromPrivateKey(pKey)
	if err != nil {
		return nil, err
	}
	return w, nil
}

func (cp *IconProvider) generateKeystoreWithPassword(name string, password string) (module.Wallet, error) {
	w := wallet.New()
	ks, err := wallet.KeyStoreFromWallet(w, []byte(password))
	if err != nil {
		log.Panicf("Failed to generate keystore. Err %+v", err)
		return nil, err
	}

	if err := cp.saveWallet(name, ks); err != nil {
		return nil, err
	}

	return w, nil
}

func (cp *IconProvider) saveWallet(name string, ks []byte) error {
	dirPath := path.Join(cp.PCfg.KeyDirectory, cp.ChainId())
	_, err := os.Stat(dirPath)
	if os.IsNotExist(err) {
		err := os.MkdirAll(dirPath, 0755)
		if err != nil {
			panic(err)
		}
	} else if err != nil {
		return err
	}
	if err := os.WriteFile(fmt.Sprintf("%s/%s.json", dirPath, name), ks, 0600); err != nil {
		log.Panicf("Fail to write keystore err=%+v", err)
		return err
	}
	return nil
}

type OnlyAddr struct {
	Address string `json:"address"`
}

func getAddrFromKeystore(keystorePath string) (string, error) {

	ksFile, err := os.ReadFile(keystorePath)
	if err != nil {
		return "", err
	}

	var a OnlyAddr
	err = json.Unmarshal(ksFile, &a)
	if err != nil {
		return "", err
	}
	return a.Address, nil

}
