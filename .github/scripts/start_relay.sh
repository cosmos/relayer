#!/bin/bash

SCRIPT=$(readlink -f $0)
SUBMODULE_DIR=$(dirname $SCRIPT)

GPG_FINGERPRINT="C787AB518A0C08B7AE1E1ADA2809A1A84E32159A"

ARCHWAY_CONTAINER='archway-node-1'

cd $SUBMODULE_DIR

# Correct path
sed -i "s|^CONTRACTS_DIR=.*|CONTRACTS_DIR=$PWD/IBC-Integration|" ./icon-ibc-setup/consts.sh
sed -i "s|^ICON_WALLET=.*|ICON_WALLET=$PWD/gochain-btp/data/godWallet.json|" ./icon-ibc-setup/consts.sh
sed -i "s|^ARCHWAY_WALLET=.*|ARCHWAY_WALLET=default|" ./icon-ibc-setup/consts.sh

# Import fd account
pass init $GPG_FINGERPRINT

echo "### Create default wallet"

wallet=$(archwayd keys add default --keyring-backend test | awk -F\: '/address/ {print $2}' | tr -d '[:space:]')
echo $wallet
archwayd keys list

echo "==> Starting icon node ..."
cd $SUBMODULE_DIR/gochain-btp
make ibc-ready

echo "==> Starting archway node ..."
cd ${SUBMODULE_DIR}/archway

sed -i '/^archwayd add-genesis-account.*/a archwayd add-genesis-account "'"$wallet"'" 1000000000stake --keyring-backend=test' contrib/localnet/localnet.sh
sed -i 's/latest/v0.4.0/' docker-compose.yaml
docker compose -f docker-compose.yaml up -d
sleep 60

echo "### Check archwayd start script content"
cat contrib/localnet/localnet.sh
docker ps

echo "### Check archwayd genesis file"
docker exec $ARCHWAY_CONTAINER cat /root/.archway/config/genesis.json

echo "### Check archwayd keys list on node"
docker exec $ARCHWAY_CONTAINER archwayd keys list

echo "### Check archwayd keys list on local"
archwayd keys list --keyring-backend os

echo "### Get fd wallet address"
fdwallet=$(docker exec $ARCHWAY_CONTAINER archwayd keys list --keyring-backend test | awk -F\: '/address/ {print $2}' | tr -d '[:space:]')

echo "default: $wallet"
echo "fd: $fdwallet"

echo "### Checking docker logs"
docker logs $ARCHWAY_CONTAINER
echo "### Query balance of account"
echo "default:"
archwayd query bank balances $wallet 
echo "fd:"
docker exec $ARCHWAY_CONTAINER archwayd query bank balances $fdwallet

cd $SUBMODULE_DIR/icon-ibc-setup

sed -i 's/ARCHWAY_NETWORK=localnet/ARCHWAY_NETWORK=docker/' consts.sh
mkdir -p ~/.relayer/config
echo "==> Setting up icon ..."
make icon
echo "==> Setting up archway ..."
make archway
echo "### Updating config ..."
make config

cat ~/.relayer/config/config.yaml

echo -e "\nCopy default key to relayer keyring ======"
mkdir -p /home/runner/.relayer/keys/localnet/keyring-test
cp ~/.archway/keyring-test/default.info ~/.relayer/keys/localnet/keyring-test/default.info


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
echo "==> Starting link..."
rly tx link icon-archway --client-tp=10000m --src-port mock --dst-port mock -d
# Enable when debug is required
# rly tx link icon-archway --client-tp=10000m --src-port mock --dst-port mock --order=ordered -d
# for txhash in $(cat log.txt | grep 'Submitted transaction" provider_type=archway chain_id=localnet txHash=' | awk -F\= '{print $NF}')
# do
  # echo -e "\n+++ Checking $txhash ...\n"
  # archwayd query tx $txhash
# done
echo
echo
docker ps
echo "### Checking relay config"
cat ~/.relayer/config/config.yaml
echo "==> Starting relayer..."
rly start icon-archway & sleep 60s; echo "* Stopping relay ..."; kill $!

