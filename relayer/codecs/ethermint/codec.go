package ethermint

import (
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// RegisterInterfaces register the Ethermint key concrete types.
func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations((*cryptotypes.PubKey)(nil), &PubKey{})
	registry.RegisterImplementations((*cryptotypes.PrivKey)(nil), &PrivKey{})

	registry.RegisterImplementations(
		(*authtypes.AccountI)(nil),
		&EthAccount{},
	)
	registry.RegisterImplementations(
		(*authtypes.GenesisAccount)(nil),
		&EthAccount{},
	)

}
