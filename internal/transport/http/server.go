package http

import (
	stdhttp "net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/vovakirdan/wirechat-server/internal/auth"
	"github.com/vovakirdan/wirechat-server/internal/config"
	"github.com/vovakirdan/wirechat-server/internal/core"
)

// NewServer builds an HTTP server with REST API and WebSocket routes.
func NewServer(hub core.Hub, authService *auth.Service, cfg *config.Config, logger *zerolog.Logger) *stdhttp.Server {
	// Set Gin mode based on log level
	gin.SetMode(gin.ReleaseMode)

	// Gin router for REST API
	ginRouter := gin.New()
	ginRouter.Use(gin.Recovery())
	ginRouter.Use(LoggerMiddleware(logger))

	// API endpoints
	apiHandlers := NewAPIHandlers(authService, logger)
	api := ginRouter.Group("/api")
	api.POST("/register", apiHandlers.Register)
	api.POST("/login", apiHandlers.Login)
	api.POST("/guest", apiHandlers.GuestLogin)

	// Main mux - combines Gin for API and direct handler for WebSocket
	mux := stdhttp.NewServeMux()

	// Health endpoint
	mux.HandleFunc("/health", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		logger.Info().Str("method", r.Method).Str("path", r.URL.Path).Int("status", 200).Msg("http request")
		w.WriteHeader(stdhttp.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			logger.Warn().Err(err).Msg("failed to write health response")
		}
	})

	// WebSocket endpoint - direct handler, no Gin wrapper
	wsHandler := NewWSHandler(hub, authService, cfg, logger)
	mux.Handle("/ws", wsHandler)

	// API endpoints - handled by Gin
	mux.Handle("/api/", ginRouter)

	return &stdhttp.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
	}
}
