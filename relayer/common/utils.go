package common

import (
	"encoding/hex"
	"strings"
)

func MustHexStrToBytes(s string) []byte {
	enc, _ := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	return enc
}
