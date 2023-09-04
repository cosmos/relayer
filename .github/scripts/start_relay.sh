#!/bin/bash

SCRIPT=$(readlink -f $0)
SUBMODULE_DIR=$(dirname $SCRIPT)

GPG_FINGERPRINT="C787AB518A0C08B7AE1E1ADA2809A1A84E32159A"

ARCHWAY_CONTAINER='archway-node-1'

cd $SUBMODULE_DIR

echo $PWD

# Correct path
search_strings=(
    "export IBC_RELAY="
    "export IBC_INTEGRATION="
    "export ICON_DOCKER_PATH="
    "export WASM_DOCKER_PATH="
)

replacement="\$GITHUB_WORKSPACE\/.github\/scripts"
for search_string in "${search_strings[@]}"; do
    sed -i "/$search_string/ s/\$HOME/$replacement/g" ./icon-ibc/const.sh
done

sed -i 's/export WASM_WALLET=godWallet/export WASM_WALLET=default/' ./icon-ibc/const.sh
sed -i 's/\(export WASM_EXTRA=\)"\([^"]*\)"\(.*\)/\1"--keyring-backend test"\3/' ./icon-ibc/const.sh

# Use below lines until there is version fix on wasm file
sed -i 's/\.wasm/_\.wasm/g' ./icon-ibc/const.sh 
sed -i 's/cw_xcall_.wasm/cw_xcall_0.1.0.wasm/g' ./icon-ibc/const.sh
sed -i 's/cw_mock_dapp_multi_.wasm/cw_mock_dapp_multi_0.1.0.wasm/g' ./icon-ibc/const.sh

cat ./icon-ibc/const.sh

# sed -i 's/ARCHWAY_NETWORK=localnet/ARCHWAY_NETWORK=docker/' consts.sh
mkdir -p ~/.relayer/config
mkdir -p ~/keystore
cp ./gochain-btp/data/godWallet.json ~/keystore/godWallet.json
# Import fd account
pass init $GPG_FINGERPRINT

echo "### Create default wallet"
wallet=$(archwayd keys add default --keyring-backend test | awk -F\: '/address/ {print $2}' | tr -d '[:space:]')
echo $wallet

relay_wallet=$(archwayd keys add relayWallet --keyring-backend test | awk -F\: '/address/ {print $2}' | tr -d '[:space:]')

archwayd keys list --keyring-backend test




cd ${SUBMODULE_DIR}/archway

echo "## Before update .."
cat contrib/localnet/localnet.sh
# sed -i '/^archwayd add-genesis-account.*/a archwayd add-genesis-account "'"$wallet"'" 100000000000stake --keyring-backend=test' contrib/localnet/localnet.sh

sed -i '/archwayd add-genesis-account "\$addr" 1000000000000stake --keyring-backend=test/{p; a\
archwayd add-genesis-account "'"$wallet"'" 100000000000stake --keyring-backend=test\
archwayd add-genesis-account "'"$relay_wallet"'" 100000000000stake --keyring-backend=test
}' contrib/localnet/localnet.sh

awk '!seen[$0]++' contrib/localnet/localnet.sh > contrib/localnet/localnet_new.sh
mv contrib/localnet/localnet_new.sh contrib/localnet/localnet.sh
sed -i 's/latest/v0.4.0/' docker-compose.yaml

echo "### Check archwayd keys list on local"
archwayd keys list --keyring-backend os

echo "### Check archwayd start script content"
cat contrib/localnet/localnet.sh


cd $SUBMODULE_DIR/icon-ibc




# make nodes
bash -x ./nodes.sh start-all

sleep 30
echo "### Get fd wallet address"
fdwallet=$(docker exec $ARCHWAY_CONTAINER archwayd keys list --keyring-backend test | awk -F\: '/address/ {print $2}' | tr -d '[:space:]')

echo "### Check archwayd genesis file"
docker exec $ARCHWAY_CONTAINER cat /root/.archway/config/genesis.json

echo "### Check archwayd keys list on node"
docker exec $ARCHWAY_CONTAINER archwayd keys list

echo "### Check archwayd keys list on local"
archwayd keys list --keyring-backend os
archwayd keys list --keyring-backend test

echo "default:"
archwayd query bank balances $wallet 
echo "fd:"
docker exec $ARCHWAY_CONTAINER archwayd query bank balances $fdwallet


echo "default: $wallet"
echo "fd: $fdwallet"

echo "### Checking docker logs"
docker logs $ARCHWAY_CONTAINER
echo "### Query balance of account"
echo "default:"
archwayd query bank balances $wallet 
echo "fd:"
docker exec $ARCHWAY_CONTAINER archwayd query bank balances $fdwallet

echo -e "\nCopy default key to relayer keyring ======"
mkdir -p /home/runner/.relayer/keys/localnet/keyring-test
cp ~/.archway/keyring-test/default.info ~/.relayer/keys/localnet/keyring-test/default.info
cp ~/.archway/keyring-test/relayWallet.info ~/.relayer/keys/localnet/keyring-test/relayWallet.info
ls ~/.archway/keyring-test/*
ls ~/.relayer/keys/localnet/keyring-test/*

# make contracts
echo 
echo "++++ Setting up icon"
bash -x ./icon.sh --setup

echo 
echo "++++ Setting up archway"
bash -x ./wasm.sh --setup
echo
echo "++++ Deploying xcall on icon"
bash -x ./icon.sh --deploy-dapp
echo
echo "++++ Deploying xcall on archway"
bash -x ./wasm.sh --deploy-dapp
bash -x ./cfg.sh

## Check Score Address
echo "Checking Score Address..."
grep . $SUBMODULE_DIR/icon-ibc/env/archway/.*
grep . $SUBMODULE_DIR/icon-ibc/env/icon/.*

echo
echo "++++ Starting handshake ..."
# make handshake
rly tx clients icon-archway --client-tp "10000000m"
rly tx conn icon-archway

echo
echo "+++ Icon - Configure Connection"
bash -x ./icon.sh -c
echo
echo "+++ Archway - Configure Connection"
bash -x ./wasm.sh -c
echo
echo "+++ Create Channel"
rly tx chan icon-archway --src-port=xcall --dst-port=xcall

echo "Checking containers..."
docker ps -a
# cat ~/.relayer/config/config.yaml




echo "### all archwayd keys:"
archwayd keys list
echo "### keyring: os"
archwayd keys list --keyring-backend os
echo "### keyring: test"
archwayd keys list --keyring-backend test

echo "### Checking keys inside archway docker node:"
docker exec $ARCHWAY_CONTAINER archwayd keys list --keyring-backend os
docker exec $ARCHWAY_CONTAINER archwayd keys list --keyring-backend test


echo "+++++++++++++++++++++"
echo
echo
docker ps
echo "### Checking relay config"
find ./ -type f -name config.yaml
cat ~/.relayer/config/config.yaml
echo "==> Starting relayer..."
rly start icon-archway & sleep 60s; echo "* Stopping relay ..."; kill $!

