<div align="center">
  <h1>Relayer</h1>

![banner](./docs/images/comp.gif)

[![Project Status: Initial Release](https://img.shields.io/badge/repo%20status-active-green.svg?style=flat-square)](https://www.repostatus.org/#active)
![GitHub Workflow Status](https://github.com/cosmos/relayer/actions/workflows/build.yml/badge.svg)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue?style=flat-square&logo=go)](https://godoc.org/github.com/cosmos/relayer)
[![Go Report Card](https://goreportcard.com/badge/github.com/cosmos/relayer)](https://goreportcard.com/report/github.com/cosmos/relayer)
[![License: Apache-2.0](https://img.shields.io/github/license/cosmos/relayer.svg?style=flat-square)](https://github.com/cosmos/relayer/blob/main/LICENSE)
[![Lines Of Code](https://img.shields.io/tokei/lines/github/cosmos/relayer?style=flat-square)](https://github.com/cosmos/relayer)
[![Version](https://img.shields.io/github/tag/cosmos/relayer.svg?style=flat-square)](https://github.com/cosmos/relayer/latest)
</div>

In IBC, blockchains do not directly pass messages to each other over the network. This is where `relayer` comes in. 
A relayer process monitors for updates on opens paths between sets of [IBC](https://ibcprotocol.org/)-enabled chains.
The relayer submits these updates in the form of specific message types to the counterparty chain. Clients are then used to 
track and verify the concnesus state.

In addtion to relaying packets, this relayer can open paths across chains, thus creating clients, connections and channels.

Additional information on how IBC works can be found [here]().

---

## Table Of Contents
- [Basic Usage - Relaying Across Chains](#Basic-Usage-Relaying-Packets-Across-Chains)
- [Create Path Across Chains](./docs/create-path-across-chain.md)
- [Troubleshooting](./docs/troubleshooting.md)
- [Features](./docs/features.md)
- [Relayer Terminology](./docs/terminology.md)
- [Recommended Pruning Settings](./docs/node_pruning.md)
- [Demo](./docs/demo.md)
- [Security Notice](./docs/sec-and_code-of-conduct.md#security-notice)
- [Code of Conduct](./docs/sec-and_code-of-conduct.md#code-of-conduct)

---

## Basic Usage - Relaying Packets Across Chains

---

\*\*The `-h` (help) flag tailing any `rly` command will be your best friend. USE THIS IN YOUR RELAYING JOURNEY.\*\*

---

To setup the IBC relayer on an active path between two IBC-enabled networks:

---

1. **Clone, checkout and install the latest release ([releases page](https://github.com/cosmos/relayer/releases)).**

   *[Go](https://go.dev/doc/install) needs to be installed and a proper Go environment needs to be configured*

    ```
    $ git clone git@github.com:cosmos/relayer.git
    $ git checkout v2.0.0
    $ cd relayer && make install
    ```

2. **Initialize the relayer's configuration directory/file.**
   
   ```shell
   $ rly config init
   ```
   **Config file location: `~/.relayer/config.yaml`**


3. **Configure the relayer to operate on desired chains.**
   
   In our example, we will configure between the Cosmos Hub and Osmosis.
   
   ```shell
   $ rly chains add cosmoshub osmosis
   ```
   The `rly chains add` command fetches chain meta-data from the [chain-registry](https://github.com/cosmos/chain-registry) and adds it to your config file.
   > **NOTE:** Don't see the chain you want to relay on?   
   > Please open a PR to add this metadata to the GitHub repo!
   
   Adding from the chain-registry randomly selects an RPC address. If you are running your own node, manually go into config and adjust setting.
   
   You also have the option to add chain from a [file](configs/demo/chains/ibc-0.json) or url using flags.

4. **Import OR create new keys for the relayer to use when signing and relaying transactions.**

   >These keys will need funds in order to relay (Step 6)
   
   `key-name` is an identifier of your choosing.    

   **Create**(add):
   
    ```shell
    $ rly keys add cosmoshub-4 [key-name]  
    $ rly keys add osmosis-1 [key-name]  
    ```
      OR

   **Import**(restore):
   ```shell
   $ rly keys restore cosmoshub-4 [key-name] "mnemonic words here"
   $ rly keys restore osmosis-1 [key-name] "mnemonic words here"
   ```

5. **Edit the relayer's "key:" values in the config file to match the `key-name`'s chosen above.**

   This step is necessary if you chose a `key-name` other than "default" 

   Example:
      ```yaml
      - type: cosmos
         value:
         key: YOUR-KEY-NAME-HERE
         chain-id: ibc-0
         rpc-addr: http://localhost:26657
      ```

6. **Ensure the keys associated with the configured chains are funded.**

   This is necessary to relay.
   
   You can query the balance of each configured key by running:

   ```shell
   $ rly q balance cosmoshub-4
   $ rly q balance osmosis-1
   ```

7. **Configure path meta-data in config file.**

   We have the chain-meta data configured, now we need path meta data. Educate yourself on `path` terminology [here](docs/troubleshooting.md).

   `rly paths fetch` will check for the relevant `path.json` files for ALL configured chains in your config file. The path meta-data is queriered from the [interchain](https://github.com/cosmos/relayer/tree/main/interchain) directory.

    > **NOTE:** Don't see the path metadata for paths you want to relay on?   
    > Please open a PR to add this metadata to the GitHub repo!

     ```shell
     $ rly paths fetch
     ```

   At minimum, this command will add two paths, in our case, one path from cosmoshub to osmosis and another path from osmosis to cosmoshub. 

   By default, the relayer will relay on all channels on a given connection. 
   
   You have the option to add a channel filter. You can do so by manually adding a `src-channel-filter:` item to your config.

   The `rule` can either be "allowlist", "denylist" or left blank.

   Example:

   ```yaml
   hubosmo:
        src:
            chain-id: cosmoshub-4
            client-id: 07-tendermint-259
            connection-id: connection-257
        dst:
            chain-id: osmosis-1
            client-id: 07-tendermint-1
            connection-id: connection-1
        src-channel-filter:
                rule: allowlist
                channel-list: [channel-207, channel-192, channel-184]  
   ```

8. **Finally, we start the relayer on the desired path.**

    The relayer will periodically update the clients and listen for IBC messages to relay.

    ```shell
    $ rly paths list
    $ rly start {path}
    ```
   
   You will need to start a separate shell instance for each path you wish to relay over. 

   ---
   
   [[TROUBLESHOOTING](docs/troubleshooting.md)]

---

[Create Path Across Chaines -->](docs/create-path-across-chain.md)