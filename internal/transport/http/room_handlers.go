package http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/vovakirdan/wirechat-server/internal/store"
)

// RoomHandlers provides HTTP handlers for room management endpoints.
type RoomHandlers struct {
	store store.Store
	log   *zerolog.Logger
}

// NewRoomHandlers creates a new room handlers instance.
func NewRoomHandlers(st store.Store, logger *zerolog.Logger) *RoomHandlers {
	return &RoomHandlers{
		store: st,
		log:   logger,
	}
}

// CreateRoomRequest represents the create room request body.
type CreateRoomRequest struct {
	Name string `json:"name" binding:"required,min=1,max=64"`
}

// RoomResponse represents a room in API responses.
type RoomResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	OwnerID   *int64 `json:"owner_id,omitempty"`
	CreatedAt string `json:"created_at"`
}

// CreateRoom handles room creation.
// POST /api/rooms
func (h *RoomHandlers) CreateRoom(c *gin.Context) {
	// Get authenticated user from context
	userID, exists := c.Get(ContextKeyUserID)
	if !exists {
		h.log.Error().Msg("user_id not found in context")
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	uid, ok := userID.(int64)
	if !ok {
		h.log.Error().Msg("invalid user_id type in context")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	var req CreateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Debug().Err(err).Msg("invalid create room request")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	// Create public room with current user as owner
	room, err := h.store.CreateRoom(c.Request.Context(), req.Name, store.RoomTypePublic, &uid)
	if err != nil {
		// Check if it's a duplicate name error (SQLite UNIQUE constraint)
		if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "UNIQUE") {
			c.JSON(http.StatusConflict, ErrorResponse{Error: "room with this name already exists"})
			return
		}
		h.log.Error().Err(err).Str("room_name", req.Name).Msg("failed to create room")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	h.log.Info().Str("room_name", room.Name).Int64("room_id", room.ID).Int64("owner_id", uid).Msg("room created successfully")
	c.JSON(http.StatusCreated, RoomResponse{
		ID:        room.ID,
		Name:      room.Name,
		Type:      string(room.Type),
		OwnerID:   room.OwnerID,
		CreatedAt: room.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// ListRooms handles listing accessible rooms.
// GET /api/rooms
func (h *RoomHandlers) ListRooms(c *gin.Context) {
	// Get authenticated user from context
	userID, exists := c.Get(ContextKeyUserID)
	if !exists {
		h.log.Error().Msg("user_id not found in context")
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	uid, ok := userID.(int64)
	if !ok {
		h.log.Error().Msg("invalid user_id type in context")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	rooms, err := h.store.ListRooms(c.Request.Context(), uid)
	if err != nil {
		h.log.Error().Err(err).Int64("user_id", uid).Msg("failed to list rooms")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	// Convert to response format
	response := make([]RoomResponse, 0, len(rooms))
	for _, room := range rooms {
		response = append(response, RoomResponse{
			ID:        room.ID,
			Name:      room.Name,
			Type:      string(room.Type),
			OwnerID:   room.OwnerID,
			CreatedAt: room.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	h.log.Debug().Int64("user_id", uid).Int("room_count", len(rooms)).Msg("rooms listed successfully")
	c.JSON(http.StatusOK, response)
}
