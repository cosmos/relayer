package cmd_test

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/cosmos/relayer/v2/cmd"
	"github.com/cosmos/relayer/v2/internal/relaydebug"
	"github.com/cosmos/relayer/v2/internal/relayermetrics"
	"github.com/cosmos/relayer/v2/internal/relayertest"
	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestMetricsServerFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		description    string
		args           []string
		wantedPort     int
		wantedRunning  bool
		wantedMessages []string
		err            error
	}{
		{
			"should start relayer without metrics server given no flags",
			[]string{"start"},
			0,
			false,
			[]string{"Metrics server is disabled. You can enable it using --enable-metrics-server flag."},
			nil,
		},
		{
			"should start relayer with metrics server running on default port",
			[]string{"start", "--enable-metrics-server"},
			relayermetrics.MetricsServerPort,
			true,
			[]string{"Metrics server is enabled", "Metrics server listening"},
			nil,
		},
		{
			"should start relayer with metrics server running on custom port",
			[]string{"start", "--enable-metrics-server", "--metrics-listen-addr", "127.0.0.1:7778"},
			7778,
			true,
			[]string{"Metrics server is enabled", "Metrics server listening"},
			nil,
		},
		{
			"should start relayer without metrics server when metrics server is not enabled",
			[]string{"start", "--metrics-listen-addr", "127.0.0.1:7778"},
			0,
			false,
			[]string{"Metrics server is disabled. You can enable it using --enable-metrics-server flag."},
			nil,
		},
		{
			"should not start relayer and report an error when address is not provided",
			[]string{"start", "--metrics-listen-addr"},
			0,
			false,
			nil,
			errors.New("flag needs an argument: --metrics-listen-addr"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			sys := setupRelayer(t)
			logs, logger := setupLogger()

			result := sys.MustRunWithLogger(t, logger, tt.args...)

			if tt.err != nil {
				require.Error(t, result.Err, tt.err)
			} else {
				require.NoError(t, result.Err)
			}

			if tt.wantedRunning == true {
				requireRunningMetricsServer(t, logs, tt.wantedPort)
			} else {
				requireDisabledMetricsServer(t, logs, tt.wantedPort)
			}
			requireMessages(t, logs, tt.wantedMessages)
		})
	}
}

func TestMetricsServerConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		description    string
		args           []string
		newSetting     string
		wantedPort     int
		serverRunning  bool
		wantedMessages []string
	}{
		{
			"should starts relayer on custom address and port provided in config file",
			[]string{"start", "--enable-metrics-server"},
			"metrics-listen-addr: 127.0.0.1:6184",
			6184,
			true,
			[]string{"Metrics server is enabled", "Metrics server listening"},
		},
		{
			"should starts relayer on custom address provided via flag",
			[]string{"start", "--enable-metrics-server", "--metrics-listen-addr", "127.0.0.1:7184"},
			"",
			7184,
			true,
			[]string{"Metrics server is enabled", "Metrics server listening"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			sys := setupRelayer(t)

			updateConfig(t, sys, "metrics-listen-addr: 127.0.0.1:5184", tt.newSetting)

			logs, logger := setupLogger()

			sys.MustRunWithLogger(t, logger, tt.args...)

			if tt.serverRunning == true {
				requireRunningMetricsServer(t, logs, tt.wantedPort)
			} else {
				requireDisabledMetricsServer(t, logs, tt.wantedPort)
			}
			requireMessages(t, logs, tt.wantedMessages)
		})
	}
}

func TestMissingMetricsListenAddr(t *testing.T) {
	sys := setupRelayer(t)

	logs, logger := setupLogger()

	updateConfig(t, sys, "metrics-listen-addr: 127.0.0.1:5184", "")

	sys.MustRunWithLogger(t, logger, []string{"start", "--enable-metrics-server"}...)

	requireDisabledMetricsServer(t, logs, 0)
	requireMessage(t, logs, "Disabled metrics server due to missing metrics-listen-addr setting in config file or --metrics-listen-addr flag.")
}

func TestDebugServerFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		description    string
		args           []string
		wantedPort     int
		wantedRunning  bool
		wantedMessages []string
		err            error
	}{
		{
			"should start relayer without debug server",
			[]string{"start"},
			0,
			false,
			[]string{"Debug server is disabled. You can enable it using --enable-debug-server flag."},
			nil,
		},
		{
			"should not start the relayer and report an error when address is missing",
			[]string{"start", "--debug-addr"},
			0,
			false,
			nil,
			errors.New("flag needs an argument: --debug-addr"),
		},
		{
			"should start relayer with debug server given --debug-addr flag and address",
			[]string{"start", "--debug-addr", "127.0.0.1:7777"},
			7777,
			true,
			[]string{
				"Debug server is enabled", "SECURITY WARNING! Debug server should only be run with caution and proper security in place.",
				"DEPRECATED: --debug-addr flag is deprecated use --enable-debug-server and --debug-listen-addr instead.",
			},
			nil,
		},
		{
			"should start relayer with debug server given --enable-debug-server flag",
			[]string{"start", "--enable-debug-server"},
			relaydebug.DebugServerPort,
			true,
			[]string{"Debug server is enabled", "SECURITY WARNING! Debug server should only be run with caution and proper security in place."},
			nil,
		},
		{
			"should start relayer with debug server given --enable-debug-server flag and an address",
			[]string{"start", "--enable-debug-server", "--debug-listen-addr", "127.0.0.1:7779"},
			7779,
			true,
			[]string{"Debug server is enabled", "SECURITY WARNING! Debug server should only be run with caution and proper security in place."},
			nil,
		},
		{
			"should not start relayer and report an error when address is missing",
			[]string{"start", "--enable-debug-server", "--debug-listen-addr"},
			0,
			false,
			nil,
			errors.New("flag needs an argument: --debug-listen-addr"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			sys := setupRelayer(t)
			logs, logger := setupLogger()

			result := sys.MustRunWithLogger(t, logger, tt.args...)

			if tt.err != nil {
				require.Error(t, result.Err, tt.err)
			} else {
				require.NoError(t, result.Err)
			}

			if tt.wantedRunning == true {
				requireRunningDebugServer(t, logs, tt.wantedPort)
			} else {
				requireDisabledDebugServer(t, logs, tt.wantedPort)
			}
			requireMessages(t, logs, tt.wantedMessages)
		})
	}
}

func TestDebugServerConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		description   string
		args          []string
		newSetting    string
		wantedPort    int
		wantedRunning bool
	}{
		{
			"should start debug server on custom address and port set in config file",
			[]string{"start", "--enable-debug-server"},
			"debug-listen-addr: 127.0.0.1:6183",
			6183,
			true,
		},
		{
			"should start debug server on custom address and port set via flag",
			[]string{"start", "--enable-debug-server", "--debug-listen-addr", "127.0.0.1:7183"},
			"debug-listen-addr: 127.0.0.1:6183",
			7183,
			true,
		},
		{
			"should start debug server on custom address and port set via deprecated flag",
			[]string{"start", "--enable-debug-server", "--debug-addr", "127.0.0.1:9183"},
			"debug-listen-addr: 127.0.0.1:6183",
			9183,
			true,
		},
		{
			"should start debug server on custom address and port set via deprecated config",
			[]string{"start", "--enable-debug-server"},
			"api-listen-addr: 127.0.0.1:10183",
			10183,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			sys := setupRelayer(t)

			updateConfig(t, sys, "debug-listen-addr: 127.0.0.1:5183", tt.newSetting)

			logs, logger := setupLogger()

			sys.MustRunWithLogger(t, logger, tt.args...)

			if tt.wantedRunning == true {
				requireRunningDebugServer(t, logs, tt.wantedPort)
			} else {
				requireDisabledDebugServer(t, logs, tt.wantedPort)
			}
		})
	}
}

func TestMissingDebugListenAddr(t *testing.T) {
	sys := setupRelayer(t)

	logs, logger := setupLogger()

	updateConfig(t, sys, "debug-listen-addr: 127.0.0.1:5183", "")

	sys.MustRunWithLogger(t, logger, []string{"start", "--enable-debug-server"}...)

	requireDisabledMetricsServer(t, logs, 0)
	requireMessage(t, logs, "Disabled debug server due to missing debug-listen-addr setting in config file or --debug-listen-addr flag.")
}

func requireDisabledMetricsServer(t *testing.T, logs *observer.ObservedLogs, port int) {
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if conn != nil {
		defer conn.Close()
	}
	require.Error(t, err, "Server should be disabled")
}

func requireRunningMetricsServer(t *testing.T, logs *observer.ObservedLogs, port int) {
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if conn != nil {
		defer conn.Close()
	}
	require.NoError(t, err, fmt.Sprintf("Metrics server should be running on port %d", port))
	res, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/metrics", port))
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, res.StatusCode, 200)
}

func requireDisabledDebugServer(t *testing.T, logs *observer.ObservedLogs, port int) {
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if conn != nil {
		defer conn.Close()
	}
	require.Error(t, err, "Server should be disabled")
}

func requireRunningDebugServer(t *testing.T, logs *observer.ObservedLogs, port int) {
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if conn != nil {
		defer conn.Close()
	}
	require.NoError(t, err, fmt.Sprintf("Server should be running on port %d", port))
	res, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/debug/pprof/goroutine", port))
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, res.StatusCode, 200)
}

func requireMessages(t *testing.T, logs *observer.ObservedLogs, messages []string) {
	for _, message := range messages {
		requireMessage(t, logs, message)
	}
}

func requireMessage(t *testing.T, logs *observer.ObservedLogs, message string) {
	require.Len(t, logs.FilterMessage(message).All(), 1, fmt.Sprintf("Expected message '%s' not found in logs", message), logs.All())
}

func setupLogger() (*observer.ObservedLogs, *zap.Logger) {
	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	observedLogger := zap.New(observedZapCore)
	return observedLogs, observedLogger
}

func setupRelayer(t *testing.T) *relayertest.System {
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
	return sys
}

func updateConfig(t *testing.T, sys *relayertest.System, oldSetting string, newSetting string) {
	configFile := fmt.Sprintf("%s/config/config.yaml", sys.HomeDir)
	data, err := os.ReadFile(configFile)
	require.NoError(t, err)

	newConfig := strings.Replace(string(data), oldSetting, newSetting, 1)

	os.WriteFile(configFile, []byte(newConfig), 0644)
}
