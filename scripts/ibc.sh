#!/bin/bash

rm -rf ~/.relayer/*

rly config init

rly chains add --file artela.json artela_11820-1

rly chains add --file evmos.json evmos_9000-1

rly paths add artela_11820-1 evmos_9000-1 artela_evmos --file ./artela_evmos.json

#rly keys add artela_11820-1 yuan

#rly keys add evmos_9000-1 jerry

rly keys restore artela_11820-1 yuan "either vessel liquid divide decide aspect ridge evidence iron diary unknown brain time kite spring guide diary theory beyond mandate cushion minimum peasant tribe"

rly keys restore evmos_9000-1 jerry "reunion someone human found fluid message session rent glimpse awkward frown radar margin hamster emotion type claim coral hospital pottery sustain shrug person travel"

rly transact connection artela_evmos

#rly tx link artela_evmos --src-port transfer --dst-port transfer --order unordered --version ics20-1 -d
