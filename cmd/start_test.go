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
		args          []string
		wantedRunning bool
	}{
		{
			[]string{"start"},
			false,
		},
		{
			[]string{"start", "--enable-metrics-server"},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			sys := setupRelayer(t)
			logs, logger := setupLogger()

			sys.MustRunWithLogger(t, logger, tt.args...)

			if tt.wantedRunning == true {
				requireRunningMetricsServer(t, logs, relayermetrics.MetricsServerPort)
			} else {
				requireDisabledMetricsServer(t, logs, relayermetrics.MetricsServerPort)
			}
		})
	}
}

func TestMetricsServerConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		args          []string
		newSetting    string
		wantedPort    int
		serverRunning bool
	}{
		{
			[]string{"start"},
			"",
			0,
			false,
		},
		{
			[]string{"start", "--enable-metrics-server"},
			"metrics-listen-addr: 127.0.0.1:6184",
			6184,
			true,
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
		})
	}
}

func TestDebugServerFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		args          []string
		wantedRunning bool
	}{
		{
			[]string{"start"},
			false,
		},
		{
			[]string{"start", "--enable-debug-server"},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			sys := setupRelayer(t)
			logs, logger := setupLogger()

			sys.MustRunWithLogger(t, logger, tt.args...)

			if tt.wantedRunning == true {
				requireRunningDebugServer(t, logs, relaydebug.DebugServerPort)
			} else {
				requireDisabledDebugServer(t, logs, relaydebug.DebugServerPort)
			}
		})
	}
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
	require.Len(t, logs.FilterMessage("Disabled debug server due to missing debug-listen-addr setting in config file.").All(), 1)
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
	require.Len(t, logs.FilterMessage("Metrics server listening").All(), 1)
	require.Len(t, logs.FilterMessage("Metrics server is enabled.").All(), 1)
}

func requireDisabledDebugServer(t *testing.T, logs *observer.ObservedLogs, port int) {
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if conn != nil {
		defer conn.Close()
	}
	require.Error(t, err, "Server should be disabled")
	require.Len(t, logs.FilterMessage("Disabled debug server due to missing debug-listen-addr setting in config file.").All(), 1)
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
	require.Len(t, logs.FilterMessage("Debug server listening").All(), 1)
	require.Len(t, logs.FilterMessage("SECURITY WARNING! Debug server is enabled. It should only be used with caution and proper security.").All(), 1)
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
