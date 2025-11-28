package http

import (
	"fmt"
	stdhttp "net/http"

	"github.com/vovakirdan/wirechat-server/internal/config"
	"github.com/vovakirdan/wirechat-server/internal/core"
)

// NewServer builds an HTTP server with basic routes.
func NewServer(hub *core.Hub, cfg config.Config) *stdhttp.Server {
	mux := stdhttp.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.Handle("/ws", NewWSHandler(hub))

	return &stdhttp.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
	}
}

func healthHandler(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
	_, _ = fmt.Fprint(w, "ok")
}
