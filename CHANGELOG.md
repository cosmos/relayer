# Changelog

## v1.0.0-rc0

*5 November 2020*

This is the first RC for the 1.0.0 release of the relayer. Currently we are waiting on the following items for the v1.0.0 release:
- [ ] [`tendermint`](https://github.com/tendermint/tendermint) v0.34.0 final - We anticipate that the current [v0.34.0-rc6](https://github.com/tendermint/tendermint/releases/tag/v0.34.0-rc6) will be `v0.34.0` final after being tested in a series of testnets for the SDK
- [ ] [`cosmos-sdk`](https://github.com/cosmos/cosmos-sdk) v0.40.0 final - the `rc0` release of the relayer relies on `v0.40.0-rc2` of the `cosmos-sdk`, there will be a `v0.40.0-rc3` cut that included `tendermint@v0.34.0-rc6` as well as some minor changes.
- [ ] A fix for the [timeouts issue](https://github.com/cosmos/relayer/pull/322). Currently timeouts aren't passing proof validation and that will be fixed for both `ORDERED` and `UNORDERED` channels prior to a `v1.0.0` final on the relayer.
