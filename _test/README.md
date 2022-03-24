# \_test

This directory contains integration-style tests that require Docker to run.

The leading underscore on the directory means that go tooling ignores this directory unless specifically requested.
This allows `go test ./...` to work without needing Docker.

The intended way to run these integration tests is with `make test-integration` from the root directory.
