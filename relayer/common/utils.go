package common

import (
	"encoding/hex"
	"strings"
)

func MustHexStrToBytes(hex_string string) []byte {
	enc, _ := hex.DecodeString(strings.TrimPrefix(hex_string, "0x"))
	return enc
}
