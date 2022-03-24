# Recommended Pruning Settings

The relayer relies on old headers and proofs constructed at past block heights
to facilitate correct [IBC](https://ibcprotocol.org/) behavior. For this reason,
connected full nodes may prune old blocks once they have passed the unbonding
period of the chain but not before. Not pruning at all is not necessary for a
fully functional relayer, however, pruning everything will lead to many issues!

Here are the settings used to configure SDK-based full nodes (assuming 3 week unbonding period):

```shell
... --pruning=custom --pruning-keep-recent=362880 --pruning-keep-every=0 --pruning-interval=100
```

`362880 (3*7*24*60*60 / 5 = 362880)` represents a 3 week unbonding period (assuming 5 seconds per block).

Note, operators can tweak `--pruning-keep-every` and `--pruning-interval` to their
liking.

[<-- Relayer Terminology](./terminology.md) - [Demo -->](./demo.md)