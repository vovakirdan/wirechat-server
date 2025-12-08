package app

import (
	"context"
	"fmt"
	stdhttp "net/http"
	"time"

	"github.com/rs/zerolog"
	"github.com/vovakirdan/wirechat-server/internal/auth"
	"github.com/vovakirdan/wirechat-server/internal/callengine"
	"github.com/vovakirdan/wirechat-server/internal/callengine/livekit"
	"github.com/vovakirdan/wirechat-server/internal/config"
	"github.com/vovakirdan/wirechat-server/internal/core"
	"github.com/vovakirdan/wirechat-server/internal/service/calls"
	"github.com/vovakirdan/wirechat-server/internal/service/friends"
	"github.com/vovakirdan/wirechat-server/internal/store"
	"github.com/vovakirdan/wirechat-server/internal/store/sqlite"
	transporthttp "github.com/vovakirdan/wirechat-server/internal/transport/http"
)

// App wires together core and transport layers.
type App struct {
	server          *stdhttp.Server
	shutdownTimeout time.Duration
	hub             core.Hub
	store           store.Store
	log             *zerolog.Logger
}

// New constructs the application with provided configuration.
func New(cfg *config.Config, logger *zerolog.Logger) (*App, error) {
	// Initialize database store
	st, err := sqlite.New(cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("init store: %w", err)
	}

	logger.Info().Str("db_path", cfg.DatabasePath).Msg("database initialized")

	// Create JWT config
	jwtConfig := &auth.JWTConfig{
		Secret:   []byte(cfg.JWTSecret),
		Issuer:   cfg.JWTIssuer,
		Audience: cfg.JWTAudience,
		TTL:      24 * time.Hour, // 24 hour token expiry
	}

	// Create auth service
	authService := auth.NewService(st, jwtConfig)

	// Create services
	friendsService := friends.New(st)

	// Create call engine (LiveKit) if enabled
	var callEngine callengine.Engine
	if cfg.LiveKit.Enabled {
		if cfg.LiveKit.APIKey == "" || cfg.LiveKit.APISecret == "" {
			return nil, fmt.Errorf("livekit is enabled but api_key or api_secret is not set")
		}
		callEngine = livekit.New(cfg.LiveKit.APIKey, cfg.LiveKit.APISecret, cfg.LiveKit.WSURL)
		logger.Info().
			Str("ws_url", cfg.LiveKit.WSURL).
			Msg("LiveKit integration enabled")
	} else {
		logger.Info().Msg("LiveKit integration disabled")
	}

	callsService := calls.New(st, callEngine, friendsService)

	hub := core.NewHub(st)
	server := transporthttp.NewServer(hub, authService, st, friendsService, callsService, cfg, logger)

	return &App{
		server:          server,
		shutdownTimeout: cfg.ShutdownTimeout,
		hub:             hub,
		store:           st,
		log:             logger,
	}, nil
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
		a.cleanup()
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.shutdownTimeout)
		defer cancel()

		a.log.Info().Msg("shutting down http server")
		if err := a.server.Shutdown(shutdownCtx); err != nil {
			a.cleanup()
			return err
		}

		a.cleanup()
		return <-serverErr
	}
}

// cleanup closes database and other resources.
func (a *App) cleanup() {
	if a.store != nil {
		if err := a.store.Close(); err != nil {
			a.log.Warn().Err(err).Msg("failed to close store")
		} else {
			a.log.Info().Msg("store closed")
		}
	}
}
