# Features

The relayer supports the following:

- creating IBC connections
- creating IBC transfer channels.
- initiating a cross chain transfer
- relaying a cross chain transfer transaction, its acknowledgement, and timeouts
- relaying from state
- relaying from streaming events
- sending an UpgradePlan proposal for an IBC breaking upgrade
- upgrading clients after a counter-party chain has performed an upgrade for IBC breaking changes
- fetching canonical chain and path metadata from the GitHub repo to quickly bootstrap a relayer instance
- automatically keep clients alive on low traffic paths
- scrape metrics via a Prometheus endpoint 

The relayer currently cannot:

- create clients with user chosen parameters (such as UpgradePath)
- submit IBC client unfreezing proposals
- monitor and submit misbehavior for clients
- connect to chains which don't implement/enable IBC


[<-- Troubleshooting](./troubleshooting.md) - [Relayer Terminology -->](./terminology.md)