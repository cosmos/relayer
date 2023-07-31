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


Alternitavely, you can choose to update clients more frequently by using the `--time-threshold` flag when running the `rly start` command.

Example:

- Say... You are relaying on a path that has a client trusting period of 9 minutes.
- If no messages are sent for 6 minutes and the client is 3 minutes (1/3) to expiration, the relayer will automatically update the client.
- If you wish to update the client more frequently, say anytime two minutes have passed without a `MsgUpdateClient` being sent, use flag: `--time-threshold 2m`

Selecting a time-threshold that is greater than 2/3 of the client trusting period will deem itself useless.

Use cases for configuring the `--time-threshold` flag:
- The underlying chain node that the relayer is using as an endpoint has restrictive pruning. Client updates are needed more frequently since states 2/3 trusting period ago would not be available due to pruning.  
- Mitiage relayer operational errors allowing more frequent updates incase a relayer node goes down for > the client trusting period.

\* It is not mandatory for relayers to include the `MsgUpdateClient` when relaying packets, however most, if not all relayers currently do.

---


[<-- Create Path Across Chains](create-path-across-chain.md) - [Troubleshooting -->](./troubleshooting.md)
