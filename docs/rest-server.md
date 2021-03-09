# Rest Server

## rly api

To start rest server, just run command `rly api`. This will start local server by default on `0.0.0.0:5183`. To modify api listen address, please edit `global.api-listen-addr` value in `~/.relayer/config/config.yaml` file.

```shell
$ rly api
listening on :5183
```

## Testing

Start two test chains for testing rest-server by running below commands:

```bash
# ensure go and jq are installed 
# Go Documentation: https://golang.org/doc/install
# jq Documentation: https://stedolan.github.io/jq/download

# First, download and build the gaia source code so we have a working blockchain to test against
$ make get-gaia build-gaia

# two-chainz creates two gaia-based chains with data directories in this repo
# it also builds and configures the relayer for operations with those chains
$ ./scripts/two-chainz
# NOTE: If you want to stop the two gaia-based chains running in the background use `killall gaiad`

# At this point the relayer --home directory is ready for normal operations between
# ibc-0 and ibc-1. Looking at the folder structure of the relayer at this point is helpful
```

Now, lets test some rest routes using `curl` command in shell:

```bash
# Get all the chains that are ready to relay over
$ curl http://localhost:5183/chains

[{"key":"testkey","chain-id":"ibc-0","rpc-addr":"http://localhost:26657","account-prefix":"cosmos","gas-adjustment":1.5,"gas-prices":"0.025stake","trusting-period":"336h"},{"key":"testkey","chain-id":"ibc-1","rpc-addr":"http://localhost:26557","account-prefix":"cosmos","gas-adjustment":1.5,"gas-prices":"0.025stake","trusting-period":"336h"}]

# Add a new chain 
# Here we are sending request body json as value to -d flag in curl
$ curl -d '{"key":"testkey3","rpc-addr":"http://localhost:26657","account-prefix":"ibc","gas-adjustment":"2","gas-prices":"0.05stake","trusting-period":"33h"}' -H 'Content-Type: application/json' http://localhost:5183/chains/ibc-2

"chain ibc-2 added successfully"

# Add new path to relay over
$ curl -d '{"src":{"chain-id":"ibc-0","client-id":"","connection-id":"","channel-id":"","port-id":"transfer","order":"unordered","version":"ics20-1"},"dst":{"chain-id":"ibc-1","client-id":"","connection-id":"","channel-id":"","port-id":"transfer","order":"unordered","version":"ics20-1"}}' -H 'Content-Type: application/json' http://localhost:5183/paths/demo-path

# Here we are creating path by sending data in format 
{
    src           PathEnd
    dst           PathEnd
}

# We can also create path by sending file path instead of src and dst data in below format
{
    file          string
}

$ curl -d '{"file":"/root/go/src/github.com/cosmos/relayer/configs/akash/demo.json"}' -H 'Content-Type: application/json' http://localhost:5183/paths/demo2

"path demo2 added successfully"

# Get all paths
$ curl http://localhost:5183/paths

{"demo-path":{"src":{"chain-id":"ibc-0","client-id":"07-tendermint-0","connection-id":"connection-0","channel-id":"channel-0","port-id":"transfer","order":"unordered","version":"ics20-1"},"dst":{"chain-id":"ibc-1","client-id":"07-tendermint-0","connection-id":"connection-0","channel-id":"channel-0","port-id":"transfer","order":"unordered","version":"ics20-1"},"strategy":{"type":"naive"}},"demo2":{"src":{"chain-id":"ibc-0","port-id":"transfer","order":"unordered","version":"ics20-1"},"dst":{"chain-id":"ibc-1","port-id":"transfer","order":"unordered","version":"ics20-1"},"strategy":{"type":"naive"}}}

```