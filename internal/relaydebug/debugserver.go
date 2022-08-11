package relaydebug

import (
	"context"
	"net"
	"net/http"
	"net/http/pprof"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// StartDebugServer starts a debug server in a background goroutine,
// accepting connections on the given listener.
// Any HTTP logging will be written at info level to the given logger.
// The server will be forcefully shut down when ctx finishes.
func StartDebugServer(ctx context.Context, log *zap.Logger, ln net.Listener) {
	// Although we could just import net/http/pprof and rely on the default global server,
	// we may want many instances of this in test,
	// and we will probably want more endpoints as time goes on,
	// so use a dedicated http.Server instance here.

	// Set up new mux identical to the default mux configuration in net/http/pprof.
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	// And redirect the browser to the /debug/pprof root,
	// so operators don't see a mysterious 404 page.
	mux.Handle("/", http.RedirectHandler("/debug/pprof", http.StatusSeeOther))

	// Serve prometheus metrics
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
