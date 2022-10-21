# ibctest

This directory provides a convenient way for `relayer` developers to run a small suite of IBC compatibility tests.

Use `make ibctest` from the root directory to run these tests.

This is provided as a nested module so that `go test ./...` from the root of the `relayer` repository
will continue to run only the faster unit tests.

## Developer notes

### New Test

If you are developing a new relayer test for `ibctest`, you may want to run:

```
go mod edit -replace=github.com/strangelove-ventures/ibctest=../../../strangelove-ventures/ibctest
```

from this directory.
Be sure to drop the replace, with:

```
go mod edit -dropreplace=github.com/strangelove-ventures/ibctest
```

before you commit.


### Specify ibctest Version

If you would like to point to a specific version of `ibctest`, you can do so using a commit hash.

From the relayer/ibctest directory, run:

```
go get github.com/strangelove-ventures/ibctest@<COMMIT_HASH_HERE>
```

Your go.mod file should update respectively.

### Inclusion in CI

ibctest requires non-trivial resources to run. As such, CI tests are partitioned. See [the github workflow](../.github/workflows/ibctest.yml).

If you want a test to be included in CI automatically, name your test with prefix `TestScenario`. For these tests, 
it's highly recommended to use a simple network of 1 validator and no full nodes.