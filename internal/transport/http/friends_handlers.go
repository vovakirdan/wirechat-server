package http

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/vovakirdan/wirechat-server/internal/service/friends"
	"github.com/vovakirdan/wirechat-server/internal/store"
)

// FriendsHandlers provides HTTP handlers for friend management endpoints.
type FriendsHandlers struct {
	service *friends.Service
	store   store.Store
	log     *zerolog.Logger
}

// NewFriendsHandlers creates a new friends handlers instance.
func NewFriendsHandlers(svc *friends.Service, st store.Store, logger *zerolog.Logger) *FriendsHandlers {
	return &FriendsHandlers{
		service: svc,
		store:   st,
		log:     logger,
	}
}

// SendFriendRequestRequest represents the request body for sending a friend request.
type SendFriendRequestRequest struct {
	UserID int64 `json:"user_id" binding:"required"`
}

// FriendResponse represents a friend in API responses.
type FriendResponse struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"user_id"`
	FriendID  int64  `json:"friend_id"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	// Additional fields for context
	FriendUsername string `json:"friend_username,omitempty"`
}

// friendToResponse converts a store.Friend to FriendResponse.
func (h *FriendsHandlers) friendToResponse(ctx *gin.Context, f *store.Friend, currentUserID int64) FriendResponse {
	resp := FriendResponse{
		ID:        f.ID,
		UserID:    f.UserID,
		FriendID:  f.FriendID,
		Status:    string(f.Status),
		CreatedAt: f.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: f.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	// Get the other user's username
	otherUserID := f.FriendID
	if f.FriendID == currentUserID {
		otherUserID = f.UserID
	}

	user, err := h.store.GetUserByID(ctx.Request.Context(), otherUserID)
	if err == nil {
		resp.FriendUsername = user.Username
	}

	return resp
}

// SendRequest handles sending a friend request.
// POST /api/friends/requests
func (h *FriendsHandlers) SendRequest(c *gin.Context) {
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

	var req SendFriendRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Debug().Err(err).Msg("invalid send friend request")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	friend, err := h.service.SendRequest(c.Request.Context(), uid, req.UserID)
	if err != nil {
		switch {
		case errors.Is(err, friends.ErrCannotFriendSelf):
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "cannot send friend request to yourself"})
		case errors.Is(err, friends.ErrAlreadyFriends):
			c.JSON(http.StatusConflict, ErrorResponse{Error: "already friends"})
		case errors.Is(err, friends.ErrRequestAlreadyExists):
			c.JSON(http.StatusConflict, ErrorResponse{Error: "friend request already exists"})
		case errors.Is(err, friends.ErrUserNotFound):
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "user not found"})
		default:
			h.log.Error().Err(err).Int64("from_user_id", uid).Int64("to_user_id", req.UserID).Msg("failed to send friend request")
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		}
		return
	}

	h.log.Info().Int64("from_user_id", uid).Int64("to_user_id", req.UserID).Msg("friend request sent")
	c.JSON(http.StatusCreated, h.friendToResponse(c, friend, uid))
}

// ListFriends handles listing accepted friends.
// GET /api/friends
func (h *FriendsHandlers) ListFriends(c *gin.Context) {
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

	friendsList, err := h.service.ListFriends(c.Request.Context(), uid)
	if err != nil {
		h.log.Error().Err(err).Int64("user_id", uid).Msg("failed to list friends")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	response := make([]FriendResponse, 0, len(friendsList))
	for _, f := range friendsList {
		response = append(response, h.friendToResponse(c, f, uid))
	}

	h.log.Debug().Int64("user_id", uid).Int("friend_count", len(friendsList)).Msg("friends listed")
	c.JSON(http.StatusOK, response)
}

// ListPendingRequests handles listing incoming pending friend requests.
// GET /api/friends/requests/incoming
func (h *FriendsHandlers) ListPendingRequests(c *gin.Context) {
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

	requests, err := h.service.ListPendingRequests(c.Request.Context(), uid)
	if err != nil {
		h.log.Error().Err(err).Int64("user_id", uid).Msg("failed to list pending requests")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	response := make([]FriendResponse, 0, len(requests))
	for _, f := range requests {
		response = append(response, h.friendToResponse(c, f, uid))
	}

	h.log.Debug().Int64("user_id", uid).Int("request_count", len(requests)).Msg("pending requests listed")
	c.JSON(http.StatusOK, response)
}

// AcceptRequest handles accepting a friend request.
// POST /api/friends/:userId/accept
func (h *FriendsHandlers) AcceptRequest(c *gin.Context) {
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

	// Parse friend user ID from URL
	fromUserIDStr := c.Param("userId")
	var fromUserID int64
	if _, err := fmt.Sscanf(fromUserIDStr, "%d", &fromUserID); err != nil {
		h.log.Debug().Str("user_id", fromUserIDStr).Msg("invalid user id")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid user id"})
		return
	}

	err := h.service.AcceptRequest(c.Request.Context(), uid, fromUserID)
	if err != nil {
		if errors.Is(err, friends.ErrRequestNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "friend request not found"})
			return
		}
		h.log.Error().Err(err).Int64("user_id", uid).Int64("from_user_id", fromUserID).Msg("failed to accept friend request")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	h.log.Info().Int64("user_id", uid).Int64("from_user_id", fromUserID).Msg("friend request accepted")
	c.JSON(http.StatusOK, gin.H{"message": "friend request accepted"})
}

// RejectRequest handles rejecting a friend request.
// DELETE /api/friends/:userId/reject
func (h *FriendsHandlers) RejectRequest(c *gin.Context) {
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

	// Parse friend user ID from URL
	fromUserIDStr := c.Param("userId")
	var fromUserID int64
	if _, err := fmt.Sscanf(fromUserIDStr, "%d", &fromUserID); err != nil {
		h.log.Debug().Str("user_id", fromUserIDStr).Msg("invalid user id")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid user id"})
		return
	}

	err := h.service.RejectRequest(c.Request.Context(), uid, fromUserID)
	if err != nil {
		if errors.Is(err, friends.ErrRequestNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "friend request not found"})
			return
		}
		h.log.Error().Err(err).Int64("user_id", uid).Int64("from_user_id", fromUserID).Msg("failed to reject friend request")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	h.log.Info().Int64("user_id", uid).Int64("from_user_id", fromUserID).Msg("friend request rejected")
	c.JSON(http.StatusOK, gin.H{"message": "friend request rejected"})
}

// BlockUser handles blocking a user.
// POST /api/friends/:userId/block
func (h *FriendsHandlers) BlockUser(c *gin.Context) {
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

	// Parse target user ID from URL
	targetUserIDStr := c.Param("userId")
	var targetUserID int64
	if _, err := fmt.Sscanf(targetUserIDStr, "%d", &targetUserID); err != nil {
		h.log.Debug().Str("user_id", targetUserIDStr).Msg("invalid user id")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid user id"})
		return
	}

	err := h.service.BlockUser(c.Request.Context(), uid, targetUserID)
	if err != nil {
		switch {
		case errors.Is(err, friends.ErrCannotFriendSelf):
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "cannot block yourself"})
		case errors.Is(err, friends.ErrUserNotFound):
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "user not found"})
		default:
			h.log.Error().Err(err).Int64("user_id", uid).Int64("target_user_id", targetUserID).Msg("failed to block user")
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		}
		return
	}

	h.log.Info().Int64("user_id", uid).Int64("target_user_id", targetUserID).Msg("user blocked")
	c.JSON(http.StatusOK, gin.H{"message": "user blocked"})
}

// UnblockUser handles unblocking a user.
// DELETE /api/friends/:userId/unblock
func (h *FriendsHandlers) UnblockUser(c *gin.Context) {
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

	// Parse target user ID from URL
	targetUserIDStr := c.Param("userId")
	var targetUserID int64
	if _, err := fmt.Sscanf(targetUserIDStr, "%d", &targetUserID); err != nil {
		h.log.Debug().Str("user_id", targetUserIDStr).Msg("invalid user id")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid user id"})
		return
	}

	err := h.service.UnblockUser(c.Request.Context(), uid, targetUserID)
	if err != nil {
		if errors.Is(err, friends.ErrNotBlocked) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "user is not blocked"})
			return
		}
		h.log.Error().Err(err).Int64("user_id", uid).Int64("target_user_id", targetUserID).Msg("failed to unblock user")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	h.log.Info().Int64("user_id", uid).Int64("target_user_id", targetUserID).Msg("user unblocked")
	c.JSON(http.StatusOK, gin.H{"message": "user unblocked"})
}
