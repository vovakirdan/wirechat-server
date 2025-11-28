package app

import (
	"context"
	stdhttp "net/http"
	"time"

	"github.com/vovakirdan/wirechat-server/internal/config"
	"github.com/vovakirdan/wirechat-server/internal/core"
	transporthttp "github.com/vovakirdan/wirechat-server/internal/transport/http"
)

// App wires together core and transport layers.
type App struct {
	server          *stdhttp.Server
	shutdownTimeout time.Duration
}

// New constructs the application with provided configuration.
func New(cfg config.Config) *App {
	hub := core.NewHub()
	server := transporthttp.NewServer(hub, cfg)

	return &App{
		server:          server,
		shutdownTimeout: cfg.ShutdownTimeout,
	}
}

// Run starts the HTTP server and blocks until context cancellation or fatal error.
func (a *App) Run(ctx context.Context) error {
	serverErr := make(chan error, 1)

	go func() {
		if err := a.server.ListenAndServe(); err != nil && err != stdhttp.ErrServerClosed {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.shutdownTimeout)
		defer cancel()

		if err := a.server.Shutdown(shutdownCtx); err != nil {
			return err
		}

		return <-serverErr
	}
}
