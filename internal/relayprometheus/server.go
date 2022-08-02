package relayprometheus

import (
	"context"
	"net"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// Serve starts a prometheus server in a background goroutine,
// accepting connections on the given listener.
// Any HTTP logging will be written at info level to the given logger.
// The server will be forcefully shut down when ctx finishes.
func Serve(ctx context.Context, log *zap.Logger, ln net.Listener) {
	// Although we could just import net/http/pprof and rely on the default global server,
	// we may want many instances of this in test,
	// and we will probably want more endpoints as time goes on,
	// so use a dedicated http.Server instance here.

	// Set up new mux identical to the default mux configuration in net/http/pprof.
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

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
