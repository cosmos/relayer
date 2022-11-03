# Advanced Usage

## Monitoring

**Prometheus exporter**

If you started `rly` with the default `--debug-addr` argument,
you can use `http://$IP:7597/metrics` as a target for your prometheus scraper.

**Example metrics**

```
go_goroutines 29
...
go_threads 39
...
observed_packets{chain="cosmoshub-4",channel="channel-141",path="hubosmo",port="transfer",type="acknowledge_packet"} 57
observed_packets{chain="cosmoshub-4",channel="channel-141",path="hubosmo",port="transfer",type="recv_packet"} 103
observed_packets{chain="cosmoshub-4",channel="channel-141",path="hubosmo",port="transfer",type="send_packet"} 58
observed_packets{chain="osmosis-1",channel="channel-0",path="hubosmo",port="transfer",type="acknowledge_packet"} 107
observed_packets{chain="osmosis-1",channel="channel-0",path="hubosmo",port="transfer",type="recv_packet"} 60
observed_packets{chain="osmosis-1",channel="channel-0",path="hubosmo",port="transfer",type="send_packet"} 102
...
relayed_packets{chain="cosmoshub-4",channel="channel-141",path="hubosmo",port="transfer",type="acknowledge_packet"} 31
relayed_packets{chain="cosmoshub-4",channel="channel-141",path="hubosmo",port="transfer",type="recv_packet"} 65
relayed_packets{chain="osmosis-1",channel="channel-0",path="hubosmo",port="transfer",type="acknowledge_packet"} 36
relayed_packets{chain="osmosis-1",channel="channel-0",path="hubosmo",port="transfer",type="recv_packet"} 35
```

---

## Auto Update Light Client

By default, the Relayer will automatically update clients (`MsgUpdateClient`) if the client has <= 1/3 of its trusting period left. 

> NOTE: The trusting period of the corresponding client is restored with each transaction a relayer relays. In other words, every time a relayer relays a message, it also sends a `MsgUpdateClient` message restarting the time to the clients expiration.*

> This auto-update functionality is specifically useful on low trafficked paths where messages aren't regularly being relayed.


You can choose to update clients more regularly by using the `--time-threshold` flag when running the `rly start` command.

Example:

- You are relaying on a path that has a client trusting period of 9 minutes.
- If no messages are sent for 6 minutes and the client is 3 minutes (1/3) to expiration, the relayer will automatically update the client.
- If you wish to update the client more frequently, say anytime two minutes have passed without a `MsgUpdateClient` being sent, use flag: `--time-threshold 2m`

Selecting a time-threshold that is greater than 2/3 of the client trusting period will deem itself useless.

\* It is not mandatory for relayers to include the `MsgUpdateClient` when relaying packets, however most, if not all relayers currently do.

---


[<-- Create Path Across Chains](create-path-across-chain.md) - [Troubleshooting -->](./troubleshooting.md)