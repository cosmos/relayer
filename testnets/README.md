# IBC Relayer Testnets

These are instructions on how to setup a node to run a basic IBC relayer testnet. Currently, coordination is happening in the [`ibc-testnet-alpha` group on telegram](https://t.me/joinchat/IYdbxRRFYIkj9FR99X3-BA).

### *14 October 2020 10:30 PST* - `stargate-4` testnet

This is the first relayer testnet that is using the `v0.40.0` (Stargate) code. Users are encouraged to submit their chain configuration (`rly chains show {my-chain} --json`) to the `testnets/stargate-4` folder in this repository to share their publiclly available node with other testnet participants.

- Gaia Version Info

```bash
$ gaiad version --long
name: gaia
server_name: gaiad
version: stargate-4
commit: 3a8b1b414004ccddfa255fd0cd1499bbf6659d71
build_tags: netgo,ledger
go: go version go1.15.2 linux/amd64
```

- Relayer Version Info

```bash
$ rly version
version: stargate-4
commit: 438815d47a857318199d556f4c1115f2fa6315a2
cosmos-sdk: v0.40.0-rc0
go: go1.15.2 linux/amd64
```

### Game Of Zones

Please see the [Game of Zones repository](https://github.com/cosmosdevs/GameOfZones) for details on competing in Game of Zones.

### *20 April 2020 17:00 PST* - `relayer-alpha-2` testnet

This is the second `relayer` testnet! I will be copying JSON files from the first testnet to the new folder as well as merging all the backlogged pull requests. Please make sure you are using the following version of `gaia` to ensure compatability.

- Gaia Version Info

```bash
$ gaiad version --long
name: gaia
server_name: gaiad
client_name: gaiad
version: 0.0.0-180-g50be36d
commit: 50be36de941b9410a4b06ec9ce4288b1529c4bd4
build_tags: netgo,ledger
go: go version go1.14 darwin/amd64
```

- Relayer Version Info

```bash
$ rly version
version: 0.3.0
commit: 781026cf46c6d144ab7fcd02d92817cc3d524903
cosmos-sdk: v0.34.4-0.20200423152229-f1fdde5d1b18
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
sudo apt install build-essential jq git -y

# Install latest go version https://golang.org/doc/install
wget https://dl.google.com/go/go1.15.2.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.15.2.linux-amd64.tar.gz

# Add $GOPATH, $GOBIN and both to $PATH
echo "" >> ~/.profile
echo 'export GOPATH=$HOME/go' >> ~/.profile
echo 'export GOBIN=$GOPATH/bin' >> ~/.profile
echo 'export PATH=$PATH:/usr/local/go/bin:$GOBIN' >> ~/.profile
echo "export GAIA=\$GOPATH/src/github.com/cosmos/gaia" >> ~/.profile
echo "export RELAYER=\$GOPATH/src/github.com/cosmos/relayer" >> ~/.profile
source ~/.profile

# Set these variables to different values that are specific to your chain
# Also, ensure they are set to make the commands below run properly
export DENOM=pylon
export CHAINID=pylonchain
export DOMAIN=shitcoincasinos.com
export RLYKEY=faucet
export GAIASHA=stargate-4
export ACCOUNT_PREFIX=cosmos

# Start by downloading and installing both gaia and the relayer
mkdir -p $(dirname $GAIA) && git clone https://github.com/cosmos/gaia $GAIA && cd $GAIA && git checkout $GAIASHA && make install
mkdir -p $(dirname $RELAYER) && git clone https://github.com/cosmos/relayer $RELAYER && cd $RELAYER && make install

# Verify gaia version matches that of the latest testnet above
gaia version --long

# Verify gaia version matches that of the latest testnet above
gaia version --long

# Now its time to configure both the relayer and gaia, start with the relayer
cd
rly config init
echo "{\"key\":\"$RLYKEY\",\"chain-id\":\"$CHAINID\",\"rpc-addr\":\"http://$DOMAIN:26657\",\"account-prefix\":\"$ACCOUNT_PREFIX\",\"gas\":200000,\"gas-prices\":\"0.025$DENOM\",\"default-denom\":\"$DENOM\",\"trusting-period\":\"330h\"}" > $CHAINID.json
# NOTE: you will want to save the content from this JSON file
rly chains add -f $CHAINID.json
rly keys add $CHAINID $RLYKEY

# Move on to configuring gaia
gaiad init --chain-id $CHAINID $CHAINID
# NOTE: ensure that the gaia rpc is open to all connections
sed -i 's#tcp://127.0.0.1:26657#tcp://0.0.0.0:26657#g' ~/.gaia/config/config.toml
sed -i "s/\"stake\"/\"$DENOM\"/g" ~/.gaia/config/genesis.json
sed -i 's/pruning = "syncable"/pruning = "nothing"/g' ~/.gaia/config/app.toml
gaiad keys add validator

# Now its time to construct the genesis file
gaiad add-genesis-account $(gaiad keys show validator -a) 100000000000$DENOM,10000000samoleans
gaiad add-genesis-account $(rly chains addr $CHAINID) 10000000000000$DENOM,10000000samoleans
gaiad gentx validator --amount 90000000000$DENOM --chain-id $CHAINID
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

### Relayer Setup

Once you have your server (you could deploy the relayer on a different machine as above server)

```bash
# install the relayer
export RELAYER=$GOPATH/src/github.com/cosmos/relayer
mkdir -p $(dirname $RELAYER) && git clone git@github.com:cosmos/relayer $RELAYER && cd $RELAYER
make install

# then to configure your local relayer to talk to your remote chain
# get the json from $CHAINID.json on your server
rly cfg init
rly ch add -f {{chain_id}}.json

# create a local rly key for the chain
rly keys add {{chain_id}} testkey

# confiure the chain to use that key by default
rly ch edit {{chain_id}} key testkey

# initialize the light client for {{chain_id}}
rly light init {{chain_id}} -f

# request funds from the faucet to test it
rly tst request {{chain_id}} testkey

# you should see a balance for the rly key now
rly q bal {{chain_id}}
```
Note that most of these instructions would also work directly on the 
server on which you deployed your gaia node on (not recommended though).

### Submit your {{chain_id}}.json to the relayer repo

Finally, open a pull request with the JSON required to configure a connection to your chain to the `testnets/relayer-alpha/{{chain_id}}.json`. You should be able to connect with other members of the relayer alpha testnet!


### Creating a connection

Once you have your chain configured on your relayer, follow these steps to send tokens between the chains:

```bash
# first ensure the chain is configured locally
cd $RELAYER

# do it either individually...
rly ch a -f testnets/relayer-alpha-2/pylonchain.json

# or add all the chain configurations for the testnet at once...
rly chains add-dir testnets/relayer-alpha-2/

# ensure the light clients are created locally...
rly light init {{src_chain_id}} -f 
rly l i {{dst_chain_id}} -f

# ensure each chain has its appropriate key...
rly keys add {{src_chain_id}}
rly k a {{dst_chain_id}}

# ensure you have funds on both chains...
# this adds tokens to your addresses from each chain's faucet
rly testnets request {{src_chain_id}}
rly tst req {{dst_chain_id}}

# either manually add a path by following the prompts...
rly paths add {{src_chain}} {{dst_chain_id}} {{path_name}}

# or generate one...
rly pth gen {{src_chain_id}} transfer {{dst_chain_id}} transfer {{path_name}}

# NOTE: path_name can be any string, but one convention is srcchain_dstchain

# or find all the existing paths...
# NOTE: this command is still under development, but will output
#  a number of existing paths between chains
rly pth find

# ensure that the path exists
rly tx link {{path_name}}

# then send some funds back and forth!
rly q bal {{src_chain_id}}
rly q bal {{dst_chain_id}}
rly tx transfer {{src_chain_id}} {{dst_chain_id}} {{amount}} true $(rly ch addr {{dst_chain_id}})
rly q bal {{src_chain_id}}
rly q bal {{dst_chain_id}}
rly tx xfer {{dst_chain_id}} {{src_chain_id}} {{amount}} false $(rly ch addr {{src_chain_id}})
rly q bal {{src_chain_id}}
rly q bal {{dst_chain_id}}
```
