package http

import (
	"fmt"
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
	Type string `json:"type,omitempty"` // "public" or "private", defaults to "public"
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

	// Determine room type (default to public)
	roomType := store.RoomTypePublic
	if req.Type != "" {
		switch req.Type {
		case "public":
			roomType = store.RoomTypePublic
		case "private":
			roomType = store.RoomTypePrivate
		default:
			h.log.Debug().Str("type", req.Type).Msg("invalid room type")
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid room type, must be 'public' or 'private'"})
			return
		}
	}

	// Create room with current user as owner
	room, err := h.store.CreateRoom(c.Request.Context(), req.Name, roomType, &uid)
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

	// If private room, auto-add creator to room_members
	if roomType == store.RoomTypePrivate {
		if err := h.store.AddMember(c.Request.Context(), uid, room.ID); err != nil {
			h.log.Error().Err(err).Int64("room_id", room.ID).Int64("user_id", uid).Msg("failed to add creator to room_members")
			// Don't fail the request, just log the error
		}
	}

	h.log.Info().
		Str("room_name", room.Name).
		Int64("room_id", room.ID).
		Int64("owner_id", uid).
		Str("type", string(roomType)).
		Msg("room created successfully")
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

// JoinRoom handles joining a room.
// POST /api/rooms/:id/join
func (h *RoomHandlers) JoinRoom(c *gin.Context) {
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

	// Parse room ID from URL
	roomID := c.Param("id")
	var rid int64
	if _, err := fmt.Sscanf(roomID, "%d", &rid); err != nil {
		h.log.Debug().Str("room_id", roomID).Msg("invalid room id")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid room id"})
		return
	}

	// Get room from database
	room, err := h.store.GetRoomByID(c.Request.Context(), rid)
	if err != nil {
		h.log.Warn().Err(err).Int64("room_id", rid).Msg("room not found")
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "room not found"})
		return
	}

	// Check if room is private (private rooms can't be joined via REST)
	if room.Type == store.RoomTypePrivate {
		h.log.Warn().Int64("room_id", rid).Int64("user_id", uid).Msg("cannot join private room via REST")
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "private rooms cannot be joined directly"})
		return
	}

	// Add user to room_members
	if err := h.store.AddMember(c.Request.Context(), uid, rid); err != nil {
		h.log.Error().Err(err).Int64("room_id", rid).Int64("user_id", uid).Msg("failed to add member")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	h.log.Info().Int64("room_id", rid).Int64("user_id", uid).Msg("user joined room")
	c.JSON(http.StatusOK, gin.H{"message": "joined room successfully"})
}

// LeaveRoom handles leaving a room.
// DELETE /api/rooms/:id/leave
func (h *RoomHandlers) LeaveRoom(c *gin.Context) {
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

	// Parse room ID from URL
	roomID := c.Param("id")
	var rid int64
	if _, err := fmt.Sscanf(roomID, "%d", &rid); err != nil {
		h.log.Debug().Str("room_id", roomID).Msg("invalid room id")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid room id"})
		return
	}

	// Remove user from room_members
	if err := h.store.RemoveMember(c.Request.Context(), uid, rid); err != nil {
		h.log.Error().Err(err).Int64("room_id", rid).Int64("user_id", uid).Msg("failed to remove member")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	h.log.Info().Int64("room_id", rid).Int64("user_id", uid).Msg("user left room")
	c.JSON(http.StatusOK, gin.H{"message": "left room successfully"})
}

// AddMemberRequest represents the add member request body.
type AddMemberRequest struct {
	UserID int64 `json:"user_id" binding:"required"`
}

// AddMember handles adding a member to a room (owner only).
// POST /api/rooms/:id/members
func (h *RoomHandlers) AddMember(c *gin.Context) {
	// Get authenticated user from context
	userID, exists := c.Get(ContextKeyUserID)
	if !exists {
		h.log.Error().Msg("user_id not found in context")
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	currentUID, ok := userID.(int64)
	if !ok {
		h.log.Error().Msg("invalid user_id type in context")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	// Parse room ID from URL
	roomID := c.Param("id")
	var rid int64
	if _, err := fmt.Sscanf(roomID, "%d", &rid); err != nil {
		h.log.Debug().Str("room_id", roomID).Msg("invalid room id")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid room id"})
		return
	}

	// Get room from database
	room, err := h.store.GetRoomByID(c.Request.Context(), rid)
	if err != nil {
		h.log.Warn().Err(err).Int64("room_id", rid).Msg("room not found")
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "room not found"})
		return
	}

	// Check if current user is the owner
	if room.OwnerID == nil || *room.OwnerID != currentUID {
		h.log.Warn().Int64("room_id", rid).Int64("user_id", currentUID).Msg("user is not room owner")
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "only room owner can add members"})
		return
	}

	// Parse request body
	var req AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Debug().Err(err).Msg("invalid add member request")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	// Add member to room
	if err := h.store.AddMember(c.Request.Context(), req.UserID, rid); err != nil {
		h.log.Error().Err(err).Int64("room_id", rid).Int64("user_id", req.UserID).Msg("failed to add member")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	h.log.Info().Int64("room_id", rid).Int64("user_id", req.UserID).Int64("added_by", currentUID).Msg("member added to room")
	c.JSON(http.StatusOK, gin.H{"message": "member added successfully"})
}

// RemoveMember handles removing a member from a room (owner only).
// DELETE /api/rooms/:id/members/:userId
func (h *RoomHandlers) RemoveMember(c *gin.Context) {
	// Get authenticated user from context
	userID, exists := c.Get(ContextKeyUserID)
	if !exists {
		h.log.Error().Msg("user_id not found in context")
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
		return
	}

	currentUID, ok := userID.(int64)
	if !ok {
		h.log.Error().Msg("invalid user_id type in context")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	// Parse room ID from URL
	roomID := c.Param("id")
	var rid int64
	if _, err := fmt.Sscanf(roomID, "%d", &rid); err != nil {
		h.log.Debug().Str("room_id", roomID).Msg("invalid room id")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid room id"})
		return
	}

	// Parse target user ID from URL
	targetUserID := c.Param("userId")
	var targetUID int64
	if _, err := fmt.Sscanf(targetUserID, "%d", &targetUID); err != nil {
		h.log.Debug().Str("user_id", targetUserID).Msg("invalid user id")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid user id"})
		return
	}

	// Get room from database
	room, err := h.store.GetRoomByID(c.Request.Context(), rid)
	if err != nil {
		h.log.Warn().Err(err).Int64("room_id", rid).Msg("room not found")
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "room not found"})
		return
	}

	// Check if current user is the owner
	if room.OwnerID == nil || *room.OwnerID != currentUID {
		h.log.Warn().Int64("room_id", rid).Int64("user_id", currentUID).Msg("user is not room owner")
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "only room owner can remove members"})
		return
	}

	// Remove member from room
	if err := h.store.RemoveMember(c.Request.Context(), targetUID, rid); err != nil {
		h.log.Error().Err(err).Int64("room_id", rid).Int64("user_id", targetUID).Msg("failed to remove member")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	h.log.Info().Int64("room_id", rid).Int64("user_id", targetUID).Int64("removed_by", currentUID).Msg("member removed from room")
	c.JSON(http.StatusOK, gin.H{"message": "member removed successfully"})
}
