# Relayer Terminology

A `path` represents an abstraction between two IBC-connected networks. Specifically,
the `path` abstraction contains metadata about a source chain, a destination
chain and a relaying strategy between the two networks. The metadata for both
the source and destination networks contains the following:

- `chain-id`: The chain ID of the network.
- `client-id`: The client ID on the corresponding chain representing the other chain's light client.
- `connection-id`: The connection ID on the corresponding chain representing a connection to the other chain.
- `channel-id`: The channel ID on the corresponding chain's connection representing a channel on the other chain.
- `port-id`: The IBC port ID which a relevant module binds to on the corresponding chain.
- `order`: Determines if packets from a sending module must be `ORDERED` or `UNORDERED`.
- `version`: IBC version.

Two chains may have many different paths between them. Any path with different
clients, connections, or channels are considered uniquely different and non-fungible.

When using with live networks, it is advised to pre-select your desired parameters
for your clients, connections, and channels. The relayer will automatically
reuse any existing clients that match your configurations since clients,
connections, and channels are public goods (no one has control over them).

[<-- Features](./features.md) - [Pruning Settings -->](./node_pruning.md)