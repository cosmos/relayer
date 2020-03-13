#!/bin/bash

# Ensure gopath is set and go is installed
if [[ ! -d $GOPATH ]] || [[ ! -d $GOBIN ]] || [[ ! -x "$(which go)" ]]; then
  echo "Your \$GOPATH is not set or go is not installed,"
  echo "ensure you have a working installation of go before trying again..."
  echo "https://golang.org/doc/install"
  exit 1
fi

GAIA_REPO="$GOPATH/src/github.com/cosmos/gaia"
RELAYER_DIR="$(pwd)"
GAIA_BRANCH=ibc-alpha
GAIA_DATA="$RELAYER_DIR/data"

# ARGS: 
# $1 -> local || remote, defaults to remote

# Ensure user understands what will be deleted
if [[ -d $GAIA_DATA ]] && [[ ! "$2" == "skip" ]]; then
  read -p "$0 will delete $GAIA_DATA folder. Do you wish to continue? (y/n): " -n 1 -r
  echo 
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
      exit 1
  fi
fi

rm -rf $GAIA_DATA &> /dev/null
killall gaiad &> /dev/null

set -e

echo "Building github.com/cosmos/gaia@$GAIA_BRANCH..."

if [[ -d $GAIA_REPO ]]; then
  cd $GAIA_REPO

  # remote build syncs with remote then builds
  if [[ "$1" == "local" ]]; then
    make install &> /dev/null
  else
    if [[ ! -n $(git status -s) ]]; then
      # sync with remote $GAIA_BRANCH
      git fetch --all &> /dev/null
      git checkout $GAIA_BRANCH &> /dev/null
      git pull origin $GAIA_BRANCH &> /dev/null

      # install
      make install &> /dev/null

      # ensure that built binary has the same version as the repo
      if [[ ! "$(gaiad version --long 2>&1 | grep "commit:" | sed 's/commit: //g')" == "$(git rev-parse HEAD)" ]]; then
        echo "built version of gaiad commit doesn't match "
        exit 1
      fi 
    else
      echo "uncommited changes in $GAIA_REPO, please commit or stash before building"
      exit 1
    fi
    
  fi 
else 
  echo "$GAIA_REPO doesn't exist, and you may not have have the gaia repo locally,"
  echo "if you want to download gaia to your \$GOPATH try running the following command:"
  echo "mkdir -p $(dirname $GAIA_REPO) && git clone git@github.com:cosmos/gaia $GAIA_REPO"
fi

chainid0=ibc0
chainid1=ibc1

echo "Generating gaia configurations..."
mkdir -p $GAIA_DATA && cd $GAIA_DATA
echo -e "\n" | gaiad testnet -o $chainid0 --v 1 --chain-id $chainid0 --node-dir-prefix n --keyring-backend test &> /dev/null
echo -e "\n" | gaiad testnet -o $chainid1 --v 1 --chain-id $chainid1 --node-dir-prefix n --keyring-backend test &> /dev/null

cfgpth="n0/gaiad/config/config.toml"
if [ "$(uname)" = "Linux" ]; then
  # TODO: Just index some specified tags
  # sed -i 's/index_keys = ""/index_keys = "tx.height,tx.hash"'
  sed -i 's/"leveldb"/"goleveldb"/g' $chainid0/$cfgpth
  sed -i 's/"leveldb"/"goleveldb"/g' $chainid1/$cfgpth
  sed -i 's#"tcp://0.0.0.0:26656"#"tcp://0.0.0.0:26556"#g' $chainid1/$cfgpth
  sed -i 's#"tcp://0.0.0.0:26657"#"tcp://0.0.0.0:26557"#g' $chainid1/$cfgpth
  sed -i 's#"localhost:6060"#"localhost:6061"#g' $chainid1/$cfgpth
  sed -i 's#"tcp://127.0.0.1:26658"#"tcp://127.0.0.1:26558"#g' $chainid1/$cfgpth
else
  # TODO: Just index some specified tags
  # sed -i 's/index_keys = ""/index_keys = "tx.height,tx.hash"'
  sed -i '' 's/"leveldb"/"goleveldb"/g' $chainid0/$cfgpth
  sed -i '' 's/"leveldb"/"goleveldb"/g' $chainid1/$cfgpth
  sed -i '' 's#"tcp://0.0.0.0:26656"#"tcp://0.0.0.0:26556"#g' $chainid1/$cfgpth
  sed -i '' 's#"tcp://0.0.0.0:26657"#"tcp://0.0.0.0:26557"#g' $chainid1/$cfgpth
  sed -i '' 's#"localhost:6060"#"localhost:6061"#g' $chainid1/$cfgpth
  sed -i '' 's#"tcp://127.0.0.1:26658"#"tcp://127.0.0.1:26558"#g' $chainid1/$cfgpth
fi

gclpth="n0/gaiacli/"
gaiacli config --home $chainid0/$gclpth chain-id $chainid0 &> /dev/null
gaiacli config --home $chainid1/$gclpth chain-id $chainid1 &> /dev/null
gaiacli config --home $chainid0/$gclpth output json &> /dev/null
gaiacli config --home $chainid1/$gclpth output json &> /dev/null
gaiacli config --home $chainid0/$gclpth node http://localhost:26657 &> /dev/null
gaiacli config --home $chainid1/$gclpth node http://localhost:26557 &> /dev/null

echo "Starting Gaiad instances..."
gaiad --home $GAIA_DATA/$chainid0/n0/gaiad start --pruning=nothing > $chainid0.log 2>&1 &
gaiad --home $GAIA_DATA/$chainid1/n0/gaiad start --pruning=nothing > $chainid1.log 2>&1 & 
