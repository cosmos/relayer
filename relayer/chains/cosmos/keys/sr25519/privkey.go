package sr25519

import (
	tmsr25519 "github.com/cometbft/cometbft/crypto/sr25519"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
)

const (
	PrivKeySize = 32
	PrivKeyName = "tendermint/PrivKeySr25519"
)

type PrivKey struct {
	tmsr25519.PrivKey
}

// type conversion
func (m *PrivKey) PubKey() cryptotypes.PubKey {
	pk, ok := m.PrivKey.PubKey().(tmsr25519.PubKey)
	if !ok {
		panic("invalid public key type for sr25519 private key")
	}
	return &PubKey{Key: pk}
}

// type conversion
func (m *PrivKey) Equals(other cryptotypes.LedgerPrivKey) bool {
	sk2, ok := other.(*PrivKey)
	if !ok {
		return false
	}
	return m.PrivKey.Equals(sk2.PrivKey)
}

func (m *PrivKey) ProtoMessage() {}

func (m *PrivKey) Reset() {
	m.PrivKey = tmsr25519.PrivKey{}
}

func (m *PrivKey) String() string {
	return string(m.Bytes())
}

func GenPrivKey() *PrivKey {
	return &PrivKey{tmsr25519.GenPrivKey()}
}
