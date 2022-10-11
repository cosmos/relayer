# Troubleshooting


**Ensure `rly` package is properly installed**

   Run: 
   ```shell
   $ rly version
   ```

   If this returns an error, make sure you have Go installed and your Go environment is setup. Then redo [Step 1](#basic-usage---relaying-packets-across-chains).

---

 **Healthy relayer log with no packets to relay should look like:**

   ```log
2022-03-25T20:11:19.511489Z	info	No packets in queue	{"src_chain_id": "ibc-0", "src_channel_id": "channel-0", "src_port_id": "transfer", "dst_chain_id": "ibc-1", "dst_channel_id": "channel-0", "dst_port_id": "transfer"}
2022-03-25T20:11:19.514370Z	info	No acknowledgements in queue	{"src_chain_id": "ibc-0", "src_channel_id": "channel-0", "src_port_id": "transfer", "dst_chain_id": "ibc-1", "dst_channel_id": "channel-0", "dst_port_id": "transfer"}
2022-03-25T20:11:20.517184Z	info	No packets in queue	{"src_chain_id": "ibc-0", "src_channel_id": "channel-0", "src_port_id": "transfer", "dst_chain_id": "ibc-1", "dst_channel_id": "channel-0", "dst_port_id": "transfer"}
2022-03-25T20:11:20.523035Z	info	No acknowledgements in queue	{"src_chain_id": "ibc-0", "src_channel_id": "channel-0", "src_port_id": "transfer", "dst_chain_id": "ibc-1", "dst_channel_id": "channel-0", "dst_port_id": "transfer"}
2022-03-25T20:11:21.528712Z	info	No packets in queue	{"src_chain_id": "ibc-0", "src_channel_id": "channel-0", "src_port_id": "transfer", "dst_chain_id": "ibc-1", "dst_channel_id": "channel-0", "dst_port_id": "transfer"}
2022-03-25T20:11:21.532996Z	info	No acknowledgements in queue	{"src_chain_id": "ibc-0", "src_channel_id": "channel-0", "src_port_id": "transfer", "dst_chain_id": "ibc-1", "dst_channel_id": "channel-0", "dst_port_id": "transfer"}
2022-03-25T20:11:22.539200Z	info	No packets in queue	{"src_chain_id": "ibc-0", "src_channel_id": "channel-0", "src_port_id": "transfer", "dst_chain_id": "ibc-1", "dst_channel_id": "channel-0", "dst_port_id": "transfer"}
2022-03-25T20:11:22.543539Z	info	No acknowledgements in queue	{"src_chain_id": "ibc-0", "src_channel_id": "channel-0", "src_port_id": "transfer", "dst_chain_id": "ibc-1", "dst_channel_id": "channel-0", "dst_port_id": "transfer"}
```

---

**Verify valid `chain`, `client`, and `connection`**

```shell
$ rly paths list
```

If output:
```shell
-> chns(✘) clnts(✘) conn(✘)
```
Verify that you have a healthy RPC address. 

If:
```shell
-> chns(✔) clnts(✘) conn(✘)
```
Your client is the culprit here. Your client may be invalid or expired.

---

**Inspect Go runtime debug data**

If you started `rly` with the default `--debug-addr` argument,
you can open `http://localhost:7597` in your browser to explore details from the Go runtime.

If you need active assistance from the Relayer development team regarding an unresponsive Relayer instance,
it will be helpful to provide the output from `http://localhost:7597/debug/pprof/goroutine?debug=2` at a minimum.

---

**Error building or broadcasting transaction**

When preparing a transaction for relaying, the amount of gas that the transaction will consume is unknown.  To compute how much gas the transaction will need, the transaction is prepared with 0 gas and delivered as a `/cosmos.tx.v1beta1.Service/Simulate` query to the RPC endpoint.  Recently chains have been creating AnteHandlers in which 0 gas triggers an error case:

```
lvl=info msg="Error building or broadcasting transaction" provider_type=cosmos chain_id=evmos_9001-2 attempt=1 max_attempts=5 error="rpc error: code = InvalidArgument desc = provided fee < minimum global fee (0aevmos < ). Please increase the gas price.: insufficient fee: invalid request"
```

A workaround is available in which the `min-gas-amount` may be set in the chain's configuration to enable simulation with a non-zero amount of gas.

```
    evmos:
        type: cosmos
        value:
            key: relayer
            chain-id: evmos_9001-2
            rpc-addr: http://127.0.0.1:26657
            account-prefix: evmos
            keyring-backend: test
            gas-adjustment: 1.2
            gas-prices: 20000000000aevmos
            min-gas-amount: 1
            debug: false
            timeout: 20s
            output-format: json
            sign-mode: direct
```
[<-- Create Path Across Chains](create-path-across-chain.md) - [Features -->](./features.md)