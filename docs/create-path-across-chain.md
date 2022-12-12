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

    This is a triplewammy, it creates a `client`, `connection`, and `channel` all in one command. 

    If you would like more control, you can run each command individually:

    - `rly transact clients`
    - `rly transact connection`
    - `rly transact channel`

    <br>

    All the above commands will update your config with the new path meta-data EXCEPT the new channel-id, which will be printed in stdout. 

    It's recommended to make note of this channel. If desired, add it to your ["allowlist"](../README.md#8--configure-the-channel-filter) in your config file. This would be `channel-0` from the print out below.

    ```log
    2022-03-25T20:09:33.997921Z	info	Channel created	{"src_chain_id": "ibc-0", "src_channel_id": "channel-0", "src_port_id": "transfer", "dst_chain_id": "ibc-1", "dst_channel_id": "channel-0", "dst_port_id": "transfer"}
    ```

    >Note: `connections` are built on top of `clients` and `channels` are built on top of `connections`.

    >As new `clients`, `connections` and `channels` are created on mainnet, consider adding and tagging them on the relevant paths.json file on the [chain-registry](https://github.com/cosmos/chain-registry) 

    
    CONGRATS! You can now start relaying over the new path!

---

[[TROUBLESHOOTING](./troubleshooting.md)]

---

<div align="center"> 

![banner](./images/github-repo-banner.gif)
 </div>

[<-- Home](../README.md) - [Advanced Usage -->](./advanced_usage.md)