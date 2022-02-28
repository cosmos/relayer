package keystore

import (
	"encoding/json"
	"fmt"

	"github.com/vedhavyas/go-subkey"
)

func newLocalInfo(name string, keypair subkey.KeyPair, address string) Info {
	return &localInfo{
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
