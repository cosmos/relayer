package relayermetrics

import (
	"context"
	"net"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// StartMetricsServer starts a metrics server in a background goroutine,
// accepting connections on the given listener.
// Any HTTP logging will be written at info level to the given logger.
// The server will be forcefully shut down when ctx finishes.
func StartMetricsServer(ctx context.Context, log *zap.Logger, ln net.Listener, registry *prometheus.Registry) {
	// Set up new mux identical to the default mux configuration in net/http/pprof.
	mux := http.NewServeMux()

	// Serve default prometheus metrics
	mux.Handle("/metrics", promhttp.Handler())

	// Serve relayer metrics
	mux.Handle("/relayer/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	srv := &http.Server{
		Handler:  mux,
		ErrorLog: zap.NewStdLog(log),
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}

	go srv.Serve(ln)

	go func() {
		<-ctx.Done()
		srv.Close()
	}()
}
