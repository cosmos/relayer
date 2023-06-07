package common

import (
	"golang.org/x/crypto/sha3"
)

func AppendKeccak256(out []byte, data ...[]byte) []byte {
	d := sha3.NewLegacyKeccak256()
	for _, b := range data {
		d.Write(b)
	}
	return d.Sum(out)
}

func Sha3keccak256(data ...[]byte) []byte {
	return AppendKeccak256(nil, data...)
}
