# Create Path Across Chains

In our "Relaying Packets Across Chains" example, we set up the relayer to relay between Cosmoshub and Osmosis. Since those two chains already have a path opened between them, we will use two mock chains: ibc-0 and ibc-1.

1. **Add basic path info to config.**

    ```shell
    # rly paths new [src-chain-id] [dst-chain-id] [path-name]

    $ rly paths new ibc-0 ibc-1 my_demo_path
    ```

2. **Next we need to create a channel, client, and connection.**

    The most efficient way to use this is to use the `rly transaction link` command.

    ```shell
    $ rly transact link my_demo_path
    ```

    This is a triplewammy, it creates a `client`, `connection`, and `channel` between the two chains. 

    If you would like more control, you can run each command individually:

    - `rly transact clients`
    - `rly transact connection`
    - `rly transact channel`

    
    
    IF YOU GET: 
    ```
    Error: error creating clients: failed to create client on src chain{ibc-0}: failed to send messages on chain{ibc-0}: rpc error: code = InvalidArgument desc = latest height revision number must match chain id revision number (0 != 1): invalid header height: invalid request
    ```
    -> Need to use --override flag

---

<div align="center"> 

![banner](./images/github-repo-banner.gif)
 </div>

[<-- Home](../README.md) [Troubleshooting -->](./terminology.md)


<!--  final somm package will only update fonts  -->