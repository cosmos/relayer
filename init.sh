#!/usr/bin/env bash

rm -rf ./data
mkdir -p ./data

killall ibc0d ibc1d


cd ./data || exit 1

starport app github.com/utx0/ibc0 --sdk-version stargate
starport app github.com/utx0/ibc1 --sdk-version stargate

starport build -p ./ibc0
which ibc0d

cd ../

echo setup chains
pwd
./scripts/one-ibc-chain ibc0 ./data 26657 26656 6060 9090
./scripts/one-ibc-chain ibc1 ./data 26557 26556 6061 9091

make install

which rly

rm -rf ~/.relayer
rly config init

cat ~/.relayer/config/config.yaml

rly cfg add-dir configs/demo/
cat ~/.relayer/config/config.yaml

# Now, add the key seeds from each chain to the relayer to give it funds to work with
rly keys restore ibc0 testkey "$(jq -r '.mnemonic' data/ibc0/key_seed.json)"
rly keys restore ibc1 testkey "$(jq -r '.mnemonic' data/ibc1/key_seed.json)"

# Then its time to initialize the relayer's light clients for each chain
# All data moving forward is validated by these light clients.
rly light init ibc0 -f
rly light init ibc1 -f

# At this point the relayer --home directory is ready for normal operations between
# ibc0 and ibc1. Looking at the folder structure of the relayer at this point is helpful
tree ~/.relayer

# Now you can connect the two chains with one command:
rly tx link demo -d -o 3s
