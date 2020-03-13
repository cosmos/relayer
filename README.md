# Relayer

The `relayer` package contains some basic relayer implementations that are
meant to be used by users wishing to relay packets between IBC enabled chains.
It is also well documented and intended as a place where users who are
interested in building their own relayer can come for working examples.

### Building and testing the relayer

The current implementation status is below. To test the functionality that does work you can do the following:

> NOTE: The relayer relies on `cosmos/cosmos-sdk@ibc-alpha` and `tendermint/tendermint@v0.33.0-dev2`. If you run into problems building it likely related to those dependancies. Also the `two-chains.sh` script requires that the `cosmos/gaia` and `cosmos/relayer` repos be present locally and buildable. Read the script and change the paths as needed.

```bash
# two-chains.sh creates two gaia-based chains with data directories in this 
$ ./two-chains.sh

# First initialize your configuration for the relayer
$ relayer config init

# NOTE: you may want to look at the config between these steps to see
# what is added in each step. The config is located at ~/.relayer/config/config.yaml
$ cat ~/.relayer/config/config.yaml

# Then add the chains and paths that you will need to work with the 
# gaia chains spun up by the two-chains script
$ relayer chains add -f demo/ibc0.json
$ relayer chains add -f demo/ibc1.json

$ cat ~/.relayer/config/config.yaml

# To finalize your config, add a path between the two chains
$ relayer paths add -f demo/path.json

# Now, add the key seeds from each chain to the relayer to give it funds to work with
$ relayer keys restore ibc0 testkey "$(jq -r '.secret' data/ibc0/n0/gaiacli/key_seed.json)" -a
$ relayer keys restore ibc1 testkey "$(jq -r '.secret' data/ibc1/n0/gaiacli/key_seed.json)" -a

# Then its time to initialize the relayer's lite clients for each chain
# All data moving forward is validated by these lite clients.
$ relayer lite init ibc0 -f
$ relayer lite init ibc1 -f

# At this point the relayer --home directory is ready for normal operations between 
# ibc0 and ibc1. Looking at the folder structure of the relayer at this point is helpful
$ tree ~/.relayer

# Now you can connect the two chains with one command:
$ relayer tx full-path ibc0 ibc1

# Then you can test sending a packet from ibc0 to ibc1
$ relayer tx raw xfer ibc0 ibc1
```

## Setting up Developer Environment

Working with the relayer can frequently involve working with local developement branches of `gaia`, `cosmos-sdk` and the `relayer`. To setup your environment to point at the local versions of the code and reduce the amount of time in your read-eval-print loops try the following:

1. Set `replace github.com/cosmos/cosmos-sdk => /path/to/local/github.com/comsos/cosmos-sdk` at the end of the `go.mod` files for the `relayer` and `gaia`. This will force building from the local versions when running the `./two-chains.sh` script.
2. After `./two-chains.sh` has run, use `go run main.go --home $RLY` for any relayer commands you are working on. This allows you make changes and immediately test them as long as there are no server side changes. 
3. If you make changes in `cosmos-sdk` that need to be reflected server-side, be sure to re-run `./two-chains.sh`.
4. If you need to work off of a `gaia` branch other than `ibc-alpha`, change the branch name at the top of the `./two-chains.sh` script. 