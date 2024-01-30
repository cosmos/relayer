# Retaining data on failed tests

By default, failed tests will clean up any temporary directories they created.
Sometimes when debugging a failed test, it can be more helpful to leave the directory behind
for further manual inspection.

Setting the environment variable `IBCTEST_SKIP_FAILURE_CLEANUP` to any non-empty value
will cause the test to skip deletion of the temporary directories.
Any tests that fail and skip cleanup will log a message like
`Not removing temporary directory for test at: /tmp/...`.

Test authors must use
[`interchaintest.TempDir`](https://pkg.go.dev/github.com/strangelove-ventures/interchaintest#TempDir)
instead of `(*testing.T).Cleanup` to opt in to this behavior.

By default, Docker volumes associated with tests are cleaned up at the end of each test run.
That same `IBCTEST_SKIP_FAILURE_CLEANUP` controls whether the volumes associated with failed tests are pruned.