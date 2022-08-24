package keystore

import (
	"encoding/json"
	"fmt"

	"github.com/ComposableFi/go-substrate-rpc-client/v4/signature"
)

// Info is the publicly exposed information about a keypair
type Info interface {
	// GetName returns the name of the underlying key
	GetName() string
	// GetAddress returns the address of the underlying key as a string
	GetAddress() string
	// GetPublicKey returns the public key as bytes
	GetPublicKey() []byte
	// GetKeyringPair returns the keyring pair
	GetKeyringPair() signature.KeyringPair
}

// localInfo is the public information about a locally stored key
type localInfo struct {
	Name    string                `json:"name"`
	Keypair signature.KeyringPair `json:"keypair"`
}

func newLocalInfo(name string, kp signature.KeyringPair) Info {
	return &localInfo{
		Name:    name,
		Keypair: kp,
	}
}

func (i localInfo) GetAddress() string {
	return i.Keypair.Address
}

func (i localInfo) GetName() string {
	return i.Name
}

func (i localInfo) GetPublicKey() []byte {
	return i.Keypair.PublicKey
}

func (i localInfo) GetKeyringPair() signature.KeyringPair {
	return i.Keypair
}

func marshalInfo(i Info) ([]byte, error) {
	marshalled, err := json.Marshal(i)
	if err != nil {
		return nil, err
	}
	return marshalled, nil
}

func unmarshalInfo(bz []byte) (Info, error) {
	localInfo := localInfo{}
	if err := json.Unmarshal(bz, &localInfo); err != nil {
		return nil, err
	}

	return localInfo, nil
}

func infoKey(name string) string { return fmt.Sprintf("%s.%s", name, infoSuffix) }

func infoKeyBz(name string) []byte { return []byte(infoKey(name)) }
