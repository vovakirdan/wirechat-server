package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/vovakirdan/wirechat-server/internal/service/calls"
	"github.com/vovakirdan/wirechat-server/internal/store"
)

// CallsHandlers provides HTTP handlers for call management endpoints.
type CallsHandlers struct {
	service *calls.Service
	log     *zerolog.Logger
}

// NewCallsHandlers creates a new calls handlers instance.
func NewCallsHandlers(svc *calls.Service, logger *zerolog.Logger) *CallsHandlers {
	return &CallsHandlers{
		service: svc,
		log:     logger,
	}
}

// CreateDirectCallRequest represents the request body for creating a direct call.
type CreateDirectCallRequest struct {
	ToUserID int64 `json:"to_user_id" binding:"required"`
}

// CreateRoomCallRequest represents the request body for creating a room call.
type CreateRoomCallRequest struct {
	RoomID int64 `json:"room_id" binding:"required"`
}

// CallResponse represents a call in API responses.
type CallResponse struct {
	ID              string  `json:"id"`
	Type            string  `json:"type"`
	Mode            string  `json:"mode"`
	InitiatorUserID int64   `json:"initiator_user_id"`
	RoomID          *int64  `json:"room_id,omitempty"`
	Status          string  `json:"status"`
	ExternalRoomID  *string `json:"external_room_id,omitempty"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
	EndedAt         *string `json:"ended_at,omitempty"`
}

// JoinInfoResponse represents join information in API responses.
type JoinInfoResponse struct {
	URL      string `json:"url"`
	Token    string `json:"token"`
	RoomName string `json:"room_name"`
	Identity string `json:"identity"`
}

// callToResponse converts a store.Call to CallResponse.
func callToResponse(c *store.Call) CallResponse {
	resp := CallResponse{
		ID:              c.ID,
		Type:            string(c.Type),
		Mode:            string(c.Mode),
		InitiatorUserID: c.InitiatorUserID,
		RoomID:          c.RoomID,
		Status:          string(c.Status),
		ExternalRoomID:  c.ExternalRoomID,
		CreatedAt:       c.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:       c.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if c.EndedAt != nil {
		endedAt := c.EndedAt.Format("2006-01-02T15:04:05Z07:00")
		resp.EndedAt = &endedAt
	}
	return resp
}

// CreateDirectCall handles creating a direct call between two users.
// POST /api/calls/direct
func (h *CallsHandlers) CreateDirectCall(c *gin.Context) {
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

	var req CreateDirectCallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Debug().Err(err).Msg("invalid create direct call request")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	call, err := h.service.CreateDirectCall(c.Request.Context(), uid, req.ToUserID)
	if err != nil {
		switch {
		case errors.Is(err, calls.ErrCannotCallSelf):
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "cannot call yourself"})
		case errors.Is(err, calls.ErrUserNotFound):
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "user not found"})
		case errors.Is(err, calls.ErrCallsNotAllowed):
			c.JSON(http.StatusForbidden, ErrorResponse{Error: "user does not accept calls from non-friends"})
		case errors.Is(err, calls.ErrLiveKitNotEnabled):
			c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "calls are not available"})
		default:
			h.log.Error().Err(err).Int64("from_user_id", uid).Int64("to_user_id", req.ToUserID).Msg("failed to create direct call")
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		}
		return
	}

	h.log.Info().Str("call_id", call.ID).Int64("from_user_id", uid).Int64("to_user_id", req.ToUserID).Msg("direct call created")
	c.JSON(http.StatusCreated, callToResponse(call))
}

// CreateRoomCall handles creating a call in a chat room.
// POST /api/calls/room
func (h *CallsHandlers) CreateRoomCall(c *gin.Context) {
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

	var req CreateRoomCallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Debug().Err(err).Msg("invalid create room call request")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	call, err := h.service.CreateRoomCall(c.Request.Context(), uid, req.RoomID)
	if err != nil {
		switch {
		case errors.Is(err, calls.ErrRoomNotFound):
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "room not found"})
		case errors.Is(err, calls.ErrNotRoomMember):
			c.JSON(http.StatusForbidden, ErrorResponse{Error: "not a member of this room"})
		case errors.Is(err, calls.ErrLiveKitNotEnabled):
			c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "calls are not available"})
		default:
			h.log.Error().Err(err).Int64("user_id", uid).Int64("room_id", req.RoomID).Msg("failed to create room call")
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		}
		return
	}

	h.log.Info().Str("call_id", call.ID).Int64("user_id", uid).Int64("room_id", req.RoomID).Msg("room call created")
	c.JSON(http.StatusCreated, callToResponse(call))
}

// GetCall handles retrieving a call by ID.
// GET /api/calls/:id
func (h *CallsHandlers) GetCall(c *gin.Context) {
	callID := c.Param("id")
	if callID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "call id required"})
		return
	}

	call, err := h.service.GetCall(c.Request.Context(), callID)
	if err != nil {
		if errors.Is(err, calls.ErrCallNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "call not found"})
			return
		}
		h.log.Error().Err(err).Str("call_id", callID).Msg("failed to get call")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	c.JSON(http.StatusOK, callToResponse(call))
}

// GetJoinInfo handles retrieving join information for a call.
// GET /api/calls/:id/join
func (h *CallsHandlers) GetJoinInfo(c *gin.Context) {
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

	callID := c.Param("id")
	if callID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "call id required"})
		return
	}

	joinInfo, err := h.service.GetJoinInfo(c.Request.Context(), callID, uid)
	if err != nil {
		switch {
		case errors.Is(err, calls.ErrCallNotFound):
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "call not found"})
		case errors.Is(err, calls.ErrCallEnded):
			c.JSON(http.StatusGone, ErrorResponse{Error: "call has ended"})
		case errors.Is(err, calls.ErrNotParticipant):
			c.JSON(http.StatusForbidden, ErrorResponse{Error: "not a participant in this call"})
		case errors.Is(err, calls.ErrLiveKitNotEnabled):
			c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "calls are not available"})
		default:
			h.log.Error().Err(err).Str("call_id", callID).Int64("user_id", uid).Msg("failed to get join info")
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		}
		return
	}

	h.log.Debug().Str("call_id", callID).Int64("user_id", uid).Msg("join info retrieved")
	c.JSON(http.StatusOK, JoinInfoResponse{
		URL:      joinInfo.URL,
		Token:    joinInfo.Token,
		RoomName: joinInfo.RoomName,
		Identity: joinInfo.Identity,
	})
}

// EndCall handles ending a call.
// PUT /api/calls/:id/end
func (h *CallsHandlers) EndCall(c *gin.Context) {
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

	callID := c.Param("id")
	if callID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "call id required"})
		return
	}

	err := h.service.EndCall(c.Request.Context(), callID, uid)
	if err != nil {
		switch {
		case errors.Is(err, calls.ErrCallNotFound):
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "call not found"})
		case errors.Is(err, calls.ErrNotParticipant):
			c.JSON(http.StatusForbidden, ErrorResponse{Error: "not a participant in this call"})
		default:
			h.log.Error().Err(err).Str("call_id", callID).Int64("user_id", uid).Msg("failed to end call")
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		}
		return
	}

	h.log.Info().Str("call_id", callID).Int64("user_id", uid).Msg("call ended")
	c.JSON(http.StatusOK, gin.H{"message": "call ended"})
}

// ListActiveCalls handles listing active calls for the current user.
// GET /api/calls/active
func (h *CallsHandlers) ListActiveCalls(c *gin.Context) {
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

	callsList, err := h.service.ListActiveCalls(c.Request.Context(), uid)
	if err != nil {
		h.log.Error().Err(err).Int64("user_id", uid).Msg("failed to list active calls")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	response := make([]CallResponse, 0, len(callsList))
	for _, call := range callsList {
		response = append(response, callToResponse(call))
	}

	h.log.Debug().Int64("user_id", uid).Int("call_count", len(callsList)).Msg("active calls listed")
	c.JSON(http.StatusOK, response)
}
