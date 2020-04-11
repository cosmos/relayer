package relayer

import (
	"github.com/cosmos/cosmos-sdk/codec"
	stdcodec "github.com/cosmos/cosmos-sdk/codec/std"
)

type contextualStdCodec struct {
	*stdcodec.Codec
	setContext func() func()
}

type contextualAminoCodec struct {
	*codec.Codec
	setContext func() func()
}

// newContextualCodec creates a codec that sets and resets context
func newContextualStdCodec(cdc *stdcodec.Codec, setContext func() func()) *contextualStdCodec {
	return &contextualStdCodec{
		cdc,
		setContext,
	}
}

// MarshalJSON marshals with the original codec and new context
func (cdc *contextualStdCodec) MarshalJSON(ptr interface{}) ([]byte, error) {
	reset := cdc.setContext()
	defer reset()
	return cdc.Codec.MarshalJSON(ptr)
}

// UnmarshalJSON unmarshals with the original codec and new context
func (cdc *contextualStdCodec) UnmarshalJSON(bz []byte, ptr interface{}) error {
	reset := cdc.setContext()
	defer reset()
	return cdc.Codec.UnmarshalJSON(bz, ptr)
}

func (cdc *contextualStdCodec) MarshalBinaryBare(ptr codec.ProtoMarshaler) ([]byte, error) {
	reset := cdc.setContext()
	defer reset()
	return cdc.Codec.MarshalBinaryBare(ptr)
}

func (cdc *contextualStdCodec) UnmarshalBinaryBare(bz []byte, ptr codec.ProtoMarshaler) error {
	reset := cdc.setContext()
	defer reset()
	return cdc.Codec.UnmarshalBinaryBare(bz, ptr)
}

// newContextualCodec creates a codec that sets and resets context
func newContextualAminoCodec(cdc *codec.Codec, setContext func() func()) *contextualAminoCodec {
	return &contextualAminoCodec{
		cdc,
		setContext,
	}
}

// MarshalJSON marshals with the original codec and new context
func (cdc *contextualAminoCodec) MarshalJSON(ptr interface{}) ([]byte, error) {
	reset := cdc.setContext()
	defer reset()
	return cdc.Codec.MarshalJSON(ptr)
}

// UnmarshalJSON unmarshals with the original codec and new context
func (cdc *contextualAminoCodec) UnmarshalJSON(bz []byte, ptr interface{}) error {
	reset := cdc.setContext()
	defer reset()
	return cdc.Codec.UnmarshalJSON(bz, ptr)
}

func (cdc *contextualAminoCodec) MarshalBinaryBare(ptr interface{}) ([]byte, error) {
	reset := cdc.setContext()
	defer reset()
	return cdc.Codec.MarshalBinaryBare(ptr)
}

func (cdc *contextualAminoCodec) UnmarshalBinaryBare(bz []byte, ptr interface{}) error {
	reset := cdc.setContext()
	defer reset()
	return cdc.Codec.UnmarshalBinaryBare(bz, ptr)
}
