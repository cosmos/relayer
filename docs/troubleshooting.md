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
I[2022-03-25|09:34:28.328] - No packets in the queue between [ibc-0]chan(channel-0)port{transfer} and [ibc-1]chan(channel-0)port{transfer} 
I[2022-03-25|09:34:34.546] [ibc-0]chan(channel-0) unrelayed acks error: no error on QueryPacketUnrelayedAcknowledgements for ibc-0, however response is nil 
I[2022-03-25|09:34:34.554] - No packets in the queue between [ibc-0]chan(channel-0)port{transfer} and [ibc-1]chan(channel-0)port{transfer} 
I[2022-03-25|09:34:40.745] [ibc-0]chan(channel-0) unrelayed acks error: no error on QueryPacketUnrelayedAcknowledgements for ibc-1, however response is nil 
I[2022-03-25|09:34:40.753] - No packets in the queue between [ibc-0]chan(channel-0)port{transfer} and [ibc-1]chan(channel-0)port{transfer} 
I[2022-03-25|09:34:46.946] [ibc-0]chan(channel-0) unrelayed acks error: no error on QueryPacketUnrelayedAcknowledgements for ibc-1, however response is nil 
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

[<-- Create Path Across Chanes](create-path-across-chain.md) - [Features -->](./features.md)