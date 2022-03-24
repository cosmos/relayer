# Create Path Across Chains

✦ **NOTICE:** Please only create new paths on mainnet with intent. The Cosmos community thanks you ⚛️

---

In our "Relaying Packets Across Chains" example, we set up the relayer to relay between Cosmoshub and Osmosis. Since those two chains already have a path opened between them, we will use two mock chains: ibc-0 and ibc-1.

1. **Add basic path info to config.**

    ```shell
    # rly paths new [src-chain-id] [dst-chain-id] [path-name] [flags]

    $ rly paths new ibc-0 ibc-1 my_demo_path
    ```

2. **Next we need to create a `channel`, `client`, and `connection`.**

    The most efficient way to do this is to use `rly transaction link` command.

    ```shell
    # rly transact link [path-name] [flags]

    $ rly transact link my_demo_path
    ```

    This is a triplewammy, it creates a `client`, `connection`, and `channel` between the two chains. 

    If you would like more control, you can run each command individually:

    - `rly transact clients`
    - `rly transact connection`
    - `rly transact channel`

    <br>

    >Remember: `connections` are built on top of `client`s and `channels` are built on top of `connections`.
    
    If you get a `InvalidArgument desc = latest height revision number must match chain id revision number` error, there is likely already a client or channel setup between these chains, please check the relevant path.json file on the [chain-registry](https://github.com/cosmos/chain-registry). Path creation might not be necessary. You can use the `--override` flag if you with to force the creation of a new client/connection.

    All the above commands will update your config with the new path meta-data. 
    
    You can now start relaying over the configured path! [Relaying Packets Across Chains Step 4](../README.md#basic-usage---relaying-packets-across-chains)

---

[[TROUBLESHOOTING](./troubleshooting.md)]

---

<div align="center"> 

![banner](./images/github-repo-banner.gif)
 </div>

[<-- Home](../README.md) [Troubleshooting -->](./troubleshooting.md)