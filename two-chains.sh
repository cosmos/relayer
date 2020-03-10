#!/bin/bash

GAIA_DIR="$GOPATH/src/github.com/cosmos/gaia"
RELAYER_DIR="$GOPATH/src/github.com/cosmos/relayer"
GAIA_BRANCH=ibc-alpha
GAIA_CONF=$(mktemp -d)
RLY_CONF=$(mktemp -d)

sleep 1

echo "Killing existing gaiad instances..."
killall gaiad

set -e

echo "Building Gaia..."
cd $GAIA_DIR
git checkout $GAIA_BRANCH &> /dev/null
make install &> /dev/null

echo "Building Relayer..."
cd $RELAYER_DIR
go build -o $GOBIN/relayer main.go

echo "Generating gaia configurations..."
cd $GAIA_CONF && mkdir ibc-testnets && cd ibc-testnets
echo -e "\n" | gaiad testnet -o ibc0 --v 1 --chain-id ibc0 --node-dir-prefix n --keyring-backend test &> /dev/null
echo -e "\n" | gaiad testnet -o ibc1 --v 1 --chain-id ibc1 --node-dir-prefix n --keyring-backend test &> /dev/null

echo "Generating relayer configurations..."
mkdir $RLY_CONF/config
cp $RELAYER_DIR/two-chains.yaml $RLY_CONF/config/config.yaml

if [ "$(uname)" = "Linux" ]; then
  sed -i 's/"leveldb"/"goleveldb"/g' ibc0/n0/gaiad/config/config.toml
  sed -i 's/"leveldb"/"goleveldb"/g' ibc1/n0/gaiad/config/config.toml
  sed -i 's#"tcp://0.0.0.0:26656"#"tcp://0.0.0.0:26556"#g' ibc1/n0/gaiad/config/config.toml
  sed -i 's#"tcp://0.0.0.0:26657"#"tcp://0.0.0.0:26557"#g' ibc1/n0/gaiad/config/config.toml
  sed -i 's#"localhost:6060"#"localhost:6061"#g' ibc1/n0/gaiad/config/config.toml
  sed -i 's#"tcp://127.0.0.1:26658"#"tcp://127.0.0.1:26558"#g' ibc1/n0/gaiad/config/config.toml
else
  sed -i '' 's/"leveldb"/"goleveldb"/g' ibc0/n0/gaiad/config/config.toml
  sed -i '' 's/"leveldb"/"goleveldb"/g' ibc1/n0/gaiad/config/config.toml
  sed -i '' 's#"tcp://0.0.0.0:26656"#"tcp://0.0.0.0:26556"#g' ibc1/n0/gaiad/config/config.toml
  sed -i '' 's#"tcp://0.0.0.0:26657"#"tcp://0.0.0.0:26557"#g' ibc1/n0/gaiad/config/config.toml
  sed -i '' 's#"localhost:6060"#"localhost:6061"#g' ibc1/n0/gaiad/config/config.toml
  sed -i '' 's#"tcp://127.0.0.1:26658"#"tcp://127.0.0.1:26558"#g' ibc1/n0/gaiad/config/config.toml
fi;

gaiacli config --home ibc0/n0/gaiacli/ chain-id ibc0 &> /dev/null
gaiacli config --home ibc1/n0/gaiacli/ chain-id ibc1 &> /dev/null
gaiacli config --home ibc0/n0/gaiacli/ output json &> /dev/null
gaiacli config --home ibc1/n0/gaiacli/ output json &> /dev/null
gaiacli config --home ibc0/n0/gaiacli/ node http://localhost:26657 &> /dev/null
gaiacli config --home ibc1/n0/gaiacli/ node http://localhost:26557 &> /dev/null

echo "Starting Gaiad instances..."
gaiad --home ibc0/n0/gaiad start --pruning=nothing > ibc0.log 2>&1 &
gaiad --home ibc1/n0/gaiad start --pruning=nothing > ibc1.log 2>&1 & 

echo "Set the following env to make working with the running chains easier:"
echo 
echo "export RLY=$RLY_CONF"
echo "export GAIA=$GAIA_CONF"
echo
echo "Key Seeds for importing into gaiacli if necessary:"
SEED0=$(jq -r '.secret' ibc0/n0/gaiacli/key_seed.json)
SEED1=$(jq -r '.secret' ibc1/n0/gaiacli/key_seed.json)
echo "  ibc0 -> $SEED0"
echo "  ibc1 -> $SEED1"
echo
echo "NOTE: Below are account addresses for each chain. They are also validator addresses:"
echo "  ibc0 address: $(relayer --home $RLY_CONF keys restore ibc0 testkey "$SEED0" -a)"
echo "  ibc1 address: $(relayer --home $RLY_CONF keys restore ibc1 testkey "$SEED1" -a)"
echo
echo "Initializing lite clients..."
sleep 8
relayer --home $RLY_CONF lite init ibc0 -f
relayer --home $RLY_CONF lite init ibc1 -f
echo "Creating client ibconeclient for ibc1 on ibc0 and ibconzeroclient for ibc0 on ibc1..."
sleep 5
relayer --home $RLY_CONF tx client ibc0 ibc1 ibconeclient
relayer --home $RLY_CONF tx client ibc1 ibc0 ibczeroclient
echo
echo "Updating clients ibconeclient and ibczeroclient..."
sleep 5
relayer --home $RLY_CONF tx update-client ibc0 ibc1 ibconeclient
relayer --home $RLY_CONF tx update-client ibc1 ibc0 ibczeroclient
echo
echo "Create connection raw"
sleep 5
relayer --home $RLY_CONF tx raw conn-init ibc0 ibc1 ibconeclient ibczeroclient connectionidtest connectionidtest
sleep 5
relayer --home $RLY_CONF tx raw conn-try ibc1 ibc0 ibczeroclient ibconeclient connectionidtest connectionidtest
sleep 5
relayer --home $RLY_CONF tx raw conn-ack ibc0 ibc1 ibconeclient ibczeroclient connectionidtest connectionidtest
sleep 5
relayer --home $RLY_CONF tx raw conn-confirm ibc1 ibc0 ibczeroclient ibconeclient connectionidtest connectionidtest