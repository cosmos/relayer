# ibctest

This directory provides a convenient way for `relayer` developers to run a small suite of IBC compatibility tests.

Use `make ibctest` from the root directory to run these tests.

This is provided as a nested module so that `go test ./...` from the root of the `relayer` repository
will continue to run only the faster unit tests.

## Developer notes

If you are developing a new relayer test for `ibc-test-framework`, you may want to run:

```
go mod edit -replace=github.com/strangelove-ventures/ibc-test-framework=../../../strangelove-ventures/ibc-test-framework
```

from this directory.
Be sure to drop the replace, with:

```
go mod edit -dropreplace=github.com/strangelove-ventures/ibc-test-framework
```

before you commit.
