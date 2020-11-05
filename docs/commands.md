# Relayer commands list

## rly

This application relays data between configured IBC enabled chains

### Synopsis

The relayer has commands for:
  1. Configuration of the Chains and Paths that the relayer with transfer packets over
  2. Management of keys and light clients on the local machine that will be used to sign and verify txs
  3. Query and transaction functionality for IBC
  4. A responsive relaying application that listens on a path
  5. Commands to assist with development, testnets, and stuff

NOTE: Most of the commands have aliases that make typing them much quicker (i.e. 'rly tx', 'rly q', etc...). Almost all commands have `--help` flag to show usage information.

### Options

```
      --config string   set config file (default "config.yaml")
  -d, --debug           debug output
  -h, --help            help for the current command
      --home string     set home directory (default "/Users/vsh/.relayer")
```


## Table of Contents

- [Relayer commands list](#relayer-commands-list)
  - [rly chains](#rly-chains)
    - [rly chains add](#rly-chains-add)
    - [rly chains add-dir](#rly-chains-add-dir)
    - [rly chains address](#rly-chains-address)
    - [rly chains delete](#rly-chains-delete)
    - [rly chains edit](#rly-chains-edit)
    - [rly chains list](#rly-chains-list)
    - [rly chains show](#rly-chains-show)
  - [rly config](#rly-config)
    - [rly config add-dir](#rly-config-add-dir)
    - [rly config init](#rly-config-init)
    - [rly config show](#rly-config-show)
  - [rly development](#rly-development)
    - [rly development faucet](#rly-development-faucet)
    - [rly development gaia](#rly-development-gaia)
    - [rly development genesis](#rly-development-genesis)
    - [rly development listen](#rly-development-listen)
    - [rly development relayer](#rly-development-relayer)
  - [rly keys](#rly-keys)
    - [rly keys add](#rly-keys-add)
    - [rly keys delete](#rly-keys-delete)
    - [rly keys export](#rly-keys-export)
    - [rly keys list](#rly-keys-list)
    - [rly keys restore](#rly-keys-restore)
    - [rly keys show](#rly-keys-show)
  - [rly light](#rly-light)
    - [rly light delete](#rly-light-delete)
    - [rly light header](#rly-light-header)
    - [rly light init](#rly-light-init)
    - [rly light update](#rly-light-update)
  - [rly paths](#rly-paths)
    - [rly paths add](#rly-paths-add)
    - [rly paths delete](#rly-paths-delete)
    - [rly paths find](#rly-paths-find)
    - [rly paths generate](#rly-paths-generate)
    - [rly paths list](#rly-paths-list)
    - [rly paths show](#rly-paths-show)
  - [rly query](#rly-query)
    - [rly query account](#rly-query-account)
    - [rly query balance](#rly-query-balance)
    - [rly query channel](#rly-query-channel)
    - [rly query channels](#rly-query-channels)
    - [rly query client-connections](#rly-query-client-connections)
    - [rly query client](#rly-query-client)
    - [rly query clients](#rly-query-clients)
    - [rly query connection-channels](#rly-query-connection-channels)
    - [rly query connection](#rly-query-connection)
    - [rly query connections](#rly-query-connections)
    - [rly query full-path](#rly-query-full-path)
    - [rly query header](#rly-query-header)
    - [rly query node-state](#rly-query-node-state)
    - [rly query packet-ack](#rly-query-packet-ack)
    - [rly query packet-commit](#rly-query-packet-commit)
    - [rly query seq-send](#rly-query-seq-send)
    - [rly query tx](#rly-query-tx)
    - [rly query txs](#rly-query-txs)
    - [rly query unrelayed](#rly-query-unrelayed)
  - [rly start](#rly-start)
  - [rly testnets](#rly-testnets)
    - [rly testnets faucet](#rly-testnets-faucet)
    - [rly testnets request](#rly-testnets-request)
  - [rly transact](#rly-transact)
    - [rly transact channel-close](#rly-transact-channel-close)
    - [rly transact channel](#rly-transact-channel)
    - [rly transact clients](#rly-transact-clients)
    - [rly transact connection](#rly-transact-connection)
    - [rly transact link](#rly-transact-link)
    - [rly transact raw](#rly-transact-raw)
      - [rly transact raw chan-ack](#rly-transact-raw-chan-ack)
      - [rly transact raw chan-close-confirm](#rly-transact-raw-chan-close-confirm)
      - [rly transact raw chan-close-init](#rly-transact-raw-chan-close-init)
      - [rly transact raw chan-confirm](#rly-transact-raw-chan-confirm)
      - [rly transact raw chan-init](#rly-transact-raw-chan-init)
      - [rly transact raw chan-try](#rly-transact-raw-chan-try)
      - [rly transact raw channel-step](#rly-transact-raw-channel-step)
      - [rly transact raw client](#rly-transact-raw-client)
      - [rly transact raw close-channel-step](#rly-transact-raw-close-channel-step)
      - [rly transact raw conn-ack](#rly-transact-raw-conn-ack)
      - [rly transact raw conn-confirm](#rly-transact-raw-conn-confirm)
      - [rly transact raw conn-init](#rly-transact-raw-conn-init)
      - [rly transact raw conn-try](#rly-transact-raw-conn-try)
      - [rly transact raw connection-step](#rly-transact-raw-connection-step)
      - [rly transact raw update-client](#rly-transact-raw-update-client)
      - [rly transact raw transfer](#rly-transact-raw-transfer)
    - [rly transact relay](#rly-transact-relay)
    - [rly transact send-packet](#rly-transact-send-packet)
    - [rly transact transfer](#rly-transact-transfer)
  - [rly version](#rly-version)

## rly chains

Manage chains configurations

### Synopsis

Manage chains configurations

### Subcommands

* [rly chains add](#rly-chains-add)	 - Add a new chain to the configuration file by passing a file (-f) or url (-u), or user input
* [rly chains add-dir](#rly-chains-add-dir)	 - Add new chains to the configuration file from a directory full of chain configuration, useful for adding testnet configurations
* [rly chains address](#rly-chains-address)	 - Returns a chain's configured key's address
* [rly chains delete](#rly-chains-delete)	 - Deletes chain configuration data
* [rly chains edit](#rly-chains-edit)	 - Edit chain configuration
* [rly chains list](#rly-chains-list)	 - Returns all chains configuration data
* [rly chains show](#rly-chains-show)	 - Returns a chain's configuration data


## rly chains add

Add a new chain to the configuration file by passing a file (-f) or url (-u), or user input

### Synopsis

Add a new chain to the configuration file by passing a file (-f) or url (-u), or user input

```
rly chains add [flags]
```

### Options

```
  -f, --file string   fetch json data from specified file
  -u, --url string    url to fetch data from
```


## rly chains add-dir

Add new chains to the configuration file from a directory full of chain configuration, useful for adding testnet configurations

### Synopsis

Add new chains to the configuration file from a directory full of chain configuration, useful for adding testnet configurations

```
rly chains add-dir [dir]
```

## rly chains address

Returns a chain's configured key's address

### Synopsis

Returns a chain's configured key's address

```
rly chains address [chain-id]
```

## rly chains delete

### Synopsis

Deletes the chain configuration data (does not clear lite client or close and channels)

```
rly chains delete [chain-id] [flags]
```


## rly chains edit

Edits chain configuration data

### Synopsis

Set chain configuration key's value

```
rly chains edit [chain-id] [key] [value] [flags]
```


## rly chains list

Returns all chains configuration data

### Synopsis

Returns all chains configuration data

```
rly chains list [flags]
```

### Options

```
  -j, --json   returns the response in json format
  -y, --yaml   output using yaml
```


## rly chains show

Returns a chain's configuration data

### Synopsis

Returns a chain's configuration data

```
rly chains show [chain-id] [flags]
```

### Options

```
  -j, --json   returns the response in json format
  -y, --yaml   output using yaml
```

## rly config

Manage relayer's main configuration file

### Synopsis

Manage configuration file

### Subcommands

* [rly config add-dir](#rly-config-add-dir)	 - Add new chains and paths to the configuration file from a directory full of chain and path configuration, useful for adding testnet configurations
* [rly config init](#rly-config-init)	 - Creates a default home directory at path defined by --home
* [rly config show](#rly-config-show)	 - Prints current configuration

## rly config add-dir

Add new chains and paths to the configuration file from a directory full of chain and path configuration, useful for adding testnet configurations

### Synopsis

Add new chains and paths to the configuration file from a directory full of chain and path configuration, useful for adding testnet configurations

```
rly config add-dir [dir] [flags]
```

## rly config init

Creates a default home directory at path defined by --home

### Synopsis

Creates a default home directory at path defined by --home

```
rly config init [flags]
```

### Options inherited from parent commands

```
      --home string     set home directory (default "/Users/vsh/.relayer")
```

## rly config show

Prints current configuration

### Synopsis

Prints current configuration

```
rly config show [flags]
```


## rly development

commands for developers either deploying or hacking on the relayer

### Synopsis

commands for developers either deploying or hacking on the relayer

### Subcommands

* [rly development faucet](#rly-development-faucet)	 - faucet returns a sample faucet service file
* [rly development gaia](#rly-development-gaia)	 - gaia returns a sample gaiad service file
* [rly development genesis](#rly-development-genesis)	 - fetch the genesis file for a configured chain
* [rly development listen](#rly-development-listen)	 - listen to all transaction and block events from a given chain and output them to stdout
* [rly development relayer](#rly-development-relayer)	 - relayer returns a service file for the relayer to relay over an individual path

## rly development faucet

faucet returns a sample faucet service file

### Synopsis

faucet returns a sample faucet service file

```
rly development faucet [user] [home] [chain-id] [key-name] [amount] [flags]
```


## rly development gaia

gaia returns a sample gaiad service file

### Synopsis

gaia returns a sample gaiad service file

```
rly development gaia [user] [home] [flags]
```


## rly development genesis

fetch the genesis file for a configured chain

### Synopsis

fetch the genesis file for a configured chain

```
rly development genesis [chain-id] [flags]
```


## rly development listen

listen to all transaction and block events from a given chain and output them to stdout

### Synopsis

listen to all transaction and block events from a given chain and output them to stdout

```
rly development listen [chain-id] [flags]
```

### Options

```
      --data       output full event data
  -b, --no-block   don't output block events
  -t, --no-tx      don't output transaction events
```


## rly development relayer

relayer returns a service file for the relayer to relay over an individual path

### Synopsis

relayer returns a service file for the relayer to relay over an individual path

```
rly development relayer [path-name] [flags]
```


## rly keys

manage keys held by the relayer for each chain

### Synopsis

manage keys held by the relayer for each chain

### Subcommands

* [rly keys add](#rly-keys-add)	 - adds a key to the keychain associated with a particular chain
* [rly keys delete](#rly-keys-delete)	 - deletes a key from the keychain associated with a particular chain
* [rly keys export](#rly-keys-export)	 - exports a privkey from the keychain associated with a particular chain
* [rly keys list](#rly-keys-list)	 - lists keys from the keychain associated with a particular chain
* [rly keys restore](#rly-keys-restore)	 - restores a mnemonic to the keychain associated with a particular chain
* [rly keys show](#rly-keys-show)	 - shows a key from the keychain associated with a particular chain

## rly keys add

adds a key to the keychain associated with a particular chain

### Synopsis

adds a key to the keychain associated with a particular chain

```
rly keys add [chain-id] [[name]] [flags]
```


## rly keys delete

deletes a key from the keychain associated with a particular chain

### Synopsis

deletes a key from the keychain associated with a particular chain

```
rly keys delete [chain-id] [[name]] [flags]
```


## rly keys export

exports a privkey from the keychain associated with a particular chain

### Synopsis

exports a privkey from the keychain associated with a particular chain

```
rly keys export [chain-id] [name] [flags]
```


## rly keys list

lists keys from the keychain associated with a particular chain

### Synopsis

lists keys from the keychain associated with a particular chain

```
rly keys list [chain-id] [flags]
```


## rly keys restore

restores a mnemonic to the keychain associated with a particular chain

### Synopsis

restores a mnemonic to the keychain associated with a particular chain

```
rly keys restore [chain-id] [name] [mnemonic] [flags]
```


## rly keys show

shows a key from the keychain associated with a particular chain

### Synopsis

shows a key from the keychain associated with a particular chain

```
rly keys show [chain-id] [[name]] [flags]
```


## rly light

manage light clients held by the relayer for each chain

### Synopsis

manage light clients held by the relayer for each chain

### Subcommands

* [rly lite delete](#rly-lite-delete)	 - wipe the lite client database, forcing re-initialization on the next run
* [rly lite header](#rly-lite-header)	 - Get header from the database. 0 returns last trusted header and all others return the header at that height if stored
* [rly lite init](#rly-lite-init)	 - Initiate the light client
* [rly lite update](#rly-lite-update)	 - Update the light client by providing a new root of trust

## rly light delete

### Synopsis

wipe the lite client database, forcing re-initialization on the next run

```
rly light delete [chain-id] [flags]
```


## rly light header

Get header from the database. 0 returns last trusted header and all others return the header at that height if stored

### Synopsis

Get header from the database. 0 returns last trusted header and all others return the header at that height if stored

```
rly light header [chain-id] [height] [flags]
```


## rly light init

Initiate the light client

### Synopsis

Initiate the light client by:
	1. passing it a root of trust as a --hash/-x and --height
	2. via --url/-u where trust options can be found
	3. Use --force/-f to initalize from the configured node

```
rly light init [chain-id] [flags]
```

### Options

```
  -f, --force           option to force non-standard behavior such as initialization of light client from configured chain or generation of new path
  -x, --hash bytesHex   Trusted header's hash
      --height int      Trusted header's height (default -1)
  -u, --url string      Optional URL to fetch trusted-hash and trusted-height
```


## rly light update

Update the light client by providing a new root of trust

### Synopsis

Update the light client by
	1. providing a new root of trust as a --hash/-x and --height
	2. via --url/-u where trust options can be found
	3. updating from the configured node by passing no flags

```
rly light update [chain-id] [flags]
```

### Options

```
  -x, --hash bytesHex   Trusted header's hash
      --height int      Trusted header's height (default -1)
  -u, --url string      Optional URL to fetch trusted-hash and trusted-height
```


## rly paths

manage path configurations

### Synopsis

A path represents the "full path" or "link" for communication between two chains. This includes the client, 
connection, and channel ids from both the source and destination chains as well as the strategy to use when relaying

### Subcommands

* [rly paths add](#rly-paths-add)	 - add a path to the list of paths
* [rly paths delete](#rly-paths-delete)	 - delete a path with a given index
* [rly paths find](#rly-paths-find)	 - WIP: finds any existing paths between any configured chains and outputs them to stdout
* [rly paths generate](#rly-paths-generate)	 - generate identifiers for a new path between src and dst, reusing any that exist
* [rly paths list](#rly-paths-list)	 - print out configured paths
* [rly paths show](#rly-paths-show)	 - show a path given its name

## rly paths add

add a path to the list of paths

### Synopsis

add a path to the list of paths

```
rly paths add [src-chain-id] [dst-chain-id] [path-name] [flags]
```

### Options

```
  -f, --file string   fetch json data from specified file
```


## rly paths delete

delete a path with a given index

### Synopsis

delete a path with a given index

```
rly paths delete [index] [flags]
```


## rly paths find

WIP: finds any existing paths between any configured chains and outputs them to stdout

### Synopsis

WIP: finds any existing paths between any configured chains and outputs them to stdout

```
rly paths find [flags]
```


## rly paths generate

generate identifiers for a new path between src and dst, reusing any that exist

### Synopsis

generate identifiers for a new path between src and dst, reusing any that exist

```
rly paths generate [src-chain-id] [src-port] [dst-chain-id] [dst-port] [name] [flags]
```

### Options

```
  -f, --force       option to force non-standard behavior such as initialization of light client from configured chain or generation of new path
  -o, --unordered   create an unordered channel
```


## rly paths list

print out configured paths

### Synopsis

print out configured paths

```
rly paths list [flags]
```

### Options

```
  -j, --json   returns the response in json format
  -y, --yaml   output using yaml
```


## rly paths show

show a path given its name

### Synopsis

show a path given its name

```
rly paths show [path-name] [flags]
```

### Options

```
  -j, --json   returns the response in json format
  -y, --yaml   output using yaml
```


## rly query

IBC Query Commands

### Synopsis

Commands to query IBC primitives, and other useful data on configured chains.

### Subcommands

* [rly query account](#rly-query-account)	 - Query the account data
* [rly query balance](#rly-query-balance)	 - Query the account balances
* [rly query channel](#rly-query-channel)	 - Query a channel given it's channel and port ids
* [rly query channels](#rly-query-channels)	 - Query for all channels on a chain
* [rly query client](#rly-query-client)	 - Query the state of a client given it's client-id
* [rly query client-connections](#rly-query-client-connections)	 - Query for all connections on a given client
* [rly query clients](#rly-query-clients)	 - Query for all client states on a chain
* [rly query connection](#rly-query-connection)	 - Query the connection state for the given connection id
* [rly query connection-channels](#rly-query-connection-channels)	 - Query any channels associated with a given connection
* [rly query connections](#rly-query-connections)	 - Query for all connections on a chain
* [rly query full-path](#rly-query-full-path)	 - Query for the status of clients, connections, channels and packets on a path
* [rly query header](#rly-query-header)	 - Query the header of a chain at a given height
* [rly query node-state](#rly-query-node-state)	 - Query the consensus state of a client at a given height
* [rly query packet-ack](#rly-query-packet-ack)	 - Query for the packet acknowledgement given it's sequence and channel ids
* [rly query packet-commit](#rly-query-packet-commit)	 - Query for the packet commitment given it's sequence and channel ids
* [rly query seq-send](#rly-query-seq-send)	 - Query the next sequence send for a given channel
* [rly query tx](#rly-query-tx)	 - Query transaction by transaction hash
* [rly query txs](#rly-query-txs)	 - Query transactions by the events they produce
* [rly query unrelayed](#rly-query-unrelayed)	 - Query for the packet sequence numbers that remain to be relayed on a given path

## rly query account

Query the account data

### Synopsis

Query the account data

```
rly query account [chain-id] [flags]
```


## rly query balance

Query the account balances

### Synopsis

Query the account balances

```
rly query balance [chain-id] [[key-name]] [flags]
```

### Options

```
  -j, --json   returns the response in json format
```


## rly query channel

Query a channel given it's channel and port ids

### Synopsis

Query a channel given it's channel and port ids

```
rly query channel [chain-id] [channel-id] [port-id] [flags]
```

### Options

```
  -l, --limit int   pagination limit of light clients to query for (default 100)
  -p, --page int    pagination page of light clients to to query for (default 1)
```


## rly query channels

Query for all channels on a chain

### Synopsis

Query for all channels on a chain

```
rly query channels [chain-id] [flags]
```

### Options

```
  -l, --limit int   pagination limit of light clients to query for (default 100)
  -p, --page int    pagination page of light clients to to query for (default 1)
```


## rly query client-connections

Query for all connections on a given client

### Synopsis

Query for all connections on a given client

```
rly query client-connections [chain-id] [client-id] [flags]
```

### Options

```
  -l, --limit int   pagination limit of light clients to query for (default 100)
  -p, --page int    pagination page of light clients to to query for (default 1)
```


## rly query client

Query the state of a client given it's client-id

### Synopsis

Query the state of a client given it's client-id

```
rly query client [chain-id] [client-id] [flags]
```


## rly query clients

Query for all client states on a chain

### Synopsis

Query for all client states on a chain

```
rly query clients [chain-id] [flags]
```

### Options

```
  -l, --limit int   pagination limit of light clients to query for (default 100)
  -p, --page int    pagination page of light clients to to query for (default 1)
```


## rly query connection-channels

Query any channels associated with a given connection

### Synopsis

Query any channels associated with a given connection

```
rly query connection-channels [chain-id] [connection-id] [flags]
```

### Options

```
  -l, --limit int   pagination limit of light clients to query for (default 100)
  -p, --page int    pagination page of light clients to to query for (default 1)
```


## rly query connection

Query the connection state for the given connection id

### Synopsis

Query the connection state for the given connection id

```
rly query connection [chain-id] [connection-id] [flags]
```


## rly query connections

Query for all connections on a chain

### Synopsis

Query for all connections on a chain

```
rly query connections [chain-id] [flags]
```

### Options

```
  -l, --limit int   pagination limit of light clients to query for (default 100)
  -p, --page int    pagination page of light clients to to query for (default 1)
```


## rly query full-path

Query for the status of clients, connections, channels and packets on a path

### Synopsis

Query for the status of clients, connections, channels and packets on a path

```
rly query full-path [path-name] [flags]
```


## rly query header

Query the header of a chain at a given height

### Synopsis

Query the header of a chain at a given height

```
rly query header [chain-id] [height] [flags]
```

### Options

```
  -f, --flags   pass flag to output the flags for light init/update
```


## rly query node-state

Query the consensus state of a client at a given height

### Synopsis

Query the consensus state of a client at a given height

```
rly query node-state [chain-id] [height] [flags]
```


## rly query packet-ack

Query for the packet acknowledgement given it's sequence and channel ids

### Synopsis

Query for the packet acknowledgement given it's sequence and channel ids

```
rly query packet-ack [chain-id] [channel-id] [port-id] [seq] [flags]
```

### Options

```
  -l, --limit int   pagination limit of light clients to query for (default 100)
  -p, --page int    pagination page of light clients to to query for (default 1)
```


## rly query packet-commit

Query for the packet commitment given it's sequence and channel ids

### Synopsis

Query for the packet commitment given it's sequence and channel ids

```
rly query packet-commit [chain-id] [channel-id] [port-id] [seq] [flags]
```

### Options

```
  -l, --limit int   pagination limit of light clients to query for (default 100)
  -p, --page int    pagination page of light clients to to query for (default 1)
```


## rly query seq-send

Query the next sequence send for a given channel

### Synopsis

Query the next sequence send for a given channel

```
rly query seq-send [chain-id] [channel-id] [port-id] [flags]
```

### Options

```
  -l, --limit int   pagination limit of light clients to query for (default 100)
  -p, --page int    pagination page of light clients to to query for (default 1)
```


## rly query tx

Query transaction by transaction hash

### Synopsis

Query transaction by transaction hash

```
rly query tx [chain-id] [tx-hash] [flags]
```


## rly query txs

Query transactions by the events they produce

### Synopsis

Search for transactions that match the exact given events where results are paginated. Each event 
takes the form of '{eventType}.{eventAttribute}={value}' with multiple events seperated by '&'. 
Please refer to each module's documentation for the full set of events to query for. Each module
documents its respective events under 'cosmos-sdk/x/{module}/spec/xx_events.md'.

```
rly query txs [chain-id] [events] [flags]
```

### Options

```
  -l, --limit int   pagination limit of light clients to query for (default 100)
  -p, --page int    pagination page of light clients to to query for (default 1)
```


## rly query unrelayed

Query for the packet sequence numbers that remain to be relayed on a given path

### Synopsis

Query for the packet sequence numbers that remain to be relayed on a given path

```
rly query unrelayed [path] [flags]
```


## rly start

Start the listening relayer on a given path according to a path's strategy.

### Synopsis

Start the listening relayer on a given path. A path must have an associated relaying strategy. Starts a loop where relayer listens for events in connected chains and, if the event requires action according to the strategy (e.g. someone posted first half of the transfer in the chain A), relayer takes the required action (e.g. finish transfer with an appropriate tx in chain B)

```
rly start [path-name] [flags]
```


## rly testnets

commands for joining and running relayer testnets

### Synopsis

commands for joining and running relayer testnets

### Subcommands

* [rly testnets faucet](#rly-testnets-faucet)	 - listens on a port for requests for tokens
* [rly testnets request](#rly-testnets-request)	 - request tokens from a relayer faucet

## rly testnets faucet

listens on a port for requests for tokens

### Synopsis

listens on a port for requests for tokens

```
rly testnets faucet [chain-id] [key-name] [amount] [flags]
```

### Options

```
  -l, --listen string   sets the faucet listener addresss (default "0.0.0.0:8000")
```


## rly testnets request

request tokens from a relayer faucet

### Synopsis

request tokens from a relayer faucet

```
rly testnets request [chain-id] [[key-name]] [flags]
```

### Options

```
  -u, --url string   url to fetch data from
```


## rly transact

IBC Transaction Commands

### Synopsis

Commands to create IBC transactions on configured chains. Most of these commands take a '[path]' arguement. Make sure:
	1. Chains are properly configured to relay over by using the 'rly chains list' command
	2. Path is properly configured to relay over by using the 'rly paths list' command

### Subcommands

* [rly transact channel](#rly-transact-channel)	 - create a channel between two configured chains with a configured path
* [rly transact channel-close](#rly-transact-channel-close)	 - close a channel between two configured chains with a configured path
* [rly transact clients](#rly-transact-clients)	 - create a clients between two configured chains with a configured path
* [rly transact connection](#rly-transact-connection)	 - create a connection between two configured chains with a configured path
* [rly transact link](#rly-transact-link)	 - create clients, connection, and channel between two configured chains with a configured path
* [rly transact raw](#rly-transact-raw)	 - raw IBC transaction commands
* [rly transact relay](#rly-transact-relay)	 - relay any packets that remain to be relayed on a given path, in both directions
* [rly transact send-packet](#rly-transact-send-packet)	 - send a raw packet from a source chain to a destination chain
* [rly transact transfer](#rly-transact-transfer)	 - transfer tokens from a source chain to a destination chain in one command

## rly transact channel-close

close a channel between two configured chains with a configured path

### Synopsis

This command is meant to close a channel

```
rly transact channel-close [path-name] [flags]
```

### Options

```
  -o, --timeout string   timeout between relayer runs (default "10s")
```


## rly transact channel

create a channel between two configured chains with a configured path

### Synopsis

This command is meant to be used to repair or create a channel between two chains with a configured path in the config file

```
rly transact channel [path-name] [flags]
```

### Options

```
  -o, --timeout string   timeout between relayer runs (default "10s")
```


## rly transact clients

create a clients between two configured chains with a configured path

### Synopsis

create a clients between two configured chains with a configured path

```
rly transact clients [path-name] [flags]
```


## rly transact connection

create a connection between two configured chains with a configured path

### Synopsis

This command is meant to be used to repair or create a connection between two chains with a configured path in the config file

```
rly transact connection [path-name] [flags]
```

### Options

```
  -o, --timeout string   timeout between relayer runs (default "10s")
```


## rly transact link

create clients, connection, and channel between two configured chains with a configured path

### Synopsis

create clients, connection, and channel between two configured chains with a configured path

```
rly transact link [path-name] [flags]
```

### Options

```
  -o, --timeout string   timeout between relayer runs (default "10s")
```


## rly transact raw

raw IBC transaction commands

### Synopsis

raw IBC transaction commands

### Subcommands

* [rly transact raw chan-ack](#rly-transact-raw-chan-ack)	 - chan-ack
* [rly transact raw chan-close-confirm](#rly-transact-raw-chan-close-confirm)	 - chan-close-confirm
* [rly transact raw chan-close-init](#rly-transact-raw-chan-close-init)	 - chan-close-init
* [rly transact raw chan-confirm](#rly-transact-raw-chan-confirm)	 - chan-confirm
* [rly transact raw chan-init](#rly-transact-raw-chan-init)	 - chan-init
* [rly transact raw chan-try](#rly-transact-raw-chan-try)	 - chan-try
* [rly transact raw channel-step](#rly-transact-raw-channel-step)	 - create the next step in creating a channel between chains with the passed identifiers
* [rly transact raw client](#rly-transact-raw-client)	 - create a client for dst-chain on src-chain
* [rly transact raw close-channel-step](#rly-transact-raw-close-channel-step)	 - create the next step in closing a channel between chains with the passed identifiers
* [rly transact raw conn-ack](#rly-transact-raw-conn-ack)	 - conn-ack
* [rly transact raw conn-confirm](#rly-transact-raw-conn-confirm)	 - conn-confirm
* [rly transact raw conn-init](#rly-transact-raw-conn-init)	 - conn-init
* [rly transact raw conn-try](#rly-transact-raw-conn-try)	 - conn-try
* [rly transact raw connection-step](#rly-transact-raw-connection-step)	 - create a connection between chains, passing in identifiers
* [rly transact raw update-client](#rly-transact-raw-update-client)	 - update client for dst-chain on src-chain
* [rly transact raw transfer](#rly-transact-raw-transfer)	 - transfer

## rly transact raw chan-ack

chan-ack

### Synopsis

chan-ack

```
rly transact raw chan-ack [src-chain-id] [dst-chain-id] [src-client-id] [src-chan-id] [dst-chan-id] [src-port-id] [dst-port-id] [flags]
```


## rly transact raw chan-close-confirm

chan-close-confirm

### Synopsis

chan-close-confirm

```
rly transact raw chan-close-confirm [src-chain-id] [dst-chain-id] [src-client-id] [src-chan-id] [dst-chan-id] [src-port-id] [dst-port-id] [flags]
```


## rly transact raw chan-close-init

chan-close-init

### Synopsis

chan-close-init

```
rly transact raw chan-close-init [chain-id] [channel-id] [port-id] [flags]
```


## rly transact raw chan-confirm

chan-confirm

### Synopsis

chan-confirm

```
rly transact raw chan-confirm [src-chain-id] [dst-chain-id] [src-client-id] [src-chan-id] [dst-chan-id] [src-port-id] [dst-port-id] [flags]
```


## rly transact raw chan-init

chan-init

### Synopsis

chan-init

```
rly transact raw chan-init [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-conn-id] [dst-conn-id] [src-chan-id] [dst-chan-id] [src-port-id] [dst-port-id] [ordering] [flags]
```


## rly transact raw chan-try

chan-try

### Synopsis

chan-try

```
rly transact raw chan-try [src-chain-id] [dst-chain-id] [src-client-id] [src-conn-id] [src-chan-id] [dst-chan-id] [src-port-id] [dst-port-id] [flags]
```


## rly transact raw channel-step

create the next step in creating a channel between chains with the passed identifiers

### Synopsis

create the next step in creating a channel between chains with the passed identifiers

```
rly transact raw channel-step [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-connection-id] [dst-connection-id] [src-channel-id] [dst-channel-id] [src-port-id] [dst-port-id] [ordering] [flags]
```


## rly transact raw client

create a client for dst-chain on src-chain

### Synopsis

create a client for dst-chain on src-chain

```
rly transact raw client [src-chain-id] [dst-chain-id] [client-id] [flags]
```


## rly transact raw close-channel-step

create the next step in closing a channel between chains with the passed identifiers

### Synopsis

create the next step in closing a channel between chains with the passed identifiers

```
rly transact raw close-channel-step [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-connection-id] [dst-connection-id] [src-channel-id] [dst-channel-id] [src-port-id] [dst-port-id] [flags]
```


## rly transact raw conn-ack

conn-ack

### Synopsis

conn-ack

```
rly transact raw conn-ack [src-chain-id] [dst-chain-id] [dst-client-id] [src-client-id] [src-conn-id] [dst-conn-id] [flags]
```


## rly transact raw conn-confirm

conn-confirm

### Synopsis

conn-confirm

```
rly transact raw conn-confirm [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-conn-id] [dst-conn-id] [flags]
```


## rly transact raw conn-init

conn-init

### Synopsis

conn-init

```
rly transact raw conn-init [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-conn-id] [dst-conn-id] [flags]
```


## rly transact raw conn-try

conn-try

### Synopsis

conn-try

```
rly transact raw conn-try [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-conn-id] [dst-conn-id] [flags]
```


## rly transact raw connection-step

create a connection between chains, passing in identifiers

### Synopsis

This command creates the next handshake message given a specific set of identifiers. If the command fails, you can safely run it again to repair an unfinished connection

```
rly transact raw connection-step [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-connection-id] [dst-connection-id] [flags]
```


## rly transact raw update-client

update client for dst-chain on src-chain

### Synopsis

update client for dst-chain on src-chain

```
rly transact raw update-client [src-chain-id] [dst-chain-id] [client-id] [flags]
```


## rly transact raw transfer

transfer

### Synopsis

This sends tokens from a relayers configured wallet on chain src to a dst addr on dst

```
rly transact raw transfer [src-chain-id] [dst-chain-id] [amount] [source] [dst-addr] [flags]
```

### Options

```
  -p, --path string   specify the path to relay over
```


## rly transact relay

relay any packets that remain to be relayed on a given path, in both directions

### Synopsis

relay any packets that remain to be relayed on a given path, in both directions

```
rly transact relay [path-name] [flags]
```


## rly transact send-packet

send a raw packet from a source chain to a destination chain

### Synopsis

This sends packet-data (default: stdin) from a relayer's configured wallet on chain src to chain dst

```
rly transact send-packet [src-chain-id] [dst-chain-id] [packet-data] [flags]
```

### Options

```
  -p, --path string   specify the path to relay over
```


## rly transact transfer

transfer tokens from a source chain to a destination chain in one command

### Synopsis

This sends tokens from a relayers configured wallet on chain src to a dst addr on dst

```
rly transact transfer [src-chain-id] [dst-chain-id] [amount] [source] [dst-chain-addr] [flags]
```

### Options

```
  -p, --path string   specify the path to relay over
```


## rly version

Print relayer version info

### Synopsis

Print relayer version info

```
rly version [flags]
```

### Options

```
  -j, --json   returns the response in json format
```
