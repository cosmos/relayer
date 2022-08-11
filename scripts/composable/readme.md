## Setup

### Download binaries
```
cd bin 

wget https://github.com/galacticcouncil/Basilisk-node/releases/download/v7.0.0/basilisk
wget https://storage.googleapis.com/substrate-ibc-composable/composable
wget https://github.com/paritytech/polkadot/releases/download/v0.9.19/polkadot
```

### Setup permissions
```
chmod +x ./bin/composable
chmod +x ./bin/basilisk
chmod +x ./bin/polkadot
```

### Install polkadot-launch deps
`yarn`

### Start local chain
`./node_modules/.bin/polkadot-launch ./composable_and_basilisk.json`

`"rpc-addr": "ws://34.125.166.155:9944",`
