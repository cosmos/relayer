#!/bin/sh

set -o errexit -o nounset

CHAINID=$1
GENACCT=$2
PRIVPATH=$3

if [ -z "$1" ]; then
  echo "Need to input chain id..."
  exit 1
fi

if [ -z "$2" ]; then
  echo "Need to input genesis account address..."
  exit 1
fi

if [ -z "$3" ]; then
  echo "Need to input path of priv_validator_key json file"
  exit 1
fi

# Build genesis file incl account for passed address
coins="10000000000stake,100000000000samoleans"
osmosisd init --chain-id $CHAINID $CHAINID
osmosisd keys add validator --keyring-backend="test"
osmosisd add-genesis-account $(osmosisd keys show validator -a --keyring-backend="test") $coins
osmosisd add-genesis-account $GENACCT $coins
cp $PRIVPATH ~/.osmosisd/config/priv_validator_key.json
osmosisd gentx validator 5000000000stake --keyring-backend="test" --chain-id $CHAINID
osmosisd collect-gentxs

# Set proper defaults and change ports
sed -i 's#"tcp://127.0.0.1:26657"#"tcp://0.0.0.0:26657"#g' ~/.osmosisd/config/config.toml
sed -i 's/timeout_commit = "5s"/timeout_commit = "1s"/g' ~/.osmosisd/config/config.toml
sed -i 's/timeout_propose = "3s"/timeout_propose = "1s"/g' ~/.osmosisd/config/config.toml
sed -i 's/index_all_keys = false/index_all_keys = true/g' ~/.osmosisd/config/config.toml

# Start the osmosisd
osmosisd start --pruning=nothing

