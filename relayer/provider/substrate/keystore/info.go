package keystore

import (
	"encoding/json"
	"fmt"

	"github.com/ComposableFi/go-substrate-rpc-client/v4/signature"
)

func newLocalInfo(name string, kp signature.KeyringPair) (Info, error) {

	return &localInfo{
		Name:    name,
		Keypair: kp,
	}, nil

}

// GetType implements Info interface
func (i localInfo) GetAddress() string {
	return i.Keypair.Address
}

// GetType implements Info interface
func (i localInfo) GetName() string {
	return i.Name
}

func (i localInfo) GetPublicKey() []byte {
	return i.Keypair.PublicKey
}

func (i localInfo) GetKeyringPair() signature.KeyringPair {
	return i.Keypair
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
func unmarshalInfo(bz []byte) (Info, error) {
	localInfo := localInfo{}
	if err := json.Unmarshal(bz, &localInfo); err != nil {
		return nil, err
	}

	return localInfo, nil
}
func infoKey(name string) string   { return fmt.Sprintf("%s.%s", name, infoSuffix) }
func infoKeyBz(name string) []byte { return []byte(infoKey(name)) }
