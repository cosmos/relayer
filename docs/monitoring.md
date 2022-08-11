# Monitoring

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
