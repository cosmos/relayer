## Demo/Dev-Environment

The demo environment is a series of bash scripts that:

1) Spins up two IBC enabled blockchains chains (gaia) in your local environment
2) Creates an IBC connection between both chains
3) Sends an IBC transaction between both chains

This can be used to learn about the inter workings of IBC. Follow along with the commands inside of the bash scripts to get a better idea.

This can also be used to spin up a quick testing environment.

To run:

```bash
cd examples/demo/
./dev-env
```

This script creates a folder called "data": `examples/demo/data/`. 
Logs and config info for each chain can be found here.


Note: After running, two `gaiad` instances will be running on your machine. 
To kill ALL `gaiad` instances run:
```bash
killall gaiad
```

---

## Example Config: [examples/config.yaml](./config_EXAMPLE.yaml)

This is an example of a config file with:

- Three chains added: `cosmoshub`, `juno`, and `osmosis`
- Three paths configured: `cosmoshub-juno`, `cosmoshub-osmosis`, `juno-osmosis`
    - Path `cosmoshub-juno` does not filter any channels while the other two paths have filters set.
- All three chains have a wallet/key called "default"