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
nohup $GAIAD --home ibc0/n0/gaiad start --pruning=nothing > ibc0.log &
nohup $GAIAD --home ibc1/n0/gaiad start --pruning=nothing > ibc1.log &

echo "Adding gaiacli keys to the relayer"
$RELAYER --home $RLY_CONF keys restore ibc0 testkey "$(jq -r '.secret' ibc0/n0/gaiacli/key_seed.json)" -a
$RELAYER --home $RLY_CONF keys restore ibc1 testkey "$(jq -r '.secret' ibc1/n0/gaiacli/key_seed.json)" -a

echo "Wait for first block"
sleep 12

# VARIABLES FOR CHAINS
c0=ibc0
c1=ibc1

echo "Check account balances"
$RELAYER --home $RLY_CONF q account $c0
$RELAYER --home $RLY_CONF q account $c1

echo "Initialize lite clients"
$RELAYER --home $RLY_CONF lite init $c0 -f
$RELAYER --home $RLY_CONF lite init $c1 -f

echo "Create clients"
$RELAYER --home $RLY_CONF tx clients $c0 $c1

echo "Query headers"
$RELAYER --home $RLY_CONF q header $c0   
$RELAYER --home $RLY_CONF q header $c0

echo "Query node-state"
$RELAYER --home $RLY_CONF q node-state $c0   
$RELAYER --home $RLY_CONF q node-state $c0   

echo "Querying clients"
$RELAYER --home $RLY_CONF q clients $c0
$RELAYER --home $RLY_CONF q clients $c1

echo "Creating connection..."
$RELAYER --home $RLY_CONF tx connection $c0 $c1

echo "Creating channel..."
$RELAYER --home $RLY_CONF tx channel $c0 $c1
