# Testing

The relayer contains a testing framework designed to be used to test compatibility
between different cosmos-sdk based chains for IBC relaying. This will be especially
useful during the period where IBC is under active development as it will provide
a central integration test to ensure that the many different implementations all
work together. It will also be required if you are looking to participate with a
custom zone in Game of Zones.

## Using the test framework

Because of the nature of the relayer (i.e. it is meant to run against different chains),
mocking out the interfaces for unit tests would be prohibitively expensive from
a resources point of view. Because of this, we've decided to go with a full
integration testing framework that tests the user-critical paths of the relayer
code for each chain to ensure compatibility between the chains and a functional
Game of Zones. To add your chain to the framework, follow the guide below:

## Overview

The test framework is built using `go test` and `docker`. What is happening for
each test is that a number of independent chains of a specified type are spun up
and the relayer runs a series of transactions and tests for the expected results.
We are using the [`ory/dockertest`](https://github.com/ory/dockertest) to provide
a nice interface for using docker programmatically within the tests.

### Step 1: Write a Dockerfile and publish an image for your chain

The testing framework expects your chain to have a `Dockerfile` with an
`ENTRYPOINT` script that accepts two arguments: `chain-id`, which should be
unique to the individual test, and `relayer-address`, an address to include in
the genesis file so that the testing relayer has access to funds. This is normally
best accomplished with an `./entrypoint.sh` script that performs the necessary
chain bootstrapping. The `cosmos/gaia` repositories provide an example of both:

- [`./entrypoint.sh`](https://github.com/cosmos/gaia/tree/master/contrib/single-node.sh)
- [`Dockerfile.test`](https://github.com/cosmos/gaia/tree/master/contrib/Dockerfile.test)

Then you need to build and push your image to a public image repository. Having
it tagged with the git sha and branch is best practice. See the build procedure
for the gaia image:

- [`Makefile`](https://github.com/cosmos/gaia/blob/master/Makefile#L164)

At the end, you should have an image you can run which starts up an instance of
your chain:

```shell
# add configuration for the yet-to-be-created chain to the relayer
rly ch add -f mychainid.json

# run the chain in a detached container with the rpc port open for calls
docker run -d -p 26657:26657 myorg/myimage:mytag mychainid $(rly ch addr mychainid)

# then, if you have a properly configured relayer, the following returns with the balance
rly q bal mychainid
```

### Step 2: Add your chain configuration to the test harness

Next you will need to define a new instance of `testChainConfig` in the
`test/test_chains.go` file. Follow the `gaiaTestConfig` example:

> NOTE: We've increased the default block timeouts for gaia to the values noted
> below. This makes the tests faster. If you would like to do the same for your
> chain see the `sed` commands in the [entrypoint](https://github.com/cosmos/gaia/tree/master/contrib/single-node.sh).

```go
// GAIA BLOCK TIMEOUTS on jackzampolin/gaiatest:jack_relayer-testing
// timeout_commit = "1000ms"
// timeout_propose = "1000ms"
// 3 second relayer timeout works well with these block times
gaiaTestConfig = testChainConfig{
    cdc:            codecstd.NewAppCodec(codecstd.MakeCodec(simapp.ModuleBasics)),
    amino:          codecstd.MakeCodec(simapp.ModuleBasics),
    dockerImage:    "jackzampolin/gaiatest",
    dockerTag:      "master",
    timeout:        3 * time.Second,
    rpcPort:        "26657",
    accountPrefix:  "cosmos",
    trustingPeriod: "330h",
}

// TODO: add your chain configuration here
// These are notes about my chain's docker image
myChainTestConfig = testChainConfig {
    ...
}
```

> NOTE: If you do any custom encoding/decoding in your chain, you may want to
> import your codec and attach it here. To do this, `go get` the package your
> codec is in, include it in the `test/test_chains.go` `import` section and
> instantiate your codec. You may run into build errors due to incompatible
> Tendermint or SDK versions. Please bring your chain up the relayer version to
> continue with a custom codec.

### Step 3: Write your tests

Now you can write tests! Create a new file named `test/relayer_{chain-type}_test.go`
and write your tests! Emulate (or copy) the `gaia` examples to start. The
framework is designed to be flexible enough to eventually allow testing of
custom functionality.

```go
var (
	gaiaChains = []testChain{
		{"ibc-0", gaiaTestConfig},
		{"ibc-1", gaiaTestConfig},
	}
)

func TestGaiaToGaiaBasicTransfer(t *testing.T) {
	t.Parallel()
	chains := spinUpTestChains(t, gaiaChains...)

	_, err := genTestPathAndSet(chains.MustGet("ibc-0"), chains.MustGet("ibc-1"), "transfer", "transfer")
	require.NoError(t, err)

	var (
		src          = chains.MustGet("ibc-0")
		dst          = chains.MustGet("ibc-1")
		testDenom    = "samoleans"
		dstDenom     = fmt.Sprintf("%s/%s/%s", dst.PathEnd.PortID, dst.PathEnd.ChannelID, testDenom)
		testCoin     = sdk.NewCoin(testDenom, sdk.NewInt(1000))
		expectedCoin = sdk.NewCoin(dstDenom, sdk.NewInt(1000))
	)

	// Check if clients have been created, if not create them
	require.NoError(t, src.CreateClients(dst))
	// Check if connection has been created, if not create it
	require.NoError(t, src.CreateConnection(dst, src.GetTimeout()))
	// Check if channel has been created, if not create it
	require.NoError(t, src.CreateChannel(dst, true, src.GetTimeout()))

	// ...and check the balance
	dstBal, err := dst.QueryBalance(dst.Key)
	require.NoError(t, err)
	require.Equal(t, expectedCoin.Amount.Int64(), dstBal.AmountOf(dstDenom).Int64())
}
```

### Step 4: Get your badge on the README

The [README](../README.md) contains a compatibility matrix that is populated with
status badges from Github Actions that shows the current status of different
implementations. If you would like to add yours to this list
(and if you have gotten this far, YOU SHOULD!!) do the following:

1. Add a `Makefile` command that just calls your chain's tests. This is made
   easy by the `-tags` flag that reads parts of the filenames of go tests.
   See the `gaia` command for an example:

   ```Makefile
   test-gaia:
   	@go test -mod=readonly -v -coverprofile coverage.out ./test/... -tags gaia

   test-mychain:
       @go test -mod=readonly -v -coverprofile coverage.out ./test/... -tags mychain
   ```

2. Add a `.github/{mychain}-tests.yml` file that is a copy of `.github/gaia-tests.yml`
   but modified for your chain.

   ```yml
   name: TESTING - gaia to mychain integration

   on: [push]

   jobs:

     build:
       name: build
       runs-on: ubuntu-latest
       steps:

       # Install and setup go
       - name: Set up Go 1.14
         uses: actions/setup-go@v1
         with:
           go-version: 1.14
         id: go

       # setup docker
       - name: Set up Docker 19.03
         uses: docker-practice/actions-setup-docker@0.0.1    
         with:
           docker-version: 19.03
           docker-channel: stable

       # checkout relayer
       - name: checkout relayer
         uses: actions/checkout@v2

       # build cache
       - uses: actions/cache@v1
         with:
           path: ~/go/pkg/mod
           key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
           restore-keys: |
             ${{ runner.os }}-go-
       
       # run tests
       - name: run mychain tests
         run: make test-mychain
   ```
