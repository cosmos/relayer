# Migrating old config files (prior to v2.0.0-rc1)

Version v2.0.0-rc1 introduces a new config layout. Old config files will not be compatible. 

To migrate, you will need to re-initialize your config file. Follow the steps below:

1) Delete old config (assuming it's in default location).
```sh
 rm -r ~/.relayer/config
```

2) Re-init config.
```sh
rly config init
```

3) Add chains to relay for to the config. This fetches chain meta-data from `cosmos/chain-registry`. Example:
```sh
rly chains add cosmoshub osmosis juno
```

4) Auto configure path meta-data from `cosmos/chain-registry/_IBC` (you can optionally do this manually).
```sh
rly paths fetch
```
OPTIONAL: Manually add channel filters. See: [configure-channel-filter]https://github.com/cosmos/relayer#configure-the-channel-filter


>NOTE: Naming of the auto-configured paths has changed to be less abbreviated. So for example "hubosmo" is now "cosmoshub-osmosis". These paths are bi-directional and only need to be added to the config once. So having both "hubosmo" and "osmohub" is not necessary, you just need "cosmoshub-osmosis" 


As long as you do not delete `~/.relayer/config/keys/`, you will not have to restore your keys. 