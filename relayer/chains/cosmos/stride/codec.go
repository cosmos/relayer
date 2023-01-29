package stride

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
)

// Originally sourced from https://github.com/Stride-Labs/stride/blob/v5.1.1/x/interchainquery/types/codec.go
// Needed for cosmos sdk Msg implementation in messages.go.

var (
	amino     = codec.NewLegacyAmino()
	ModuleCdc = codec.NewAminoCodec(amino)
)

func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgSubmitQueryResponse{}, "/stride.interchainquery.MsgSubmitQueryResponse", nil)
}

func init() {
	RegisterLegacyAminoCodec(amino)
	cryptocodec.RegisterCrypto(amino)
	amino.Seal()
}
