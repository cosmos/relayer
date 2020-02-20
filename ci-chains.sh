#!/bin/bash

GAIAD="/tmp/build/gaiad"
GAIACLI="/tmp/build/gaiacli"
RELAYER="/tmp/build/relayer"

GAIA_CONF=$(mktemp -d)
RLY_CONF=$(mktemp -d)

sleep 1

echo "Killing existing gaiad instances..."
killall gaiad

set -e

echo "Generating relayer configurations..."
mkdir -p $RLY_CONF/config
echo "cp $(pwd)/two-chains.yaml $RLY_CONF/config/config.yaml"
cp $(pwd)/two-chains.yaml $RLY_CONF/config/config.yaml

echo "Generating gaia configurations..."
cd $GAIA_CONF && mkdir ibc-testnets && cd ibc-testnets
echo -e "\n" | $GAIAD testnet -o ibc0 --v 1 --chain-id ibc0 --node-dir-prefix n --keyring-backend test
echo -e "\n" | $GAIAD testnet -o ibc1 --v 1 --chain-id ibc1 --node-dir-prefix n --keyring-backend test

sed -i 's/"leveldb"/"goleveldb"/g' ibc0/n0/gaiad/config/config.toml
sed -i 's/"leveldb"/"goleveldb"/g' ibc1/n0/gaiad/config/config.toml
sed -i 's#"tcp://0.0.0.0:26656"#"tcp://0.0.0.0:26556"#g' ibc1/n0/gaiad/config/config.toml
sed -i 's#"tcp://0.0.0.0:26657"#"tcp://0.0.0.0:26557"#g' ibc1/n0/gaiad/config/config.toml
sed -i 's#"localhost:6060"#"localhost:6061"#g' ibc1/n0/gaiad/config/config.toml
sed -i 's#"tcp://127.0.0.1:26658"#"tcp://127.0.0.1:26558"#g' ibc1/n0/gaiad/config/config.toml

$GAIACLI config --home ibc0/n0/gaiacli/ chain-id ibc0
$GAIACLI config --home ibc1/n0/gaiacli/ chain-id ibc1
$GAIACLI config --home ibc0/n0/gaiacli/ output json
$GAIACLI config --home ibc1/n0/gaiacli/ output json
$GAIACLI config --home ibc0/n0/gaiacli/ node http://localhost:26657
$GAIACLI config --home ibc1/n0/gaiacli/ node http://localhost:26557

echo "Starting Gaiad instances..."
nohup $GAIAD --home ibc0/n0/gaiad start > ibc0.log &
nohup $GAIAD --home ibc1/n0/gaiad start > ibc1.log &

echo "Adding gaiacli keys to the relayer"
$RELAYER --home $RLY_CONF keys restore ibc0 testkey "$(jq -r '.secret' ibc0/n0/gaiacli/key_seed.json)" -a
$RELAYER --home $RLY_CONF keys restore ibc1 testkey "$(jq -r '.secret' ibc1/n0/gaiacli/key_seed.json)" -a

echo "Wait for first block"
sleep 12

echo "Check account balances"
$RELAYER --home $RLY_CONF q account ibc0
$RELAYER --home $RLY_CONF q account ibc1

echo "Initialize lite clients"
$RELAYER --home $RLY_CONF lite init ibc0 -f
$RELAYER --home $RLY_CONF lite init ibc1 -f

echo "Create clients"
$RELAYER --home $RLY_CONF tx client ibc0 ibc1 ibconeclient
$RELAYER --home $RLY_CONF tx client ibc1 ibc0 ibczeroclient

echo "Query headers"
$RELAYER --home $RLY_CONF q header ibc0   
$RELAYER --home $RLY_CONF q header ibc0

echo "Query node-state"
$RELAYER --home $RLY_CONF q node-state ibc0   
$RELAYER --home $RLY_CONF q node-state ibc0   

echo "Querying client states"
$RELAYER --home $RLY_CONF q client ibc0 ibconeclient 
$RELAYER --home $RLY_CONF q client ibc1 ibczeroclient

echo "Querying clients"
$RELAYER --home $RLY_CONF q clients ibc0 
$RELAYER --home $RLY_CONF q clients ibc1 