package cmd_test

import (
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

func TestMetricsServerFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		args           []string
		wantedPort     int
		wantedRunning  bool
		wantedMessages []string
	}{
		{
			[]string{"start"},
			0,
			false,
			[]string{"Metrics server is disabled. You can enable it using --enable-metrics-server flag."},
		},
		{
			[]string{"start", "--enable-metrics-server"},
			relayermetrics.MetricsServerPort,
			true,
			[]string{"Metrics server is enabled", "Metrics server listening"},
		},
		{
			[]string{"start", "--enable-metrics-server", "--metrics-listen-addr", "127.0.0.1:7778"},
			7778,
			true,
			[]string{"Metrics server is enabled", "Metrics server listening"},
		},
		{
			[]string{"start", "--metrics-listen-addr", "127.0.0.1:7778"},
			0,
			false,
			[]string{"Metrics server is disabled. You can enable it using --enable-metrics-server flag."},
		},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			sys := setupRelayer(t)
			logs, logger := setupLogger()

			sys.MustRunWithLogger(t, logger, tt.args...)

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
		args           []string
		newSetting     string
		wantedPort     int
		serverRunning  bool
		wantedMessages []string
	}{
		{
			[]string{"start"},
			"",
			0,
			false,
			[]string{"Metrics server is disabled. You can enable it using --enable-metrics-server flag."},
		},
		{
			[]string{"start", "--enable-metrics-server"},
			"metrics-listen-addr: 127.0.0.1:6184",
			6184,
			true,
			[]string{"Metrics server is enabled", "Metrics server listening"},
		},
		{
			[]string{"start", "--enable-metrics-server", "--metrics-listen-addr", "127.0.0.1:7184"},
			"",
			7184,
			true,
			[]string{"Metrics server is enabled", "Metrics server listening"},
		},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
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

func TestDebugServerFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		args           []string
		wantedPort     int
		wantedRunning  bool
		wantedMessages []string
	}{
		{
			[]string{"start"},
			0,
			false,
			[]string{"Debug server is disabled. You can enable it using --enable-debug-server flag."},
		},
		{
			[]string{"start", "--enable-debug-server"},
			relaydebug.DebugServerPort,
			true,
			[]string{"Debug server is enabled", "SECURITY WARNING! Debug server should only be run with caution and proper security in place."},
		},
		{
			[]string{"start", "--enable-debug-server", "--debug-listen-addr", "127.0.0.1:7777"},
			7777,
			true,
			[]string{"Debug server is enabled", "SECURITY WARNING! Debug server should only be run with caution and proper security in place."},
		},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			sys := setupRelayer(t)
			logs, logger := setupLogger()

			sys.MustRunWithLogger(t, logger, tt.args...)

			if tt.wantedRunning == true {
				requireRunningDebugServer(t, logs, tt.wantedPort)
			} else {
				requireDisabledDebugServer(t, logs, tt.wantedPort)
			}
			requireMessages(t, logs, tt.wantedMessages)
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

func TestDebugServerConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		args          []string
		newSetting    string
		wantedPort    int
		wantedRunning bool
	}{
		{
			[]string{"start"},
			"",
			0,
			false,
		},
		{
			[]string{"start", "--enable-debug-server"},
			"debug-listen-addr: 127.0.0.1:6183",
			6183,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
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
	require.NoError(t, err, "Metrics server should be running")
	res, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/metrics", port))
	require.NoError(t, err)
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
	require.NoError(t, err, "Server should be running")
	res, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/debug/pprof/goroutine", port))
	require.NoError(t, err)
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
