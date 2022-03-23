package keystore

import (
	"encoding/json"
	"fmt"
	"github.com/ComposableFi/go-substrate-rpc-client/v4/signature"

	"github.com/vedhavyas/go-subkey"
)

func newLocalInfo(name string, keypair subkey.KeyPair, address string) Info {
	// reason for using network argument as 42
	// https://github.com/ComposableFi/go-substrate-rpc-client/blob/master/signature/signature.go#L126
	// TODO: handle error from KeyringPairFromSecret method
	kp, _ := signature.KeyringPairFromSecret(string(keypair.Seed()), 42)

	return &localInfo{
		KeyPair:   kp,
		Name:      name,
		PubKey:    keypair.Public(),
		AccountID: keypair.AccountID(),
		Address:   address,
	}
}

// GetType implements Info interface
func (i localInfo) GetAddress() string {
	return i.Address
}

// GetType implements Info interface
func (i localInfo) GetName() string {
	return i.Name
}

func (i localInfo) GetPublicKey() []byte {
	return i.PubKey
}

func (i localInfo) GetKeyringPair() signature.KeyringPair {
	return i.KeyPair
}

// encoding info
func marshalInfo(i Info) ([]byte, error) {
	marshalled, err := json.Marshal(i)
	if err != nil {
		return nil, err
	}
	return marshalled, nil
}

// decoding info
func unmarshalInfo(bz []byte) (info Info, err error) {
	if err := json.Unmarshal(bz, &info); err != nil {
		return nil, err
	}
	return info, nil
}
func infoKey(name string) string   { return fmt.Sprintf("%s.%s", name, infoSuffix) }
func infoKeyBz(name string) []byte { return []byte(infoKey(name)) }
