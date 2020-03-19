# ibc-testnet-instructions

These are instructions on how to setup a node to run a basic IBC relayer testnet. Currently, coordination is happening in the [`ibc-testnet-alpha` group on telegram](https://t.me/joinchat/IYdbxRRFYIkj9FR99X3-BA).

## Server Side
In order to run a node in this testnet, you need to run it from a server of some sort. This guide will not specify a specific cloud provider. The instructions assume you have SSH access w/ `sudo` to the server. While setting up the server, ensure that ports `:8000` and `:26657` are open.

> NOTE: Setup DNS for your IP address

### Server Setup

```bash
# Update the system and install dependancies
sudo apt update
sudo apt install build-essential jq -y

# Install latest go version https://golang.org/doc/install
wget https://dl.google.com/go/go1.14.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.14.linux-amd64.tar.gz

# Add $GOPATH, $GOBIN and both to $PATH
echo "" >> ~/.profile
echo 'export GOPATH=$HOME/go' >> ~/.profile
echo 'export GOBIN=$GOPATH/bin' >> ~/.profile
echo 'export PATH=$PATH:/usr/local/go/bin:$GOBIN' >> ~/.profile
source ~/.profile

# Set these variables to different values that are specific to your chain
# They will make the commands below easier
export DENOM=pylon
export CHAINID=pylonchain
export GAIA=$GOPATH/src/github.com/cosmos/gaia
export RELAYER=$GOPATH/src/github.com/iqlusioninc/relayer
export DOMAIN=mydomain.com
export RLYKEY=faucet

# Start by downloading and installing both gaia and the relayer
mkdir -p $(dirname $GAIA) && git clone https://github.com/cosmos/gaia $GAIA && cd $GAIA && make install
mkdir -p $(dirname $RELAYER) && git clone https://github.com/iqlusioninc/relayer $RELAYER && cd $RELAYER && make install

# Now its time to configure both the relayer and gaia, start with the relayer
cd
relayer config init
echo "{\"key\":\"$RLYKEY\",\"chain-id\":\"$CHAINID\",\"rpc-addr\":\"http://$DOMAIN:26657\",\"account-prefix\":\"cosmos\",\"gas\":200000,\"gas-prices\":\"0.025$DENOM\",\"default-denom\":\"$DENOM\",\"trusting-period\":\"330h\"}" > $CHAINID.json
relayer chains add -f $CHAINID.json
relayer keys add $CHAINID $RLYKEY
# NOTE: you will want to save this JSON file

# Move on to configuring gaia
gaiad init --chain-id $CHAINID $CHAINID
# NOTE: ensure that the gaia rpc is open to all connections
sed -i 's#tcp://127.0.0.1:26657#tcp://0.0.0.0:26657#g' ~/.gaiad/config/config.toml
sed -i "s/stake/$DENOM/g" ~/.gaiad/config/genesis.json
gaiacli keys add validator

# Now its time to construct the genesis file
gaiad add-genesis-account $(gaiacli keys show validator -a) 100000000000$DENOM,10000000samoleans
gaiad add-genesis-account $(relayer keys show $CHAINID $RLYKEY -a) 10000000000000$DENOM,10000000samoleans
gaiad gentx --name validator --amount 90000000000$DENOM
gaiad collect-gentxs

# Setup the service definitions
relayer testnets gaia-service $USER > gaiad.service
relayer testnets faucet-service $USER $CHAINID faucet 100000$DENOM > faucet.service
sudo mv gaiad.service /etc/systemd/system/gaiad.service
sudo mv faucet.service /etc/systemd/system/faucet.service
sudo systemctl daemon-reload
sudo systemctl start gaiad
sudo systemctl start faucet

# Server _should_ be ready to go!
```

### on your local
TODO :point_down:

```bash
# configure the relayer to 
```
- use the relayer to generate a chain config (relayer chains add) that points to your chain
- test the connection to your chain
- open a PR with that json (relayer chains show [chain-id] --json) to ./testnets/relayer-alpha/[chain-id].json
