# interchaintest

This directory provides a convenient way for `relayer` developers to run a small suite of IBC compatibility tests.

Use `make interchaintest` from the root directory to run these tests.

This is provided as a nested module so that `go test ./...` from the root of the `relayer` repository
will continue to run only the faster unit tests.

## Developer notes

### New Test

If you are developing a new relayer test for `interchaintest`, you may want to run:

```
go mod edit -replace=github.com/strangelove-ventures/interchaintest=../../../strangelove-ventures/interchaintest
```

from this directory.
Be sure to drop the replace, with:

```
go mod edit -dropreplace=github.com/strangelove-ventures/interchaintest
```

before you commit.


### Specify interchaintest Version

If you would like to point to a specific version of `interchaintest`, you can do so using a commit hash.

From the relayer/interchaintest directory, run:

```
go get github.com/strangelove-ventures/interchaintest@<COMMIT_HASH_HERE>
```

Your go.mod file should update respectively.

### Inclusion in CI

interchaintest requires non-trivial resources to run. As such, CI tests are partitioned. See [the github workflow](../.github/workflows/interchaintest.yml).

If you want a test to be included in CI automatically, name your test with prefix `TestScenario`. For these tests, 
it's highly recommended to use a simple network of 1 validator and no full nodes.