# Troubleshooting

- Ensure you have the `rly` package properly installed.

   Run: 
   ```shell
   $ rly version
   ```
   Output should look something like this:
   ```
   version: 1.0.0-31-g8f20da2
   commit: 8f20da2866595fa7ca53cae3cf4e55b17208dc51
   cosmos-sdk: v0.45.1
   go: go1.17.6 darwin/amd64
   ```
   If you don't get this output make sure you have Go installed and your Go environment is setup. Then redo [Step 1](#basic-usage---relaying-packets-across-chains).

---

- A healthy relayer log with no packets to relay should look like:

   ```log
   I[2022-03-22|19:26:00.822] - No packets in the queue between [ibc-0]port{transfer} and [ibc-1]port{transfer} 
   I[2022-03-22|19:26:00.823] - No acks in the queue between [ibc-0]port{transfer} and [ibc-1]port{transfer} 
   I[2022-03-22|19:26:01.828] - No packets in the queue between [ibc-0]port{transfer} and [ibc-1]port{transfer} 
   I[2022-03-22|19:26:01.830] - No acks in the queue between [ibc-0]port{transfer} and [ibc-1]port{transfer} 
   ```

---

- Check to make sure you have your chains and paths configured by running:
```shell
$ rly paths list
```

If output:
```shell
-> chns(✘) clnts(✘) conn(✘)
```
Verify that you have a healthy RPC address. 


[[<-- Create Path Across Chaines  ](create-path-across-chain.md)[  Features -->](./features.md)