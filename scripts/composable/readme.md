## Setup

### Download binaries
```
cd bin 

wget https://github.com/galacticcouncil/Basilisk-node/releases/download/v7.0.0/basilisk
wget https://github.com/ComposableFi/composable/releases/download/v2.1.6/composable
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