package processor

import (
	"github.com/cosmos/cosmos-sdk/client"
	sdkcodec "github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	ibc "github.com/cosmos/ibc-go/v7/modules/core"
	solotypes "github.com/cosmos/ibc-go/v7/modules/light-clients/06-solomachine"
	tmtypes "github.com/cosmos/ibc-go/v7/modules/light-clients/07-tendermint"
)

var modBasics = []module.AppModuleBasic{
	ibc.AppModuleBasic{},
	tmtypes.AppModuleBasic{},
	solotypes.AppModuleBasic{},
}

type codec struct {
	InterfaceRegistry types.InterfaceRegistry
	Marshaler         sdkcodec.Codec
	TxConfig          client.TxConfig
	Amino             *sdkcodec.LegacyAmino
}

func makeCodec() codec {
	modBasic := module.NewBasicManager(modBasics...)
	encodingConfig := makeCodecConfig()
	std.RegisterLegacyAminoCodec(encodingConfig.Amino)
	std.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	modBasic.RegisterLegacyAminoCodec(encodingConfig.Amino)
	modBasic.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	return encodingConfig
}

func makeCodecConfig() codec {
	interfaceRegistry := types.NewInterfaceRegistry()
	marshaler := sdkcodec.NewProtoCodec(interfaceRegistry)
	return codec{
		InterfaceRegistry: interfaceRegistry,
		Marshaler:         marshaler,
		TxConfig:          tx.NewTxConfig(marshaler, tx.DefaultSignModes),
		Amino:             sdkcodec.NewLegacyAmino(),
	}
}
