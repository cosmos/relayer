#!/bin/zsh

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
echo -e "\n" | gaiad testnet -o ibc0 --v 1 --chain-id ibc0 --node-dir-prefix n &> /dev/null
echo -e "\n" | gaiad testnet -o ibc1 --v 1 --chain-id ibc1 --node-dir-prefix n &> /dev/null

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
nohup gaiad --home ibc0/n0/gaiad --log_level="*:debug" start > ibc0.log &
nohup gaiad --home ibc1/n0/gaiad --log_level="*:debug" start > ibc1.log &

echo 
echo "Key Seeds for importing into gaiacli if necessary:"
SEED0=$(jq -r '.secret' ibc0/n0/gaiacli/key_seed.json)
SEED1=$(jq -r '.secret' ibc1/n0/gaiacli/key_seed.json)
echo "  ibc0 -> $SEED0"
echo "  ibc1 -> $SEED1"
echo
echo "Set the following env to make working with the running chains easier:"
echo 
echo "export RLY=$RLY_CONF"
echo "export GAIA=$GAIA_CONF"
echo 
echo "NOTE: Below are account addresses for each chain. They are also validator addresses:"
echo "  ibc0 address: $(relayer --home $RLY_CONF keys restore ibc0 testkey "$SEED0" -a)"
echo "  ibc1 address: $(relayer --home $RLY_CONF keys restore ibc1 testkey "$SEED1" -a)"
echo 
echo "Example commands:"
echo "  balance ibc0: gaiacli --home \$GAIA/ibc-testnets/ibc0/n0/gaiacli q account \$(relayer --home \$RLY keys show ibc0 testkey)"
echo "  balance ibc1: gaiacli --home \$GAIA/ibc-testnets/ibc1/n0/gaiacli q account \$(relayer --home \$RLY keys show ibc1 testkey)"