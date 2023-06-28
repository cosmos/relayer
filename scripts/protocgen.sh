#!/usr/bin/env bash

set -eox pipefail

echo "Generating gogo proto code"
proto_dirs=$(find ./proto -path -prune -o -name '*.proto' -print0 | xargs -0 -n1 dirname | sort | uniq)
for dir in $proto_dirs; do
  for file in $(find "${dir}" -maxdepth 1 -name '*.proto'); do
    # this regex checks if a proto file has its go_package set to cosmossdk.io/api/...
    # gogo proto files SHOULD ONLY be generated if this is false
    # we don't want gogo proto to run for proto files which are natively built for google.golang.org/protobuf
    #if grep -q "option go_package" "$file" && grep -H -o -c 'option go_package.*cosmossdk.io/api' "$file" | grep -q ':0$'; then
      buf generate --template proto/buf.gen.gogo.yaml $file
    #fi
  done
done

buf generate --template proto/buf.gen.penumbra.yaml buf.build/penumbra-zone/penumbra

# move proto files to the right places

#
# Note: Proto files are suffixed with the current binary version.
rm -r github.com/cosmos/relayer/v2/relayer/chains/penumbra/client
rm -r github.com/cosmos/relayer/v2/relayer/chains/penumbra/narsil
cp -r github.com/cosmos/relayer/v2/* ./
cp -r github.com/cosmos/relayer/relayer/* relayer/
rm -rf github.com
