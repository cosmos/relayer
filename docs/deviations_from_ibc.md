# Deviations from IBC Relay

This relayer was meant to be used for [ICON-IBC Integration](https://github.com/icon-project/IBC-Integration). So, it is made to accomodate all the changes made on IBC Integration such that IBC can be established between ICON and Cosmos based chains.

All the deviations that the ICON-IBC Integrations can be found [here]().

## Changes 
The major changes in the relayer include: 
- [ICON Module](#icon-module)
- [Wasm Module](#wasm-module)
- [Relayer Internal](#relayer-internal)
- [Cmd](#cmd)
- [Config](#config)

## Icon Module
- Icon Chain Module has been added to support ICON Chain.
- We use [BTP Blocks](https://icon.community/glossary/btp-blocks/) provided by ICON chain significantly.
- Icon provides a websocket api which we can use to stream all blocks of mainnet. We do use this websockets instead of polling each height as done on cosmos chain processor.
- The relayer interfaces with IBC contract deployed on ICON and calls methods of the contract via ICON Provider.
- To update counterparty light client, we use information obtained BTP Blocks rather than actual main ICON chain.
- We query if the block is a BTP Block, and if yes, it initiates messages that'll be sent to counterparty contract. So, for each BTP Block produced, light client is updated.

## Wasm Module
- Wasm Module has been added to support CosmWasm contracts.
- Like cosmos chain processor, polling is done for each height of the chain.
- Wasm Provider provides methods to interface with the IBC Wasm contract deployed on respective cosmos chain.

## Relayer Internal
- Minimal changes has had to be added on the internal logic of the relayer to support for BTP Blocks.
- Since ICON cannot produce non membership proofs via BTP Blocks, a method called `requestTimeout` was introduced on ICON contract for when packet is to be timed out on wasm contracts. Logic for this has been added on relayer internal.
- Update client and the message to be sent after, has been sent on different transaction. [Reason for this decision]()

## Cmd
- Relevant methods for ICON has been added accordingly.
- When a BTP client is to be created on WASM contract, we need to specify a BTP height for the relayer. By default, it takes the first BTP height for this network. If we are to give a custom height, a flag `btp-block-height` has been added. The height should be a BTP Block for given BTP network type and BTP Network Id.
    ```sh
    rly tx clients icon-path-name --client-tp "1000000m"  --btp-block-height 11313986
    ```

## Config
- Example Config for ICON-IBC Integration
    - [Here](../examples/config_IBC_ICON.yaml)



## Wasm Config
Other parameters are same as in cosmos chains. The specifications for added fields in config are
- IBC Handler Address
    - WASM Contract Address of IBC Handler
- First Retry Block After:
    - For first retry transaction, wait for this number of blocks (default set to 3)
- Start Height:
    - Custom height to start chain processor from
- Block Interval:
    - Block Interval of cosmos chain

## Icon Config
- keystore and password
    - Keystore and password is used to generate a wallet for relay
- Icon Network ID
    - Network ID for ICON chain (varies for testnet, localnet and mainnet)
- BTP Network ID, BTP Network Type ID
    - BTP Specific IDs required 
- Start Height:
    - Height to start ICON chain processor from
- IBC Handler Address:
    - Address of IBC java contract
- Block Interval:
    - Block Interval of ICON chain
- First Retry Block After:
    - For first retry transaction, wait for this number of blocks (default set to 8)
