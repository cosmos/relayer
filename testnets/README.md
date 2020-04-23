# IBC Relayer Testnets

These are instructions on how to setup a node to run a basic IBC relayer testnet. Currently, coordination is happening in the [`ibc-testnet-alpha` group on telegram](https://t.me/joinchat/IYdbxRRFYIkj9FR99X3-BA).

### *20 April 2020 17:00 PST* - `relayer-alpha-2` testnet

This is the second `relayer` testnet! I will be copying JSON files from the first testnet to the new folder as well as merging all the backlogged pull requests. Please make sure you are using the following version of `gaia` to ensure compatability.

- Gaia Version Info

```bash
$ gaiad version --long
name: gaia
server_name: gaiad
client_name: gaiacli
version: 0.0.0-180-g50be36d
commit: 50be36de941b9410a4b06ec9ce4288b1529c4bd4
build_tags: netgo,ledger
go: go version go1.14 darwin/amd64
```

- Relayer Version Info

```bash
$ rly version
version: 0.2.0
commit: 49e0d7316410b480236e280b0d94731adc807383
cosmos-sdk: v0.34.4-0.20200419154345-84774907316c
go: go1.14 darwin/amd64
```

### *19 March 2020 17:00 PST* - `relayer-alpha` testnet

This is the first `relayer` testnet! Please submit your JSON files for this testnet to `./testnets/relayer-alpha/{{chain_id}}.json`.

## Testnet Node Setup
In order to run a node in this testnet, you need to run it from a server of some sort. This guide is agnostic to a specific cloud provider. The instructions assume you have SSH access w/ `sudo` on the server. While setting up the server, ensure that ports `:8000` (`faucet`) and `:26657` (`gaiad`) are open and accessable.

> NOTE: You can and should (:wink:) setup a DNS A record pointing to your server to make the URL human readable instead of an IP address. Many services make this easy. I'm using https://domains.google.com.

### Server Setup

The following instructions have been tested and are working from a default install of Ubuntu on a shared CPU instance w/ 2GB RAM.

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
echo "export GAIA=\$GOPATH/src/github.com/cosmos/gaia" >> ~/.profile
echo "export RELAYER=\$GOPATH/src/github.com/iqlusioninc/relayer" >> ~/.profile
source ~/.profile

# Set these variables to different values that are specific to your chain
# Also, ensure they are set to make the commands below run properly
export DENOM=pylon
export CHAINID=pylonchain
export DOMAIN=shitcoincasinos.com
export RLYKEY=faucet
export GAIASHA=50be36d

# Start by downloading and installing both gaia and the relayer
mkdir -p $(dirname $GAIA) && git clone https://github.com/cosmos/gaia $GAIA && cd $GAIA && git checkout $GAIASHA && make install
mkdir -p $(dirname $RELAYER) && git clone https://github.com/iqlusioninc/relayer $RELAYER && cd $RELAYER && make install

# Now its time to configure both the relayer and gaia, start with the relayer
cd
rly config init
echo "{\"key\":\"$RLYKEY\",\"chain-id\":\"$CHAINID\",\"rpc-addr\":\"http://$DOMAIN:26657\",\"account-prefix\":\"cosmos\",\"gas\":200000,\"gas-prices\":\"0.025$DENOM\",\"default-denom\":\"$DENOM\",\"trusting-period\":\"330h\"}" > $CHAINID.json
# NOTE: you will want to save the content from this JSON file
rly chains add -f $CHAINID.json
rly keys add $CHAINID $RLYKEY

# Move on to configuring gaia
gaiad init --chain-id $CHAINID $CHAINID
# NOTE: ensure that the gaia rpc is open to all connections
sed -i 's#tcp://127.0.0.1:26657#tcp://0.0.0.0:26657#g' ~/.gaiad/config/config.toml
sed -i "s/stake/$DENOM/g" ~/.gaiad/config/genesis.json
sed -i 's/pruning = "syncable"/pruning = "nothing"/g' ~/.gaiad/config/app.toml
gaiacli keys add validator

# Now its time to construct the genesis file
gaiad add-genesis-account $(gaiacli keys show validator -a) 100000000000$DENOM,10000000samoleans
gaiad add-genesis-account $(rly chains addr $CHAINID) 10000000000000$DENOM,10000000samoleans
gaiad gentx --name validator --amount 90000000000$DENOM
gaiad collect-gentxs

# Setup the service definitions
rly dev gaia $USER $HOME > gaiad.service
rly dev faucet $USER $HOME $CHAINID $RLYKEY 100000$DENOM > faucet.service
sudo mv gaiad.service /etc/systemd/system/gaiad.service
sudo mv faucet.service /etc/systemd/system/faucet.service
sudo systemctl daemon-reload
sudo systemctl start gaiad
sudo systemctl start faucet

# Server _should_ be ready to go!
# Be sure you have the text from ~/$CHAINID.json for the next step
```

### Local Setup
Once you have your server 

```bash
# install the relayer
export RELAYER=$GOPATH/src/github.com/iqlusioninc/relayer
mkdir -p $(dirname $RELAYER) && git clone git@github.com:iqlusioninc/relayer $RELAYER && cd $RELAYER
make install

# then to configure your local relayer to talk to your remote chain
# get the json from $CHAINID.json on your server
rly cfg init
rly ch add -f {{chain_id}}.json

# create a local rly key for the chain
rly keys add {{chain_id}} testkey

# confiure the chain to use that key by default
rly ch edit {{chain_id}} key testkey

# initialize the lite client for {{chain_id}}
rly lite init {{chain_id}} -f

# request funds from the faucet to test it
rly tst request {{chain_id}} testkey

# you should see a balance for the rly key now
rly q bal {{chain_id}}
```

### Submit your {{chain_id}}.json to the relayer repo

Finally, open a pull request with the JSON required to configure a connection to your chain to the `testnets/relayer-alpha/{{chain_id}}.json`. You should be able to connect with other members of the relayer alpha testnet!


### Creating a connection

Once you have your chain configured on your relayer, follow these steps to send tokens between the chains:

```bash
# first ensure the chain is configured locally
# do it either individually...
rly ch a -f testnets/relayer-alpha/pylonchain.json

# or add all the chain configurations for the testnet at once...
rly chains add-dir tesetnets/relayer-alpha/

# ensure the lite clients are created locally...
rly lite init {{src_chain_id}} -f 
rly l i {{dst_chain_id}} -f

# ensure each chain has its appropriate key...
rly keys add {{src_chain_id}}
rly k a {{dst_chain_id}}

# ensure you have funds on both chains...
rly testnets request {{src_chain_id}}
rly tst req {{dst_chain_id}}

# either manually add a path by following the prompts...
rly paths add {{src_chain}} {{dst_chain_id}} {{path_name}}

# or generate one...
rly pth gen {{src_chain_id}} {{src_port}} {{dst_chain_id}} {{dst_port}} {{path_name}}

# or find all the existing paths...
# NOTE: this command is still under development, but will output
#  a number of existing paths between chains
rly pth find

# ensure that the path exists
rly tx link {{src_chain_id}} {{dst_chain_id}}

# then send some funds back and forth!
rly q bal {{src_chain_id}}
rly q bal {{dst_chain_id}}
rly tx transfer {{src_chain_id}} {{dst_chain_id}} {{amount}} true $(rly ch addr {{dst_chain_id}})
rly q bal {{src_chain_id}}
rly q bal {{dst_chain_id}}
rly tx xfer {{ds_chain_id}} {{src_chain_id}} {{amount}} false $(rly ch addr {{src_chain_id}})
rly q bal {{src_chain_id}}
rly q bal {{dst_chain_id}}
```
