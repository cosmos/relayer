#/bin/bash -e

# Ensure jq is installed
if [[ ! -x "$(which jq)" ]]; then
  echo "jq (a tool for parsing json in the command line) is required..."
  echo "https://stedolan.github.io/jq/download/"
  exit 1
fi

RELAYER_DIR="$GOPATH/src/github.com/cosmos/relayer"
RELAYER_CONF="$HOME/.relayer"
GAIA_CONF="$RELAYER_DIR/data"

# Ensure user understands what will be deleted
if ([[ -d $RELAYER_CONF ]] || [[ -d $GAIA_CONF ]]) && [[ ! "$1" == "skip" ]]; then
  read -p "$0 will delete $RELAYER_CONF and $GAIA_CONF folder. Do you wish to continue? (y/n): " -n 1 -r
  echo
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
      exit 1
  fi
fi

cd $RELAYER_DIR
rm -rf $RELAYER_CONF &> /dev/null
bash two-chains.sh "local" "skip"


echo "Building Relayer..."
make install

echo "Generating relayer configurations..."
relayer config init
relayer chains add -f demo/ibc0.json
relayer chains add -f demo/ibc1.json
relayer paths add ibc0 ibc1 -f demo/path.json

SEED0=$(jq -r '.secret' $GAIA_CONF/ibc0/n0/gaiacli/key_seed.json)
SEED1=$(jq -r '.secret' $GAIA_CONF/ibc1/n0/gaiacli/key_seed.json)
echo 
echo "Key $(relayer keys restore ibc0 testkey "$SEED0" -a) imported from ibc0 to relayer..."
echo "Key $(relayer keys restore ibc1 testkey "$SEED1" -a) imported from ibc1 to relayer..."
echo
echo "Creating configured path between ibc0 and ibc1..."
echo
sleep 8
relayer lite init ibc0 -f
relayer lite init ibc1 -f
sleep 5
relayer tx full-path ibc0 ibc1
