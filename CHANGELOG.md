# Changelog

## v0.8.0

**2021/02/17**

* [\#428](https://github.com/cosmos/relayer/pull/428) Fix add-paths failure when called on exiting configuration
* [\#427](https://github.com/cosmos/relayer/pull/427) Fix nil paths bug which occurred when validating paths
* [\#424](https://github.com/cosmos/relayer/pull/424) Fix update bug via DRY handshake code
* [\#412](https://github.com/cosmos/relayer/pull/412) Auto update clients to prevent expiry. `rly start` command supports auto updating a client if it is about to expire. Use the `--time-threshold` flag.
* [\#323](https://github.com/cosmos/relayer/pull/323) Implmenet an API server. A rest server with API endpoints to support interacting with the relayer
* [\#421](https://github.com/cosmos/relayer/pull/421) Fix update client bug and reduce code complexity
* [\#416](https://github.com/cosmos/relayer/pull/416) Refactor light client handling, remove dependency on historical info for constructing update messages
* [\#419](https://github.com/cosmos/relayer/pull/419) Fix acknowledgement bug which occurred in a 3 chain environment
* [\#410](https://github.com/cosmos/relayer/pull/410) Remove rly tx channel command code. It was a duplicate of rly tx link, which contains channel as an alias for link. 
* [\#411](https://github.com/cosmos/relayer/pull/411) Make root command publicly accessible
* [\#408](https://github.com/cosmos/relayer/pull/408) Fix nchainz script
* [\#406](https://github.com/cosmos/relayer/pull/406) Split `add-dir` into `add-chains` and `add-paths`. You must add a chain, then the keys, and then the paths. This enables support of bottom up validation.
* [\#402](https://github.com/cosmos/relayer/pull/402) Chain.logger is configurable
* [\#399](https://github.com/cosmos/relayer/pull/399) Config file can be shown as json, rly tx conn creates clients as necessary, minor fixes
* [\#398](https://github.com/cosmos/relayer/pull/398) Bottom up configuration file validation
* [\#394](https://github.com/cosmos/relayer/pull/394) Fix lint issues
* [\#390](https://github.com/cosmos/relayer/pull/390) Code improvements for upgrading clients

## v0.7.0

**2021/02/01**

* (deps) Bump SDK version to [v0.41.0](https://github.com/cosmos/cosmos-sdk/releases/tag/v0.41.0).
* Bug fixes and minor improvements
* (relayer)[\#329](https://github.com/cosmos/relayer/issues/329) client, conenction, and channel handshake refactor (identifiers generated auto generated on-chain)
* (relayer)[\#386](https://github.com/cosmos/relayer/pull/386) client, connection and channel identifier reuse
