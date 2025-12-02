package http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/vovakirdan/wirechat-server/internal/auth"
)

const (
	// ContextKeyUserID is the context key for storing user ID.
	ContextKeyUserID = "user_id"
	// ContextKeyUsername is the context key for storing username.
	ContextKeyUsername = "username"
	// ContextKeyIsGuest is the context key for storing guest status.
	ContextKeyIsGuest = "is_guest"
)

// AuthMiddleware creates a middleware that validates JWT tokens.
func AuthMiddleware(authService *auth.Service, logger *zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			logger.Debug().Msg("missing authorization header")
			c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "missing authorization header"})
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			logger.Debug().Msg("invalid authorization header format")
			c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "invalid authorization header format"})
			c.Abort()
			return
		}

		token := parts[1]
		claims, err := authService.ValidateToken(token)
		if err != nil {
			logger.Debug().Err(err).Msg("invalid token")
			c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "invalid token"})
			c.Abort()
			return
		}

		// Store user info in context
		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyUsername, claims.Username)
		c.Set(ContextKeyIsGuest, claims.IsGuest)

		c.Next()
	}
}

// LoggerMiddleware creates a middleware that logs HTTP requests.
func LoggerMiddleware(logger *zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Process request
		c.Next()

		// Log after request
		logger.Info().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Msg("http request")
	}
}
