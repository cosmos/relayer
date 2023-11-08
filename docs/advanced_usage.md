# Advanced Usage

## Monitoring

**Prometheus exporter**

If you started `rly` with the default `--debug-addr` argument,
you can use `http://$IP:5183/relayer/metrics` as a target for your prometheus scraper.


Exported metrics:

|              **Exported Metric**              	|                                                                                                        **Description**                                                                                                       	| **Type** 	|
|:---------------------------------------------:	|:----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------:	|:--------:	|
| cosmos_relayer_observed_packets               	| The total number of observed packets                                                                                                                                                                                         	|  Counter 	|
| cosmos_relayer_relayed_packets                	| The total number of relayed packets                                                                                                                                                                                          	|  Counter 	|
| cosmos_relayer_chain_latest_height            	| The current height of the chain                                                                                                                                                                                              	|   Gauge  	|
| cosmos_relayer_wallet_balance                 	| The current balance for the relayer's wallet                                                                                                                                                                                 	|   Gauge  	|
| cosmos_relayer_fees_spent                     	| The amount of fees spent from the relayer's wallet                                                                                                                                                                           	|   Gauge  	|
| cosmos_relayer_tx_failure                     	| <br>The total number of tx failures broken up into categories:<br> - "packet messages are redundant"<br> - "insufficient funds"<br> - "invalid coins"<br> - "out of gas"<br><br><br>"Tx Failure" is the the catch all bucket 	|  Counter 	|
| cosmos_relayer_block_query_errors_total       	| The total number of block query failures. The failures are separated into two categories:<br> - "RPC Client"<br> - "IBC Header"                                                                                              	| Counter  	|
| cosmos_relayer_client_expiration_seconds      	| Seconds until the client expires                                                                                                                                                                                             	| Gauge    	|
| cosmos_relayer_client_trusting_period_seconds 	| The trusting period (in seconds) of the client                                                                                                                                                                               	| Gauge    	|



---

## Auto Update Light Client

By default, the Relayer will automatically update clients (`MsgUpdateClient`) if the client has <= 1/3 of its trusting period left. 

> NOTE: The trusting period of the corresponding client is restored with each transaction a relayer relays. In other words, every time a relayer relays a message, it also sends a `MsgUpdateClient` message restarting the time to the clients expiration.*

> This auto-update functionality is specifically useful on low trafficked paths where messages aren't regularly being relayed.


Alternatively, you can choose to update clients more frequently by using the `--time-threshold` flag when running the `rly start` command.

Example:

- Say... You are relaying on a path that has a client trusting period of 9 minutes.
- If no messages are sent for 6 minutes and the client is 3 minutes (1/3) to expiration, the relayer will automatically update the client.
- If you wish to update the client more frequently, say anytime two minutes have passed without a `MsgUpdateClient` being sent, use flag: `--time-threshold 2m`

Selecting a time-threshold that is greater than 2/3 of the client trusting period will deem itself useless.

Use cases for configuring the `--time-threshold` flag:
- The underlying chain node that the relayer is using as an endpoint has restrictive pruning. Client updates are needed more frequently since states 2/3 trusting period ago would not be available due to pruning.  
- Mitiage relayer operational errors allowing more frequent updates incase a relayer node goes down for > the client trusting period.

\* It is not mandatory for relayers to include the `MsgUpdateClient` when relaying packets, however most, if not all relayers currently do.

## Feegrants

Feegrant configurations can be applied to each chain in the relayer. Note that Osmosis does not support Feegrants.

 - When feegrants are enabled, TXs will be signed in round robin by the grantees.
 - Feegrants reduce sequencing error rates by using many signing addresses instead of a single signer, especially when broadcast-mode is set to single.
 - Feegrants are especially useful when relaying on multiple paths with the same wallet.
 - Funds are held on a single address, the "granter".

For example, configure feegrants for Kujira:
- `rly chains configure feegrant basicallowance kujira default --num-grantees 10`
- Note: above, `default` is the key that will need to contain funds (the granter)
- 10 grantees will be configured, so those 10 address will sign TXs in round robin order.


You may also choose to specify the exact names of your grantees:
- `rly chains configure feegrant basicallowance kujira default --grantees "kuji1,kuji2,kuji3"`

Rerunning the feegrant command will simply confirm your configuration is correct, e.g. "Valid grant found for granter `addr` and grantee `addr2`" but will not create additional TXs on chain. Rerunning the feegrant command can therefore be a good way to check what addresses exist.


To remove the feegrant configuration:
- `rly chains configure feegrant basicallowance kujira --delete`


## Stuck Packet

There can be scenarios where a standard flush fails to clear a packet due to differences in the way packets are observed. The standard flush depends on the packet queries working properly. Sometimes the packet queries can miss things that the block scanning performed by the relayer during standard operation wouldn't. For packets affected by this, if they were emitted in recent blocks, the `--block-history` flag can be used to have the standard relayer block scanning start at a block height that many blocks behind the current chain tip. However, if the stuck packet occurred at an old height, farther back than would be reasonable for the `--block-history` scan from historical to current, there is an additional set of flags that can be used to zoom in on the block heights where the stuck packet occurred.

For example, say a relayer is configured between Chain A and B. The relayer was not operational during the time a user on Chain A sends a packet to Chain B. Due to an issue in the queries to Chain A, the typical flush of the relayer does not relay the packet. Say that many days go by before recognition of the issue by the relayer operator. The relayer operator could start up the relayer with a massive `--block-history` to query all blocks from the time of the stuck packet until the current block, but that could take many hours to query through each block. Instead, the relayer operator can flush out the packet by doing the following:

```bash
rly start $PATH_NAME --stuck-packet-chain-id $CHAIN_A_CHAIN_ID --stuck-packet-height-start $CHAIN_A_STUCK_PACKET_HEIGHT --stuck-packet-height-end $CHAIN_A_STUCK_PACKET_HEIGHT -d
```

Alternatively, a flush can be run with these flags so that the relayer exits once it is done:

```bash
rly tx flush $PATH_NAME --stuck-packet-chain-id $CHAIN_A_CHAIN_ID --stuck-packet-height-start $CHAIN_A_STUCK_PACKET_HEIGHT --stuck-packet-height-end $CHAIN_A_STUCK_PACKET_HEIGHT -d
```

If the `CHAIN_A_STUCK_PACKET_HEIGHT` is not exactly known, the `stuck-packet-height-start` and `stuck-packet-height-end` flags can be placed at heights surrounding the range where the stuck packet is expected to be, for convenience of not needing to dig through every block to determine the exact height.

Note that this narrows the window of visibility that the relayer has into what has happened on the chain, since the relayer is only getting a picture of what happened between `stuck-packet-height-start` and `stuck-packet-height-end` and then starts observing the most recent blocks after that. If a packet was actually relayed properly in between `stuck-packet-height-end` and the chain tip, then the relayer would encounter errors trying to relay a packet that was already relayed. This feature should only be used by advanced users for zooming in on a troublesome packet.

---

[<-- Create Path Across Chains](create-path-across-chain.md) - [Troubleshooting -->](./troubleshooting.md)
