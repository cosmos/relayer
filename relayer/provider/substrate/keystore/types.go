package keystore

import (
	"github.com/99designs/keyring"
	"github.com/ComposableFi/go-substrate-rpc-client/v4/signature"
)

type keystore struct {
	db keyring.Keyring
}

// Keyring exposes operations over a backend supported by github.com/99designs/keyring.
type Keyring interface {
	// List all keys.
	List() ([]Info, error)

	// Key and KeyByAddress return keys by uid and address respectively.
	Key(uid string) (Info, error)

	// Delete and DeleteByAddress remove keys from the keyring.
	Delete(uid string) error

	// NewAccount converts a mnemonic to a private key and BIP-39 HD Path and persists it.
	// It fails if there is an existing key Info with the same address.
	NewAccount(name, mnemonic string, network uint8) (Info, error)
}

// Info is the publicly exposed information about a keypair
type Info interface {
	// Name of the key
	GetName() string
	// Address
	GetAddress() string
	// Public key
	GetPublicKey() []byte
	// KeyPair
	GetKeyringPair() signature.KeyringPair
}

// localInfo is the public information about a locally stored key
// Note: Algo must be last field in struct for backwards amino compatibility
type localInfo struct {
	KeyPair   signature.KeyringPair `json:"key_pair"`
	Name      string                `json:"name"`
	PubKey    []byte                `json:"pubkey"`
	AccountID []byte                `json:"account_id"`
	Address   string                `json:"address"`
}
