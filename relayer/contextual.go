package relayer

import (
	"github.com/cosmos/cosmos-sdk/codec"
)

type contextualStdCodec struct {
	codec.JSONMarshaler
	useContext func() func()
}

type contextualAminoCodec struct {
	*codec.LegacyAmino
	useContext func() func()
}

// newContextualCodec creates a codec that sets and resets context
func newContextualStdCodec(cdc codec.JSONMarshaler, useContext func() func()) *contextualStdCodec {
	return &contextualStdCodec{
		JSONMarshaler: cdc,
		useContext:    useContext,
	}
}

// MarshalJSON marshals with the original codec and new context
func (cdc *contextualStdCodec) MarshalJSON(ptr interface{}) ([]byte, error) {
	done := cdc.useContext()
	defer done()

	return cdc.MarshalJSON(ptr)
}

// UnmarshalJSON unmarshals with the original codec and new context
func (cdc *contextualStdCodec) UnmarshalJSON(bz []byte, ptr interface{}) error {
	done := cdc.useContext()
	defer done()

	return cdc.UnmarshalJSON(bz, ptr)
}

func (cdc *contextualStdCodec) MarshalBinaryBare(ptr codec.ProtoMarshaler) ([]byte, error) {
	done := cdc.useContext()
	defer done()

	return cdc.MarshalBinaryBare(ptr)
}

func (cdc *contextualStdCodec) UnmarshalBinaryBare(bz []byte, ptr codec.ProtoMarshaler) error {
	done := cdc.useContext()
	defer done()

	return cdc.UnmarshalBinaryBare(bz, ptr)
}

// newContextualCodec creates a codec that sets and resets context
func newContextualAminoCodec(cdc *codec.LegacyAmino, useContext func() func()) *contextualAminoCodec {
	return &contextualAminoCodec{
		LegacyAmino: cdc,
		useContext:  useContext,
	}
}

// MarshalJSON marshals with the original codec and new context
func (cdc *contextualAminoCodec) MarshalJSON(ptr interface{}) ([]byte, error) {
	done := cdc.useContext()
	defer done()

	return cdc.MarshalJSON(ptr)
}

// UnmarshalJSON unmarshals with the original codec and new context
func (cdc *contextualAminoCodec) UnmarshalJSON(bz []byte, ptr interface{}) error {
	done := cdc.useContext()
	defer done()

	return cdc.UnmarshalJSON(bz, ptr)
}

func (cdc *contextualAminoCodec) MarshalBinaryBare(ptr interface{}) ([]byte, error) {
	done := cdc.useContext()
	defer done()

	return cdc.MarshalBinaryBare(ptr)
}

func (cdc *contextualAminoCodec) UnmarshalBinaryBare(bz []byte, ptr interface{}) error {
	done := cdc.useContext()
	defer done()

	return cdc.UnmarshalBinaryBare(bz, ptr)
}
