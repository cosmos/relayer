package stride

import (
	"cosmossdk.io/api/tendermint/crypto"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	proto "github.com/cosmos/gogoproto/proto"
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

func RegisterInterfaces(registry types.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgSubmitQueryResponse{},
	)
}

func RegisterCometTypes() {
	proto.RegisterType((*crypto.Proof)(nil), "cometbft.crypto.Proof")
	proto.RegisterType((*crypto.ValueOp)(nil), "cometbft.crypto.ValueOp")
	proto.RegisterType((*crypto.DominoOp)(nil), "cometbft.crypto.DominoOp")
	proto.RegisterType((*crypto.ProofOp)(nil), "cometbft.crypto.ProofOp")
	proto.RegisterType((*crypto.ProofOps)(nil), "cometbft.crypto.ProofOps")
}

func init() {
	RegisterCometTypes()
	RegisterLegacyAminoCodec(amino)
	cryptocodec.RegisterCrypto(amino)
	amino.Seal()
}
