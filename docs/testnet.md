# ibc-testnet-instructions

These are instructions on how to setup a node to run a basic IBC relayer testnet. Currently, coordination is happening in the [`ibc-testnet-alpha` group on telegram](https://t.me/joinchat/IYdbxRRFYIkj9FR99X3-BA).

### Server Side
In order to run a node in this testnet, you need to run it from a server of some sort. This guide will not specify a specific cloud provider. The instructions assume you have SSH access w/ `sudo` to the server. While setting up the server, ensure that ports `:8000` and `:26657` are open.

> NOTE: Setup DNS for your IP address

#### Install Go

Instructions: https://golang.org/doc/install
Be sure to configure your gopath

```
sudo apt install build-essential -y
wget https://dl.google.com/go/go1.14.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.14.linux-amd64.tar.gz
echo "" >> ~/.profile
echo 'export GOPATH=$HOME/go' >> ~/.profile
echo 'export GOBIN=$GOPATH/bin' >> ~/.profile
echo 'export PATH=$PATH:/usr/local/go/bin:$GOBIN' >> ~/.profile
source ~/.profile
```

#### Download the code and install

```bash
# Set some env to make running the future commands easier
export DENOM=pylon
export CHAINID=pylonchain
export GAIA=$GOPATH/src/github.com/cosmos/gaia
export RELAYER=$GOPATH/src/github.com/iqlusioninc/relayer
mkdir -p $(dirname $GAIA)
mkdir -p $(dirname $RELAYER)
git clone https://github.com/cosmos/gaia $GAIA
git clone https://github.com/iqlusioninc/relayer $RELAYER
cd $GAIA
make install
cd $RELAYER
make install
cd
relayer config init
# NOTE: interactive command
relayer chains add
gaiad init --chain-id $CHAINID {{moniker}}
sed -i "s/stake/$DENOM/g" ~/.gaiad/config/genesis.json
gaiacli keys add validator
gaiad add-genesis-account $(gaiacli keys show validator -a) 100000000000$DENOM,10000000samoleans
gaiad add-genesis-account $(relayer keys add $CHAINID faucet -a) 10000000000000$DENOM,10000000samoleans
gaiad gentx --name validator --amount 100000000000$DENOM
gaia collect-gentxs
sudo relayer testnets gaia-service $USER > /etc/systemd/system/gaiad.service
sudo relayer testnets faucet-service $USER $CHAINID faucet 100000$DENOM > /etc/systemd/system/faucet.service
sudo systemctl daemon-reload
sudo systemctl start gaiad
sudo systemctl start faucet
```

### on your local
- install the relayer
- use the relayer to generate a chain config (relayer chains add) that points to your chain
- test the connection to your chain
- open a PR with that json (relayer chains show [chain-id] --json) to ./testnets/relayer-alpha/[chain-id].json
