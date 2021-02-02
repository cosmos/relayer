package relayer

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/gogo/protobuf/proto"
)

type contextualStdCodec struct {
	codec.Marshaler
	useContext func() func()
}

var _ codec.Marshaler = &contextualStdCodec{}

// newContextualStdCodec creates a codec that sets and resets context
func newContextualStdCodec(cdc codec.Marshaler, useContext func() func()) *contextualStdCodec {
	return &contextualStdCodec{
		Marshaler:  cdc,
		useContext: useContext,
	}
}

// MarshalJSON marshals with the original codec and new context
func (cdc *contextualStdCodec) MarshalJSON(ptr proto.Message) ([]byte, error) {
	done := cdc.useContext()
	defer done()

	return cdc.Marshaler.MarshalJSON(ptr)
}

func (cdc *contextualStdCodec) MustMarshalJSON(ptr proto.Message) []byte {
	out, err := cdc.MarshalJSON(ptr)
	if err != nil {
		panic(err)
	}
	return out
}

// UnmarshalJSON unmarshals with the original codec and new context
func (cdc *contextualStdCodec) UnmarshalJSON(bz []byte, ptr proto.Message) error {
	done := cdc.useContext()
	defer done()

	return cdc.Marshaler.UnmarshalJSON(bz, ptr)
}

func (cdc *contextualStdCodec) MustUnmarshalJSON(bz []byte, ptr proto.Message) {
	if err := cdc.UnmarshalJSON(bz, ptr); err != nil {
		panic(err)
	}
}

func (cdc *contextualStdCodec) MarshalBinaryBare(ptr codec.ProtoMarshaler) ([]byte, error) {
	done := cdc.useContext()
	defer done()

	return cdc.Marshaler.MarshalBinaryBare(ptr)
}

func (cdc *contextualStdCodec) MustMarshalBinaryBare(ptr codec.ProtoMarshaler) []byte {
	out, err := cdc.MarshalBinaryBare(ptr)
	if err != nil {
		panic(err)
	}
	return out
}

func (cdc *contextualStdCodec) UnmarshalBinaryBare(bz []byte, ptr codec.ProtoMarshaler) error {
	done := cdc.useContext()
	defer done()

	return cdc.Marshaler.UnmarshalBinaryBare(bz, ptr)
}

func (cdc *contextualStdCodec) MustUnmarshalBinaryBare(bz []byte, ptr codec.ProtoMarshaler) {
	if err := cdc.UnmarshalBinaryBare(bz, ptr); err != nil {
		panic(err)
	}
}

// // newContextualCodec creates a codec that sets and resets context
// func newContextualAminoCodec(cdc *codec.LegacyAmino, useContext func() func()) *contextualAminoCodec {
// 	return &contextualAminoCodec{
// 		LegacyAmino: cdc,
// 		useContext:  useContext,
// 	}
// }

// // MarshalJSON marshals with the original codec and new context
// func (cdc *contextualAminoCodec) MarshalJSON(ptr proto.Message) ([]byte, error) {
// 	done := cdc.useContext()
// 	defer done()

// 	return cdc.LegacyAmino.MarshalJSON(ptr)
// }

// // MustMarshalJSON marshals with the original codec and new context
// func (cdc *contextualAminoCodec) MustMarshalJSON(ptr proto.Message) []byte {
// 	out, err := cdc.MarshalJSON(ptr)
// 	if err != nil {
// 		panic(err)
// 	}
// 	return out
// }

// // UnmarshalJSON unmarshals with the original codec and new context
// func (cdc *contextualAminoCodec) UnmarshalJSON(bz []byte, ptr proto.Message) error {
// 	done := cdc.useContext()
// 	defer done()

// 	return cdc.LegacyAmino.UnmarshalJSON(bz, ptr)
// }

// // MustUnmarshalJSON unmarshals with the original codec and new context
// func (cdc *contextualAminoCodec) MustUnmarshalJSON(bz []byte, ptr proto.Message) {
// 	if err := cdc.UnmarshalJSON(bz, ptr); err != nil {
// 		panic(err)
// 	}
// 	return
// }

// func (cdc *contextualAminoCodec) MarshalBinaryBare(ptr codec.ProtoMarshaler) ([]byte, error) {
// 	done := cdc.useContext()
// 	defer done()

// 	return cdc.LegacyAmino.MarshalBinaryBare(ptr)
// }

// func (cdc *contextualAminoCodec) MustMarshalBinaryBare(ptr codec.ProtoMarshaler) []byte {
// 	out, err := cdc.MarshalBinaryBare(ptr)
// 	if err != nil {
// 		panic(err)
// 	}
// 	return out
// }

// func (cdc *contextualAminoCodec) UnmarshalBinaryBare(bz []byte, ptr codec.ProtoMarshaler) error {
// 	done := cdc.useContext()
// 	defer done()

// 	return cdc.LegacyAmino.UnmarshalBinaryBare(bz, ptr)
// }

// func (cdc *contextualAminoCodec) MustUnmarshalBinaryBare(bz []byte, ptr codec.ProtoMarshaler) {
// 	if err := cdc.UnmarshalBinaryBare(bz, ptr); err != nil {
// 		panic(err)
// 	}
// 	return
// }

// func (cdc *contextualAminoCodec) MarshalBinaryLengthPrefixed(ptr codec.ProtoMarshaler) ([]byte, error) {
// 	done := cdc.useContext()
// 	defer done()

// 	return cdc.LegacyAmino.MarshalBinaryLengthPrefixed(ptr)
// }

// func (cdc *contextualAminoCodec) MustMarshalBinaryLengthPrefixed(ptr codec.ProtoMarshaler) []byte {
// 	out, err := cdc.MarshalBinaryLengthPrefixed(ptr)
// 	if err != nil {
// 		panic(err)
// 	}
// 	return out
// }

// func (cdc *contextualAminoCodec) UnmarshalBinaryLengthPrefixed(bz []byte, ptr codec.ProtoMarshaler) error {
// 	done := cdc.useContext()
// 	defer done()

// 	return cdc.LegacyAmino.UnmarshalBinaryLengthPrefixed(bz, ptr)
// }

// func (cdc *contextualAminoCodec) MustUnmarshalBinaryLengthPrefixed(bz []byte, ptr codec.ProtoMarshaler) {
// 	if err := cdc.UnmarshalBinaryLengthPrefixed(bz, ptr); err != nil {
// 		panic(err)
// 	}
// 	return
// }
