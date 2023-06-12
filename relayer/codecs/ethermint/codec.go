package ethermint

import (
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	etherminttypes "github.com/evmos/ethermint/types"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
)

// RegisterInterfaces register the Ethermint key concrete types.
func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations((*cryptotypes.PubKey)(nil), &PubKey{})
	registry.RegisterImplementations((*cryptotypes.PrivKey)(nil), &PrivKey{})

	registry.RegisterImplementations(
		(*authtypes.AccountI)(nil),
		&etherminttypes.EthAccount{},
	)
	registry.RegisterImplementations(
		(*authtypes.GenesisAccount)(nil),
		&etherminttypes.EthAccount{},
	)
	registry.RegisterImplementations(
		(*tx.TxExtensionOptionI)(nil),
		&etherminttypes.ExtensionOptionsWeb3Tx{},
		&etherminttypes.ExtensionOptionDynamicFeeTx{},
		&evmtypes.ExtensionOptionsEthereumTx{},
	)
	registry.RegisterImplementations(
		(*sdk.Msg)(nil),
		&evmtypes.MsgEthereumTx{},
		&evmtypes.MsgUpdateParams{},
	)
	registry.RegisterInterface(
		"ethermint.evm.v1.TxData",
		(*evmtypes.TxData)(nil),
		&evmtypes.DynamicFeeTx{},
		&evmtypes.AccessListTx{},
		&evmtypes.LegacyTx{},
	)
}
