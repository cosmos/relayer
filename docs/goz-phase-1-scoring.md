# Game of Zones Phase 1 Scoring Write Up

The following are instructions for how to reproduce the phase 1 scoring output.

1. Build `gaiad` and `gaiacli` locally from [this commit](https://github.com/cosmos/gaia/commit/b617e2b)
2. Run `gaiad init gameofzoneshub-1a --chain-id gameofzoneshub-1a` and then replace the `~/.gaiad/data` directory with the `gozhub-1a` [data dir from here](todo:add-link). Start `gaiad`, this should allow for querying of the hub-1a data.
3. Build `rly` locally from [this commit](https://github.com/iqlusioninc/relayer/commit/2282f8b).
4. Add a chain with `chain-id => gameofzoneshub-1a` and `rpc-add => http://localhost:26657` to the relayer configuration. Be sure to initialize the lite client.
5. Run `rly dev goz-client-data gameofzoneshub-1a goz-roster.csv > goz-client-data.json`. This will query the chain at tip for all clients and associate them with team data based on `chain-id`. This command takes a minute or two to run.
6. Run `rly dev phase-one gameofzoneshub-1a goz-client-data.json 35916 > export.log 2>&1 &`. This will query each block for data at that height about all clients in `goz-client-data.json` and save that data in a `phase-1/{height}.json` file for later processing. This command may take some hours to run depending on your hardware.
7. Start an [OSS influxdb instance](https://v2.docs.influxdata.com/v2.0/get-started/#start-with-influxdb-oss) and [configure it](https://v2.docs.influxdata.com/v2.0/get-started/#set-up-influxdb). Keep the UI at `http://localhost:9999` up. Make sure to set the following `env` in the shell you are using: `INFLUX_AUTH_TOKEN` (found in the Data -> Tokens section of the UI),`INFLUX_BUCKET`,`INFLUX_ORG`.
8. Run `rly dev process gameofzoneshub-1a phase-1/ 35916`. This will iterate over the per block data and write them as data points to influxdb for visualization and analysis. This will take 10-20 minutes.
9. Using the data explorer tab on `localhost:9999` use the `Script Editor` to run the following `flux` lang query and export the results to CSV. The game of zones team will score client that was updated for the longest by each team: 

```coffee
from(bucket: "mybucket")
  |> range(start: v.timeRangeStart, stop: v.timeRangeStop)
  |> filter(fn: (r) => r["_measurement"] == "client-updates")
  |> filter(fn: (r) => r["_field"] == "sinceLastUpdate")
  |> filter(fn: (r) => r._value < 5400000 )
  |> group(columns: ["chainID", "clientID", "teamname"])
  |> range(start: 2020-05-06T07:00:00Z, stop: 2020-05-10T19:00:00Z)
  |> count(column: "_value")
```