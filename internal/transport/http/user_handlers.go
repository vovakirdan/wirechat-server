package http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/vovakirdan/wirechat-server/internal/store"
)

// UserHandlers provides HTTP handlers for user operations.
type UserHandlers struct {
	store store.Store
	log   *zerolog.Logger
}

// NewUserHandlers creates a new user handlers instance.
func NewUserHandlers(st store.Store, logger *zerolog.Logger) *UserHandlers {
	return &UserHandlers{
		store: st,
		log:   logger,
	}
}

// UserResponse represents a user in API responses.
type UserResponse struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"` // Fallback to username for now
}

// SearchUsers handles searching for users.
// GET /api/users/search?q=query
func (h *UserHandlers) SearchUsers(c *gin.Context) {
	query := c.Query("q")
	trimmed := strings.TrimSpace(query)
	if len(trimmed) < 3 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "search query must be at least 3 characters"})
		return
	}

	// Get current user ID to exclude from results
	currentUserID, exists := c.Get(ContextKeyUserID)
	if !exists {
		h.log.Error().Msg("user_id not found in context")
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	uid, ok := currentUserID.(int64)
	if !ok {
		h.log.Error().Msg("invalid user_id type in context")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	users, err := h.store.SearchUsers(c.Request.Context(), trimmed)
	if err != nil {
		h.log.Error().Err(err).Str("query", trimmed).Msg("failed to search users")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	response := make([]UserResponse, 0)
	for _, u := range users {
		// key exclusion: don't show self
		if u.ID == uid {
			continue
		}
		
		response = append(response, UserResponse{
			ID:       u.ID,
			Username: u.Username,
			Name:     u.Username, // wirechat doesn't have display names yet
		})
	}

	c.JSON(http.StatusOK, response)
}
