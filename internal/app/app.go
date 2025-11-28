package app

import (
	"context"
	stdhttp "net/http"
	"time"

	"github.com/rs/zerolog"
	"github.com/vovakirdan/wirechat-server/internal/config"
	"github.com/vovakirdan/wirechat-server/internal/core"
	transporthttp "github.com/vovakirdan/wirechat-server/internal/transport/http"
)

// App wires together core and transport layers.
type App struct {
	server          *stdhttp.Server
	shutdownTimeout time.Duration
	hub             core.Hub
	log             *zerolog.Logger
}

// New constructs the application with provided configuration.
func New(cfg *config.Config, logger *zerolog.Logger) *App {
	hub := core.NewHub()
	server := transporthttp.NewServer(hub, cfg, logger)

	return &App{
		server:          server,
		shutdownTimeout: cfg.ShutdownTimeout,
		hub:             hub,
		log:             logger,
	}
}

// Run starts the HTTP server and blocks until context cancellation or fatal error.
func (a *App) Run(ctx context.Context) error {
	serverErr := make(chan error, 1)

	go a.hub.Run(ctx)

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

		a.log.Info().Msg("shutting down http server")
		if err := a.server.Shutdown(shutdownCtx); err != nil {
			return err
		}

		return <-serverErr
	}
}
