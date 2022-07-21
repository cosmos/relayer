package cmd_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/cosmos/relayer/v2/cmd"
	"github.com/cosmos/relayer/v2/internal/relayertest"
	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/stretchr/testify/require"
)

func TestChainsList_Empty(t *testing.T) {
	t.Parallel()

	sys := relayertest.NewSystem(t)

	_ = sys.MustRun(t, "config", "init")

	res := sys.MustRun(t, "chains", "list")

	// Before adding any chains, attempting to list the chains gives a helpful message on stderr.
	require.Empty(t, res.Stdout.String())
	require.Contains(t, res.Stderr.String(), "no chains found")
}

func TestChainsAdd_File(t *testing.T) {
	t.Parallel()

	sys := relayertest.NewSystem(t)

	_ = sys.MustRun(t, "config", "init")

	sys.MustAddChain(t, "testChain", cmd.ProviderConfigWrapper{
		Type: "cosmos",
		Value: cosmos.CosmosProviderConfig{
			ChainID:        "testcosmos",
			KeyringBackend: "test",
			Timeout:        "10s",
		},
	})

	// List the chain back in plaintext.
	res := sys.MustRun(t, "chains", "list")
	require.Regexp(t, regexp.MustCompile(`^\s*\d+: testcosmos\s+-> type\(cosmos\) key\(✘\) bal\(✘\) path\(✘\)`), res.Stdout.String())
	require.Empty(t, res.Stderr.String())
}

func TestChainsAdd_URL(t *testing.T) {
	t.Parallel()

	sys := relayertest.NewSystem(t)

	_ = sys.MustRun(t, "config", "init")

	// Start an HTTP server to use for chains add --url.
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chain.json" {
			http.NotFound(w, r)
			return
		}

		pcw := cmd.ProviderConfigWrapper{
			Type: "cosmos",
			Value: cosmos.CosmosProviderConfig{
				ChainID:        "testcosmos",
				KeyringBackend: "test",
				Timeout:        "10s",
			},
		}

		enc := json.NewEncoder(w)
		enc.Encode(pcw)
	})
	srv := httptest.NewServer(h)
	defer srv.Close()

	// Add the chain at the URL and path of the test server.
	chainName := "testChain"
	res := sys.MustRun(t, "chains", "add", "--url", srv.URL+"/chain.json", chainName)
	require.Empty(t, res.Stdout.String())
	require.Empty(t, res.Stderr.String())

	// List the chain back in plaintext.
	res = sys.MustRun(t, "chains", "list")
	require.Regexp(t, regexp.MustCompile(`^\s*\d+: testcosmos\s+-> type\(cosmos\) key\(✘\) bal\(✘\) path\(✘\)`), res.Stdout.String())
	require.Empty(t, res.Stderr.String())
}

func TestChainsAdd_Delete(t *testing.T) {
	t.Parallel()

	sys := relayertest.NewSystem(t)

	_ = sys.MustRun(t, "config", "init")

	sys.MustAddChain(t, "testChain", cmd.ProviderConfigWrapper{
		Type: "cosmos",
		Value: cosmos.CosmosProviderConfig{
			ChainID:        "testcosmos",
			KeyringBackend: "test",
			Timeout:        "10s",
		},
	})

	// List the chain back in plaintext.
	res := sys.MustRun(t, "chains", "list")
	require.Contains(t, res.Stdout.String(), "testcosmos")
	require.Empty(t, res.Stderr.String())

	// Delete the added chain. Silent output.
	res = sys.MustRun(t, "chains", "delete", "testChain")
	require.Empty(t, res.Stdout.String())
	require.Empty(t, res.Stderr.String())

	// Listing again gives a warning about no chains.
	res = sys.MustRun(t, "chains", "list")
	require.Empty(t, res.Stdout.String())
	require.Contains(t, res.Stderr.String(), "no chains found")
}
