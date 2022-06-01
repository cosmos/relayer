package cosmos

import (
	"os"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	cosmosClient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v3/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	commitmenttypes "github.com/cosmos/ibc-go/v3/modules/core/23-commitment/types"
	ibctypes "github.com/cosmos/ibc-go/v3/modules/core/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	libclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
)

type Codec struct {
	InterfaceRegistry types.InterfaceRegistry
	Marshaler         codec.Codec
	TxConfig          client.TxConfig
	Amino             *codec.LegacyAmino
}

func makeCodecConfig() Codec {
	interfaceRegistry := types.NewInterfaceRegistry()
	marshaler := codec.NewProtoCodec(interfaceRegistry)
	chantypes.RegisterInterfaces(interfaceRegistry)
	clienttypes.RegisterInterfaces(interfaceRegistry)
	conntypes.RegisterInterfaces(interfaceRegistry)
	ibctypes.RegisterInterfaces(interfaceRegistry)
	transfertypes.RegisterInterfaces(interfaceRegistry)
	commitmenttypes.RegisterInterfaces(interfaceRegistry)

	return Codec{
		InterfaceRegistry: interfaceRegistry,
		Marshaler:         marshaler,
		TxConfig:          tx.NewTxConfig(marshaler, tx.DefaultSignModes),
		Amino:             codec.NewLegacyAmino(),
	}
}

func makeCodec(moduleBasics []module.AppModuleBasic) Codec {
	modBasic := module.NewBasicManager(moduleBasics...)
	encodingConfig := makeCodecConfig()
	std.RegisterLegacyAminoCodec(encodingConfig.Amino)
	std.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	modBasic.RegisterLegacyAminoCodec(encodingConfig.Amino)
	modBasic.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	return encodingConfig
}

func newClient(addr string) (rpcclient.Client, error) {
	httpClient, err := libclient.DefaultHTTPClient(addr)
	if err != nil {
		return nil, err
	}

	httpClient.Timeout = 10 * time.Second
	rpcClient, err := rpchttp.NewWithClient(addr, "/websocket", httpClient)
	if err != nil {
		return nil, err
	}

	return rpcClient, nil
}

func getCosmosClient(rpcAddress string, chainID string) (*cosmosClient.Context, error) {
	client, err := newClient(rpcAddress)
	if err != nil {
		return nil, err
	}
	codec := makeCodec([]module.AppModuleBasic{})
	return &cosmosClient.Context{
		Client:            client,
		ChainID:           chainID,
		Input:             os.Stdin,
		Output:            os.Stdout,
		TxConfig:          codec.TxConfig,
		Codec:             codec.Marshaler,
		InterfaceRegistry: codec.InterfaceRegistry,
		OutputFormat:      "json",
	}, nil
}
