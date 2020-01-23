# Relayer

The `relayer` package contains some basic relayer implementations that are
meant to be used by users wishing to relay packets between IBC enabled chains.
It is also well documented and intended as a place where users who are
interested in building their own relayer can come for working examples.

### Notes:

- Relayer can manage keys with the test keybase backend currently. It should be
  easy to switch this out for another keybase in the future. Have a meeting
  with @alessio to talk about details here.
    * When running (`relayer start`) Relayer needs to be able to use the keys
      each `Chain` configured in the `~/.relayer/config/config.yaml`
    * Relayers can be kept off the open internet as there is no need to send
      them messages, this should help improve the security of the relayer even
      though it must keep "hot" keys.
- Relayer should have ability to do CRUD on each of the lite clients it manages
 (e.g. `relayer lite update <chain-id>`). The groundwork is laid for this, just
 needs to be hooked up a bit more. 
    * Q: Should we have commands to manage the lite clients independent of the
      `relay start` command?
- Relayer should have ability to list the chains/connection it is managing
  (`relayer chains list`)
    * TODO
- Relayers should gracefully handle `ErrInsufficentFunds` when/if accounts run
  out of funds
    * _Stretch_: Relayer should notify a configurable endpoint when it hits
      `ErrInsufficentFunds`
- Many of the messages in the `relayer/strategies.go` file are not implemented.
  This work needs to be completed according to the relayer spec.
- The packet transfer section of the `naive` algo is currently worse than a
  stub and will need to wait on the completion of ADR 15.

### Open Questions

- Do we want to force users to name their `ibc.Client`s, `ibc.Connection`s,
 `ibc.Channel`s and `ibc.Port`s? Can we use randomly generated identifiers
 instead?
