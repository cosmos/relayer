#!/bin/bash

#from container environment vars
SLEEPTIME=${SLEEPTIME:-5} # if unset, set to 5 seconds
SLEEPTIME_IBC=${SLEEPTIME_IBC:-60} # Amount of time to wait for an IBC channel to be created
SLEEPTIME_BLOCKWAIT=${SLEEPTIME_BLOCKWAIT:-1}

PROVIDER_CHAIN=${PROVIDER_CHAIN:-provider}
CONSUMER_CHAIN=${CONSUMER_CHAIN:-consumer}
PROVIDER_RLY_GASPRICE=${PROVIDER_RLY_GASPRICE:-0stake}
CONSUMER_RLY_GASPRICE=${CONSUMER_RLY_GASPRICE:-0stake}
PROVIDER_RLY_CLIENTID=${PROVIDER_RLY_CLIENTID:-07-tendermint-0}
CONSUMER_RLY_CLIENTID=${CONSUMER_RLY_CLIENTID:-07-tendermint-0}
RLY_SRC_PORT=${RLY_SRC_PORT:-transfer}
RLY_DST_PORT=${RLY_DST_PORT:-transfer}
RLY_ORDERING=${RLY_ORDERING:-unordered}
RLY_CHANNEL_VERSION=${RLY_CHANNEL_VERSION:-1}
RLY_DEBUG=${RLY_DEBUG:-false}

KEYRING=${KEYRING:-file}
TESTKEYRING="test"
KEYALGO=${KEYALGO:-"secp256k1"}
KEYNAME=relayer
KEYPASSWD=${KEYPASSWD:-"passw0rdK3y"}

SESSION_STAMP=RLY_`date +%m%d%Y%H%M%S`
TMPDIR=/tmp
LOGDIR=/tmp
LOGFILE=${LOGDIR}/${SESSION_STAMP}.log
ERRFILE=${LOGDIR}/${SESSION_STAMP}.err

Logger()
{
	MSG=$1
	echo "`date` $MSG" >> $LOGFILE
	echo "`date` $MSG"
}

CheckRetcode()
{
	# ERRTYPE 1 = HARD ERROR (Exit script), ERRTYPE <> 1 = SOFT ERROR (Report and Continue)
	local RETCODE=$1
	local ERRTYPE=$2
	local MSG=$3
	if [ $RETCODE -ne 0 ];
	then
		if [ $ERRTYPE -eq 1 ];
		then
			Logger "$MSG"
			exit 1
		else
			Logger "$MSG"
		fi
	else
		Logger "Return code was $RETCODE. Success!"
	fi
}

ValidateEnvVar()
{
  local ENVVAR=$1
  Logger "Validating environment variable $ENVVAR"
  local EXITIFUNSET=${2:-1}  # exit if env var is not set. Pass 1 for true, 0 for false i.e. if 0, script will continue executing. Default: True (exit)
  local ECHOVAL=${3:-1} # echo the value of the variable in a log entry. Pass 1 = true, 0 = false. Default: True (will echo)
  if [[ -z ${!ENVVAR} ]];
  then
    Logger "Environment variable $ENVVAR is not set"
    if [ $EXITIFUNSET -eq 1 ];
    then
      Logger "Exiting in error as environment variable $ENVVAR is not set"
      exit 1
    else
      Logger "Continuing even though environment variable $ENVVAR is not set"
    fi
  fi
  if [ $ECHOVAL -eq 1 ];
  then
    Logger "Environment variable $ENVVAR is set to ${!ENVVAR}"
  fi
  Logger "Finished validating environment variable $ENVVAR"
}

ValidateAndEchoEnvVars()
{
  Logger "Starting function ValidateAndEchoEnvVars"
  ValidateEnvVar PROVIDER_CHAIN
  ValidateEnvVar CONSUMER_CHAIN
  ValidateEnvVar PROVIDER_CHAINID
  ValidateEnvVar CONSUMER_CHAINID
  ValidateEnvVar PROVIDER_RLY_MNEMONIC 1 0
  ValidateEnvVar CONSUMER_RLY_MNEMONIC 1 0
  ValidateEnvVar PROVIDER_RPC_ADDRESS
  ValidateEnvVar CONSUMER_RPC_ADDRESS
  ValidateEnvVar PROVIDER_RLY_GASPRICE
  ValidateEnvVar CONSUMER_RLY_GASPRICE
  ValidateEnvVar PROVIDER_RLY_CLIENTID
  ValidateEnvVar CONSUMER_RLY_CLIENTID
  ValidateEnvVar RLY_SRC_PORT
  ValidateEnvVar RLY_DST_PORT
  ValidateEnvVar RLY_ORDERING
  ValidateEnvVar RLY_CHANNEL_VERSION
  ValidateEnvVar RLY_DEBUG
  ValidateEnvVar KEYRING
  ValidateEnvVar SLEEPTIME 0 1
  ValidateEnvVar KEYALGO
  ValidateEnvVar KEYPASSWD
  ValidateEnvVar TMPDIR
  Logger "Exiting function ValidateAndEchoEnvVars"
}

InitRelayer()
{
    Logger "Starting function InitRelayer"
    rm -rf .relayer 1>> $LOGFILE 2>> $ERRFILE
    rly config init --home .relayer 1>> $LOGFILE 2>> $ERRFILE
    RETCODE=$?
    CheckRetcode $RETCODE 1 "Could not initialize relayer. Return code was $RETCODE. Exiting"
    Logger "Exiting function InitRelayer"
}

GenerateChainFiles()
{
    Logger "Starting function GenerateChainFiles"
    jq --arg KEY $KEYNAME '.value.key = $KEY' /root/provider-rly.json > /root/provider-rly-tmp.json && mv /root/provider-rly-tmp.json /root/provider-rly.json
    jq --arg CHAINID $PROVIDER_CHAINID '.value."chain-id" = $CHAINID' /root/provider-rly.json > /root/provider-rly-tmp.json && mv /root/provider-rly-tmp.json /root/provider-rly.json
    jq --arg RPCADDR $PROVIDER_RPC_ADDRESS '.value."rpc-addr" = $RPCADDR' /root/provider-rly.json > /root/provider-rly-tmp.json && mv /root/provider-rly-tmp.json /root/provider-rly.json
    jq --arg KEYRING $KEYRING '.value."keyring-backend" = $KEYRING' /root/provider-rly.json > /root/provider-rly-tmp.json && mv /root/provider-rly-tmp.json /root/provider-rly.json
    jq --argjson DEBUG $RLY_DEBUG '.value.debug = $DEBUG' /root/provider-rly.json > /root/provider-rly-tmp.json && mv /root/provider-rly-tmp.json /root/provider-rly.json
    jq --arg GAS $PROVIDER_RLY_GASPRICE '.value."gas-prices" = $GAS' /root/provider-rly.json > /root/provider-rly-tmp.json && mv /root/provider-rly-tmp.json /root/provider-rly.json

    jq --arg KEY $KEYNAME '.value.key = $KEY' /root/consumer-rly.json > /root/consumer-rly-tmp.json && mv /root/consumer-rly-tmp.json /root/consumer-rly.json
    jq --arg CHAINID $CONSUMER_CHAINID '.value."chain-id" = $CHAINID' /root/consumer-rly.json > /root/consumer-rly-tmp.json && mv /root/consumer-rly-tmp.json /root/consumer-rly.json
    jq --arg RPCADDR $CONSUMER_RPC_ADDRESS '.value."rpc-addr" = $RPCADDR' /root/consumer-rly.json > /root/consumer-rly-tmp.json && mv /root/consumer-rly-tmp.json /root/consumer-rly.json
    jq --arg KEYRING $KEYRING '.value."keyring-backend" = $KEYRING' /root/consumer-rly.json > /root/consumer-rly-tmp.json && mv /root/consumer-rly-tmp.json /root/consumer-rly.json
    jq --argjson DEBUG $RLY_DEBUG '.value.debug = $DEBUG' /root/consumer-rly.json > /root/consumer-rly-tmp.json && mv /root/consumer-rly-tmp.json /root/consumer-rly.json
    jq --arg GAS $CONSUMER_RLY_GASPRICE '.value."gas-prices" = $GAS' /root/consumer-rly.json > /root/consumer-rly-tmp.json && mv /root/consumer-rly-tmp.json /root/consumer-rly.json
    Logger "Exiting function GenerateChainFiles"
}

ConfigRelayer()
{
    Logger "Starting function ConfigRelayer"
    local PATHNAME=pc
    rly chains add $PROVIDER_CHAIN --file /root/provider-rly.json --home .relayer #1>> $LOGFILE 2>> $ERRFILE
    RETCODE=$?
    CheckRetcode $RETCODE 1 "Could not add chain $PROVIDER_CHAINID to relayer config. Return code was $RETCODE. Exiting"
    rly chains add $CONSUMER_CHAIN --file /root/consumer-rly.json --home .relayer #1>> $LOGFILE 2>> $ERRFILE
    RETCODE=$?
    CheckRetcode $RETCODE 1 "Could not add chain $CONSUMER_CHAINID to relayer config. Return code was $RETCODE. Exiting"
    Logger "Added both provider and consumer chains"
    Logger "Restoring keys from provided mnemonics"
    if [[ "$KEYRING" == "file" ]];
    then
        (echo $KEYPASSWD; echo $KEYPASSWD) | rly keys restore $PROVIDER_CHAIN relayer "${PROVIDER_RLY_MNEMONIC}" --home .relayer #1>> $LOGFILE 2>> $ERRFILE
        RETCODE=$?
        CheckRetcode $RETCODE 1 "Could not restore keys from mnemonic for chain $PROVIDER_CHAINID. Return code was $RETCODE. Exiting"
        (echo $KEYPASSWD; echo $KEYPASSWD) | rly keys restore $CONSUMER_CHAIN relayer "${CONSUMER_RLY_MNEMONIC}" --home .relayer #1>> $LOGFILE 2>> $ERRFILE
        RETCODE=$?
        CheckRetcode $RETCODE 1 "Could not restore keys from mnemonic for chain $CONSUMER_CHAINID. Return code was $RETCODE. Exiting"
    else
        rly keys restore $PROVIDER_CHAIN relayer "${PROVIDER_RLY_MNEMONIC}" --home .relayer #1>> $LOGFILE 2>> $ERRFILE
        RETCODE=$?
        CheckRetcode $RETCODE 1 "Could not restore keys from mnemonic for chain $PROVIDER_CHAINID. Return code was $RETCODE. Exiting"
        rly keys restore $CONSUMER_CHAIN relayer "${CONSUMER_RLY_MNEMONIC}" --home .relayer #1>> $LOGFILE 2>> $ERRFILE
        RETCODE=$?
        CheckRetcode $RETCODE 1 "Could not restore keys from mnemonic for chain $CONSUMER_CHAINID. Return code was $RETCODE. Exiting"        
    fi
    Logger "Created keys"
    Logger "Creating relayer paths..."
    rly paths new $CONSUMER_CHAINID $PROVIDER_CHAINID $PATHNAME --home .relayer #1>> $LOGFILE 2>> $ERRFILE
    RETCODE=$?
    CheckRetcode $RETCODE 1 "Could not create a new path for chains $PROVIDER_CHAINID and $CONSUMER_CHAINID. Return code was $RETCODE. Exiting"
    Logger "New path $PATHNAME successfully created for chains $PROVIDER_CHAINID and $CONSUMER_CHAINID"
    rly paths update $PATHNAME --src-client-id $CONSUMER_RLY_CLIENTID --dst-client-id $PROVIDER_RLY_CLIENTID --home .relayer #1>> $LOGFILE 2>> $ERRFILE
    RETCODE=$?
    CheckRetcode $RETCODE 1 "Could not update the path with source/destination client-id for chains $PROVIDER_CHAINID and $CONSUMER_CHAINID. Return code was $RETCODE. Exiting"
    Logger "Path $PATHNAME successfully updated for chains $PROVIDER_CHAINID and $CONSUMER_CHAINID"
    Logger "Exiting function ConfigRelayer"
}

LinkRelayer()
{
    Logger "Starting function LinkRelayer"
    # Logger "Debug sleep"
    # sleep 600
    Logger "Now connecting $PROVIDER_CHAIN and $CONSUMER_CHAIN"
    (echo $KEYPASSWD; sleep 1; echo $KEYPASSWD) | rly transact link pc --home .relayer --src-port $RLY_SRC_PORT --dst-port $RLY_DST_PORT --order $RLY_ORDERING --version $RLY_CHANNEL_VERSION #1>> $LOGFILE 2>> $ERRFILE
    RETCODE=$?
    CheckRetcode $RETCODE 1 "Could not create a connection between chains $PROVIDER_CHAINID and $CONSUMER_CHAINID. Return code was $RETCODE. Exiting"
    Logger "Chains $PROVIDER_CHAINID and $CONSUMER_CHAINID successfully connected"
    Logger "Exiting function LinkRelayer"
}

CheckLaunchReadiness()
{
    Logger "Starting function CheckLaunchReadiness"
    # we want to make sure that chainlet is up and running
    while true
    do
        rly q node-state $PROVIDER_CHAIN --home .relayer #1>> $LOGFILE 2>> $ERRFILE
        RETCODEP=$?
        rly q node-state $CONSUMER_CHAIN --home .relayer #1>> $LOGFILE 2>> $ERRFILE
        RETCODEC=$?
        if [[ ${RETCODEP} -eq 0 && ${RETCODEC} -eq 0 ]]; then
            break
        fi
        Logger "DEBUG Provider chain $PROVIDER_CHAINID shows $RETCODEP and consumer chain $CONSUMER_CHAINID shows $RETCODEC"
        Logger "Waiting for provider chain $PROVIDER_CHAINID and consumer chain $CONSUMER_CHAINID to come online"
        sleep $SLEEPTIME
    done
    Logger "Both provider and consumer chains are online. Continuing"
    Logger "Exiting function CheckLaunchReadiness"
}


## MAIN
ValidateAndEchoEnvVars
InitRelayer
GenerateChainFiles
ConfigRelayer
CheckLaunchReadiness
LinkRelayer
rly start pc --home .relayer
