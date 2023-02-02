// Package interchaintest is used for running a relatively quick IBC compatibility test
// against a pinned version of ibc-test-framework.
//
// These tests are intended to be run via 'make interchaintest'.
//
// This is meant as a convenience to relayer maintainers.
// The ibc-test-framework, at its main branch,
// should be considered the canonical definition of the IBC compatibility tests.
//
// As the upstream ibc-test-framework gets updated,
// it is the relayer maintainers' responsibility to keep
// the version of ibc-test-framework updated in go.mod.
package interchaintest
