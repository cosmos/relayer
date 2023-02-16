package injective

import (
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

var (
	_ cryptotypes.PubKey  = &PubKey{}
	_ cryptotypes.PrivKey = &PrivKey{}
)

type (
	ExtensionOptionsWeb3TxI interface{}
)

// RegisterInterfaces registers the ethsecp256k1 implementations of tendermint crypto types.
func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations((*cryptotypes.PubKey)(nil), &PubKey{})
	registry.RegisterImplementations((*cryptotypes.PrivKey)(nil), &PrivKey{})

	registry.RegisterInterface("injective.types.v1beta1.EthAccount", (*authtypes.AccountI)(nil))

	registry.RegisterImplementations(
		(*authtypes.AccountI)(nil),
		&EthAccount{},
	)

	registry.RegisterImplementations(
		(*authtypes.GenesisAccount)(nil),
		&EthAccount{},
	)

	registry.RegisterInterface("injective.types.v1beta1.ExtensionOptionsWeb3Tx", (*ExtensionOptionsWeb3TxI)(nil))
	registry.RegisterImplementations(
		(*ExtensionOptionsWeb3TxI)(nil),
		&ExtensionOptionsWeb3Tx{},
	)
}
