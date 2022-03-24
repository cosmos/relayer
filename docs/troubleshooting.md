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
   I[2022-03-22|19:26:00.822] - No packets in the queue between [ibc-0]port{transfer} and [ibc-1]port{transfer} 
   I[2022-03-22|19:26:00.823] - No acks in the queue between [ibc-0]port{transfer} and [ibc-1]port{transfer} 
   I[2022-03-22|19:26:01.828] - No packets in the queue between [ibc-0]port{transfer} and [ibc-1]port{transfer} 
   I[2022-03-22|19:26:01.830] - No acks in the queue between [ibc-0]port{transfer} and [ibc-1]port{transfer} 
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

[<-- Create Path Across Chaines](create-path-across-chain.md) [Features -->](./features.md)