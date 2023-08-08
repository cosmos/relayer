# Changelog

## [Unreleased]

### Relayer

* [\#487](https://github.com/cosmos/relayer/pull/487) Add override flag for client reuse.
* [\#491](https://github.com/cosmos/relayer/pull/491) Add flag for client parameters which allow or disallow governance to update frozen/expired clients
* [\#486](https://github.com/cosmos/relayer/pull/486) Refactor fly path generate command
* [\#475](https://github.com/cosmos/relayer/pull/475) remove cmd docs file
* [\#468](https://github.com/cosmos/relayer/pull/468) cli: UX cleanup for query commands
* [\#467](https://github.com/cosmos/relayer/pull/467) cli: UX cleanup for tx commands
* [\#466](https://github.com/cosmos/relayer/pull/466) Docs cleanup. 
* [\#506](https://github.com/cosmos/relayer/pull/506) Fix Timeout Handling on Relayer restart
* [\#940](https://github.com/cosmos/relayer/pull/940) Add min-gas-amount parameter for chain configs, to workaround gas estimation failure.
* [\#1177](https://github.com/cosmos/relayer/pull/1177) Avoid panic due to nil map when add new path and ensure path get written to config.
* [\#1178](https://github.com/cosmos/relayer/pull/1178) Add max-gas-amount parameter in chain configs.
* [\#1180](https://github.com/cosmos/relayer/pull/1180) Update SDK from v0.47.0 to v0.47.2.
* [\#1205](https://github.com/cosmos/relayer/pull/1205) Update ibc-go to v7.0.1.
* [\#1179](https://github.com/cosmos/relayer/pull/1179) Add extension-options parameter in chain configs and update SDK to v0.47.3.
* [\#1208](https://github.com/cosmos/relayer/pull/1208) Replace gogo/protobuf to cosmos/gogoproto.
* [\#1221](https://github.com/cosmos/relayer/pull/1221) Update cometbft to v0.37.2 and ibc-go to v7.2.0.
* [\#1226](https://github.com/cosmos/relayer/pull/1226) Avoid invalid Bech32 prefix error in parallel tests when sdk Config get overwritten by each other in single process.
* [\#1231](https://github.com/cosmos/relayer/pull/1231) Reduce get bech32 prefix when get signer.

## v0.9.3

**2021/04/14**

### Dependencies

* [\#492](https://github.com/cosmos/relayer/pull/492) Update SDK and Tendermint versions

### Relayer

* [\#488](https://github.com/cosmos/relayer/pull/488) Fix state based relaying for acknowledgements.
* [\#483](https://github.com/cosmos/relayer/pull/483) Fix timeout to correctly set revision number
* [\#485](https://github.com/cosmos/relayer/pull/485) Fix misbehaviour monitoring
* [\#490](https://github.com/cosmos/relayer/pull/490) Fix relay acknowledgement bug for streaming relays

## v0.9.2

**2021/03/29**

### Relayer

* [\#478](https://github.com/cosmos/relayer/pull/478) Retry on DB failure due to concurrent access.
* [\#471](https://github.com/cosmos/relayer/pull/471) Add a send command to send to a different address on the same chain.


## v0.9.1

**2021/03/26**

### Relayer

* [\#474](https://github.com/cosmos/relayer/pull/474) Fix validator set mismatch when updating IBC client.


## v0.9.0

**2021/03/23**

### Relayer

* [\#465](https://github.com/cosmos/relayer/pull/465) Fix light client error when getting the latest light header
* [\#462](https://github.com/cosmos/relayer/pull/462) Refactor integration test setup to allow support for misbehaviour tests
* [\#457](https://github.com/cosmos/relayer/pull/457) Simple misbehaviour monitoring
* [\#464](https://github.com/cosmos/relayer/pull/464) Fix query balance response
* [\#431](https://github.com/cosmos/relayer/pull/431) Refactor state based relaying to correctly construct the UpdateClient Msg on retries.
* [\#456](https://github.com/cosmos/relayer/pull/456) Fix bug which incorrectly set the timeout on a transfer.
* [\#455](https://github.com/cosmos/relayer/pull/455) Set default client parameter to allow governance to update the client if expiry or misbehaviour freezing occurs. 

## v0.8.3

**2021/03/12**

### Relayer

* [\#453](https://github.com/cosmos/relayer/pull/453) Fix light block not found error on missing header
* [\#449](https://github.com/cosmos/relayer/pull/449) Close database connection even if error occurs on initialization
* [\#447](https://github.com/cosmos/relayer/pull/447) Add a light client database lock to prevent concurrency panics
* [\#434](https://github.com/cosmos/relayer/pull/434) Implement swagger docs and fix path validation
* [\#448](https://github.com/cosmos/relayer/pull/448) update pruning error message

### Dependencies

* [\#451](https://github.com/cosmos/relayer/pull/451) bump SDK to version 0.42.0


## v0.8.2

**2021/03/01**

### Relayer

* [\#441](https://github.com/cosmos/relayer/pull/441) Disable tendermint light client light block pruning. 1 instance initialization was missed in #437. 
* [\#438](https://github.com/cosmos/relayer/pull/438) Typo fixes

## v0.8.1

**2021/02/26**

### Dependencies

* [\#430](https://github.com/cosmos/relayer/pull/430) Bump SDK version to v0.41.3

### Relayer

* [\#437](https://github.com/cosmos/relayer/pull/437) Off-chain Tendermint light client will no longer prune light blocks. The default pruning strategy broke the relayer after it wasn't used for 1000 blocks. 

## v0.8.0

**2021/02/17**

### Dependencies 

* [\#429](https://github.com/cosmos/relayer/pull/429) Bump SDK version to v0.41.1

### Relayer 

* [\#424](https://github.com/cosmos/relayer/pull/424) Fix update bug via DRY handshake code
* [\#421](https://github.com/cosmos/relayer/pull/421) Fix update client bug and reduce code complexity
* [\#416](https://github.com/cosmos/relayer/pull/416) Refactor light client handling, remove dependency on historical info for constructing update messages
* [\#419](https://github.com/cosmos/relayer/pull/419) Fix acknowledgement bug which occurred in a 3 chain environment
* [\#390](https://github.com/cosmos/relayer/pull/390) Code improvements for upgrading clients
* [\#394](https://github.com/cosmos/relayer/pull/394) Fix lint issues

### CLI/scripts

* [\#412](https://github.com/cosmos/relayer/pull/412) Auto update clients to prevent expiry. `rly start` command supports auto updating a client if it is about to expire. Use the `--time-threshold` flag.
* [\#323](https://github.com/cosmos/relayer/pull/323) Implmenet an API server. A rest server with API endpoints to support interacting with the relayer
* [\#406](https://github.com/cosmos/relayer/pull/406) Split `add-dir` into `add-chains` and `add-paths`. You must add a chain, then the keys, and then the paths. This enables support of bottom up validation.
* [\#428](https://github.com/cosmos/relayer/pull/428) Fix add-paths failure when called on exiting configuration
* [\#427](https://github.com/cosmos/relayer/pull/427) Fix nil paths bug which occurred when validating paths
* [\#410](https://github.com/cosmos/relayer/pull/410) Remove rly tx channel command code. It was a duplicate of rly tx link, which contains channel as an alias for link. 
* [\#411](https://github.com/cosmos/relayer/pull/411) Make root command publicly accessible
* [\#408](https://github.com/cosmos/relayer/pull/408) Fix nchainz script
* [\#402](https://github.com/cosmos/relayer/pull/402) Chain.logger is configurable
* [\#399](https://github.com/cosmos/relayer/pull/399) Config file can be shown as json, rly tx conn creates clients as necessary, minor fixes
* [\#398](https://github.com/cosmos/relayer/pull/398) Bottom up configuration file validation

## v0.7.0

**2021/02/01**

* (deps) Bump SDK version to [v0.41.0](https://github.com/cosmos/cosmos-sdk/releases/tag/v0.41.0).
* Bug fixes and minor improvements
* (relayer)[\#329](https://github.com/cosmos/relayer/issues/329) client, conenction, and channel handshake refactor (identifiers generated auto generated on-chain)
* (relayer)[\#386](https://github.com/cosmos/relayer/pull/386) client, connection and channel identifier reuse
