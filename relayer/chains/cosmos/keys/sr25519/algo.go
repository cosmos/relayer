package sr25519

import (
	bip39 "github.com/cosmos/go-bip39"

	tmsr25519 "github.com/cometbft/cometbft/crypto/sr25519"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/types"
)

var Sr25519 = sr25519Algo{}

type sr25519Algo struct {
}

func (s sr25519Algo) Name() hd.PubKeyType {
	return hd.Sr25519Type
}

// Derive derives and returns the sr25519 private key for the given seed and HD path.
func (s sr25519Algo) Derive() hd.DeriveFn {
	return func(mnemonic string, bip39Passphrase, hdPath string) ([]byte, error) {
		seed, err := bip39.NewSeedWithErrorChecking(mnemonic, bip39Passphrase)
		if err != nil {
			return nil, err
		}

		masterPriv, ch := hd.ComputeMastersFromSeed(seed)
		if len(hdPath) == 0 {
			return masterPriv[:], nil
		}
		derivedKey, err := hd.DerivePrivateKeyForPath(masterPriv, ch, hdPath)

		return derivedKey, err
	}
}

// Generate generates a sr25519 private key from the given bytes.
func (s sr25519Algo) Generate() hd.GenerateFn {
	return func(bz []byte) types.PrivKey {
		var bzArr = make([]byte, 32)
		copy(bzArr, bz)

		return &PrivKey{PrivKey: tmsr25519.GenPrivKeyFromSecret(bzArr)}
	}
}
