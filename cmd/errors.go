package cmd

import (
	"errors"
	"fmt"
)

func errKeyExists(name string) error {
	return errors.New("a key with name %s already exists", name)
}

func errKeyDoesntExist(name string) error {
	return errors.New("a key with name %s doesn't exist", name)
}

func errChainNotFound(chainName string) error {
	return errors.New("chain with name \"%s\" not found in config. consider running `rly chains add %s`", chainName, chainName)
}

func invalidRpcAddr(rpcAddr string) error {
	return errors.New("rpc-addr %s is  not valid", rpcAddr)
}

var (
	errMultipleAddFlags   = errors.New("expected either --file/-f OR --url/u, found multiple")
	errInvalidTestnetFlag = errors.New("cannot use --testnet with --file/-f OR --url/u, must be used alone")
)
