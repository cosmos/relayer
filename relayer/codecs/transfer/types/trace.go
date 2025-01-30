package types

import (
	"crypto/sha256"
	fmt "fmt"
	"strings"

	tmbytes "github.com/cometbft/cometbft/libs/bytes"
)

const (
	// DenomPrefix is the prefix used for internal SDK coin representation.
	DenomPrefix = "ibc"
)

// Traces defines a wrapper type for a slice of DenomTrace.
type Traces []DenomTrace

// GetPrefix returns the receiving denomination prefix composed by the trace info and a separator.
func (dt DenomTrace) GetPrefix() string {
	return dt.Path + "/"
}

// Hash returns the hex bytes of the SHA256 hash of the DenomTrace fields using the following formula:
//
// hash = sha256(tracePath + "/" + baseDenom)
func (dt DenomTrace) Hash() tmbytes.HexBytes {
	hash := sha256.Sum256([]byte(dt.GetFullDenomPath()))
	return hash[:]
}

// IBCDenom a coin denomination for an ICS20 fungible token in the format
// 'ibc/{hash(tracePath + baseDenom)}'. If the trace is empty, it will return the base denomination.
func (dt DenomTrace) IBCDenom() string {
	if dt.Path != "" {
		return fmt.Sprintf("%s/%s", DenomPrefix, dt.Hash())
	}
	return dt.BaseDenom
}

// GetFullDenomPath returns the full denomination according to the ICS20 specification:
// tracePath + "/" + baseDenom
// If there exists no trace then the base denomination is returned.
func (dt DenomTrace) GetFullDenomPath() string {
	if dt.Path == "" {
		return dt.BaseDenom
	}
	return dt.GetPrefix() + dt.BaseDenom
}

func ErrUnknownRequest(err error) bool {
	return strings.Contains(err.Error(), "unknown query path")
}
