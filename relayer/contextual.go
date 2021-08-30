package relayer

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/gogo/protobuf/proto"
)

type contextualStdCodec struct {
	codec.Codec
	useContext func() func()
}

var _ codec.Codec = &contextualStdCodec{}

// newContextualStdCodec creates a codec that sets and resets context
func newContextualStdCodec(cdc codec.Codec, useContext func() func()) *contextualStdCodec {
	return &contextualStdCodec{
		Codec:      cdc,
		useContext: useContext,
	}
}

// MarshalJSON marshals with the original codec and new context
func (cdc *contextualStdCodec) MarshalJSON(ptr proto.Message) ([]byte, error) {
	done := cdc.useContext()
	defer done()

	return cdc.Codec.MarshalJSON(ptr)
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

	return cdc.Codec.UnmarshalJSON(bz, ptr)
}

func (cdc *contextualStdCodec) MustUnmarshalJSON(bz []byte, ptr proto.Message) {
	if err := cdc.UnmarshalJSON(bz, ptr); err != nil {
		panic(err)
	}
}

func (cdc *contextualStdCodec) Marshal(ptr codec.ProtoMarshaler) ([]byte, error) {
	done := cdc.useContext()
	defer done()

	return cdc.Codec.Marshal(ptr)
}

func (cdc *contextualStdCodec) MustMarshal(ptr codec.ProtoMarshaler) []byte {
	out, err := cdc.Marshal(ptr)
	if err != nil {
		panic(err)
	}
	return out
}

func (cdc *contextualStdCodec) Unmarshal(bz []byte, ptr codec.ProtoMarshaler) error {
	done := cdc.useContext()
	defer done()

	return cdc.Codec.Unmarshal(bz, ptr)
}

func (cdc *contextualStdCodec) MustUnmarshal(bz []byte, ptr codec.ProtoMarshaler) {
	if err := cdc.Unmarshal(bz, ptr); err != nil {
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

// func (cdc *contextualAminoCodec) Marshal(ptr codec.ProtoMarshaler) ([]byte, error) {
// 	done := cdc.useContext()
// 	defer done()

// 	return cdc.LegacyAmino.Marshal(ptr)
// }

// func (cdc *contextualAminoCodec) MustMarshal(ptr codec.ProtoMarshaler) []byte {
// 	out, err := cdc.Marshal(ptr)
// 	if err != nil {
// 		panic(err)
// 	}
// 	return out
// }

// func (cdc *contextualAminoCodec) Unmarshal(bz []byte, ptr codec.ProtoMarshaler) error {
// 	done := cdc.useContext()
// 	defer done()

// 	return cdc.LegacyAmino.Unmarshal(bz, ptr)
// }

// func (cdc *contextualAminoCodec) MustUnmarshal(bz []byte, ptr codec.ProtoMarshaler) {
// 	if err := cdc.Unmarshal(bz, ptr); err != nil {
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
