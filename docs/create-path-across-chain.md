# Create Path Across Chains

In our "Relaying Packets Across Chains" example, we set up the relayer to relay between Cosmoshub and Osmois. Since those two chains already have a path opened between them, we will use two mock chains: ibc-0 and ibc-1.

1. Add basic path info to config.

    ```shell
    # rly paths new [src-chain-id] [dst-chain-id] [path-name]

    & rly paths new ibc-0 ibc-1 my_demo_path
    ```

2. Next we need to create a channel, client, and connection.

    The most effecient way to use this is to use the `rly transaction link` command.

    IF YOU GET: 
    ```
    Error: error creating clients: failed to create client on src chain{ibc-0}: failed to send messages on chain{ibc-0}: rpc error: code = InvalidArgument desc = latest height revision number must match chain id revision number (0 != 1): invalid header height: invalid request
    ```
    -> Need to use --override flag

---

<div align="center"> 

![banner](./images/github-repo-banner.gif)
 </div>

 <div style="text-align: right"> <a href="./README.md"><-- Home  </a> <a href="./docs/troubleshooting.md">  Troubleshooting --></a> </div>