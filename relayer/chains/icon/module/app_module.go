package module

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/gogoproto/proto"
	"github.com/cosmos/ibc-go/v7/modules/core/exported"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/icon-project/IBC-Integration/libraries/go/common/icon"
	"github.com/icon-project/IBC-Integration/libraries/go/common/tendermint"
	"github.com/spf13/cobra"
)

// AppModuleBasic defines the basic application module used by the module.
type AppModuleBasic struct{}

// Name returns the module's name.
func (AppModuleBasic) Name() string {
	return "icon_chain_provider"
}

// RegisterLegacyAminoCodec does nothing. IBC does not support amino.
func (AppModuleBasic) RegisterLegacyAminoCodec(*codec.LegacyAmino) {}

type MerkleProofState interface {
	proto.Message
}

// RegisterInterfaces registers module concrete types into protobuf Any.
func (AppModuleBasic) RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations(
		(*exported.ClientState)(nil),
		&tendermint.ClientState{},
	)
	registry.RegisterInterface(
		"icon.types.v1.MerkleProofs",
		(*MerkleProofState)(nil),
		&icon.MerkleProofs{},
	)

}

// RegisterGRPCGatewayRoutes registers the gRPC Gateway routes for the ibc module.
func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux) {
	panic("not implemented")
}

// GetTxCmd returns the root tx command for the ibc module.
func (AppModuleBasic) GetTxCmd() *cobra.Command {
	panic("not implemented")
}

// GetQueryCmd returns no root query command for the ibc module.
func (AppModuleBasic) GetQueryCmd() *cobra.Command {
	panic("not implemented")
}
