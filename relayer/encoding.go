package relayer

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/simapp"
	"github.com/cosmos/cosmos-sdk/simapp/params"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
)

// MakeEncodingConfig returns the encoding txConfig for the chain
func (c *Chain) MakeEncodingConfig() params.EncodingConfig {
	amino := codec.NewLegacyAmino()
	interfaceRegistry := types.NewInterfaceRegistry()
	marshaler := c.NewProtoCodec(interfaceRegistry)
	txCfg := tx.NewTxConfig(marshaler, tx.DefaultSignModes)

	encodingConfig := params.EncodingConfig{
		InterfaceRegistry: interfaceRegistry,
		Marshaler:         marshaler,
		TxConfig:          txCfg,
		Amino:             amino,
	}

	std.RegisterLegacyAminoCodec(encodingConfig.Amino)
	std.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	simapp.ModuleBasics.RegisterLegacyAminoCodec(encodingConfig.Amino)
	simapp.ModuleBasics.RegisterInterfaces(encodingConfig.InterfaceRegistry)

	return encodingConfig
}

// ProtoCodec defines a codec that utilizes Protobuf for both binary and JSON
// encoding.
type ProtoCodec struct {
	useContext        func() func()
	interfaceRegistry types.InterfaceRegistry
}

var _ codec.Marshaler = &ProtoCodec{}
var _ codec.ProtoCodecMarshaler = &ProtoCodec{}

// NewProtoCodec returns a reference to a new ProtoCodec
func (c *Chain) NewProtoCodec(interfaceRegistry types.InterfaceRegistry) *ProtoCodec {
	return &ProtoCodec{useContext: c.UseSDKContext, interfaceRegistry: interfaceRegistry}
}

// MarshalBinaryBare implements BinaryMarshaler.MarshalBinaryBare method.
func (pc *ProtoCodec) MarshalBinaryBare(o codec.ProtoMarshaler) ([]byte, error) {
	reset := pc.useContext()
	defer reset()
	return o.Marshal()
}

// MustMarshalBinaryBare implements BinaryMarshaler.MustMarshalBinaryBare method.
func (pc *ProtoCodec) MustMarshalBinaryBare(o codec.ProtoMarshaler) []byte {
	bz, err := pc.MarshalBinaryBare(o)
	if err != nil {
		panic(err)
	}

	return bz
}

// MarshalBinaryLengthPrefixed implements BinaryMarshaler.MarshalBinaryLengthPrefixed method.
func (pc *ProtoCodec) MarshalBinaryLengthPrefixed(o codec.ProtoMarshaler) ([]byte, error) {
	reset := pc.useContext()
	defer reset()
	bz, err := pc.MarshalBinaryBare(o)
	if err != nil {
		return nil, err
	}

	var sizeBuf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(sizeBuf[:], uint64(o.Size()))
	return append(sizeBuf[:n], bz...), nil
}

// MustMarshalBinaryLengthPrefixed implements BinaryMarshaler.MustMarshalBinaryLengthPrefixed method.
func (pc *ProtoCodec) MustMarshalBinaryLengthPrefixed(o codec.ProtoMarshaler) []byte {
	bz, err := pc.MarshalBinaryLengthPrefixed(o)
	if err != nil {
		panic(err)
	}

	return bz
}

// UnmarshalBinaryBare implements BinaryMarshaler.UnmarshalBinaryBare method.
func (pc *ProtoCodec) UnmarshalBinaryBare(bz []byte, ptr codec.ProtoMarshaler) error {
	reset := pc.useContext()
	defer reset()
	err := ptr.Unmarshal(bz)
	if err != nil {
		return err
	}
	err = types.UnpackInterfaces(ptr, pc)
	if err != nil {
		return err
	}
	return nil
}

// MustUnmarshalBinaryBare implements BinaryMarshaler.MustUnmarshalBinaryBare method.
func (pc *ProtoCodec) MustUnmarshalBinaryBare(bz []byte, ptr codec.ProtoMarshaler) {
	if err := pc.UnmarshalBinaryBare(bz, ptr); err != nil {
		panic(err)
	}
}

// UnmarshalBinaryLengthPrefixed implements BinaryMarshaler.UnmarshalBinaryLengthPrefixed method.
func (pc *ProtoCodec) UnmarshalBinaryLengthPrefixed(bz []byte, ptr codec.ProtoMarshaler) error {
	reset := pc.useContext()
	defer reset()
	size, n := binary.Uvarint(bz)
	if n < 0 {
		return fmt.Errorf("invalid number of bytes read from length-prefixed encoding: %d", n)
	}

	if size > uint64(len(bz)-n) {
		return fmt.Errorf("not enough bytes to read; want: %v, got: %v", size, len(bz)-n)
	} else if size < uint64(len(bz)-n) {
		return fmt.Errorf("too many bytes to read; want: %v, got: %v", size, len(bz)-n)
	}

	bz = bz[n:]
	return pc.UnmarshalBinaryBare(bz, ptr)
}

// MustUnmarshalBinaryLengthPrefixed implements BinaryMarshaler.MustUnmarshalBinaryLengthPrefixed method.
func (pc *ProtoCodec) MustUnmarshalBinaryLengthPrefixed(bz []byte, ptr codec.ProtoMarshaler) {
	if err := pc.UnmarshalBinaryLengthPrefixed(bz, ptr); err != nil {
		panic(err)
	}
}

// MarshalJSON implements JSONMarshaler.MarshalJSON method,
// it marshals to JSON using proto codec.
func (pc *ProtoCodec) MarshalJSON(o proto.Message) ([]byte, error) {
	reset := pc.useContext()
	defer reset()
	m, ok := o.(codec.ProtoMarshaler)
	if !ok {
		return nil, fmt.Errorf("cannot protobuf JSON encode unsupported type: %T", o)
	}

	return codec.ProtoMarshalJSON(m, pc.interfaceRegistry)
}

// MustMarshalJSON implements JSONMarshaler.MustMarshalJSON method,
// it executes MarshalJSON except it panics upon failure.
func (pc *ProtoCodec) MustMarshalJSON(o proto.Message) []byte {
	bz, err := pc.MarshalJSON(o)
	if err != nil {
		panic(err)
	}

	return bz
}

// UnmarshalJSON implements JSONMarshaler.UnmarshalJSON method,
// it unmarshals from JSON using proto codec.
func (pc *ProtoCodec) UnmarshalJSON(bz []byte, ptr proto.Message) error {
	reset := pc.useContext()
	defer reset()
	m, ok := ptr.(codec.ProtoMarshaler)
	if !ok {
		return fmt.Errorf("cannot protobuf JSON decode unsupported type: %T", ptr)
	}

	err := jsonpb.Unmarshal(strings.NewReader(string(bz)), m)
	if err != nil {
		return err
	}

	return types.UnpackInterfaces(ptr, pc)
}

// MustUnmarshalJSON implements JSONMarshaler.MustUnmarshalJSON method,
// it executes UnmarshalJSON except it panics upon failure.
func (pc *ProtoCodec) MustUnmarshalJSON(bz []byte, ptr proto.Message) {
	if err := pc.UnmarshalJSON(bz, ptr); err != nil {
		panic(err)
	}
}

// UnpackAny implements AnyUnpacker.UnpackAny method,
// it unpacks the value in any to the interface pointer passed in as
// iface.
func (pc *ProtoCodec) UnpackAny(any *types.Any, iface interface{}) error {
	reset := pc.useContext()
	defer reset()
	return pc.interfaceRegistry.UnpackAny(any, iface)
}

func (pc *ProtoCodec) InterfaceRegistry() types.InterfaceRegistry {
	return pc.interfaceRegistry
}
