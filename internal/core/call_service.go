package core

import (
	"context"

	"github.com/vovakirdan/wirechat-server/internal/callengine"
	"github.com/vovakirdan/wirechat-server/internal/store"
)

// CallService abstracts call business logic for the Hub.
// This interface allows the Hub to process call commands without
// depending directly on the service layer implementation.
type CallService interface {
	// CreateDirectCall initiates a direct call between two users.
	// Returns the created call record.
	CreateDirectCall(ctx context.Context, fromUserID, toUserID int64) (*store.Call, error)

	// CreateRoomCall initiates a call in a chat room.
	// Returns the created call record.
	CreateRoomCall(ctx context.Context, initiatorUserID, roomID int64) (*store.Call, error)

	// GetCall retrieves a call by ID.
	GetCall(ctx context.Context, callID string) (*store.Call, error)

	// GetJoinInfo generates LiveKit credentials for a user to join a call.
	// This also marks the user as joined in call_participants.
	GetJoinInfo(ctx context.Context, callID string, userID int64) (*callengine.JoinInfo, error)

	// EndCall terminates a call for all participants.
	EndCall(ctx context.Context, callID string, byUserID int64) error

	// RejectCall rejects an incoming call.
	RejectCall(ctx context.Context, callID string, byUserID int64, reason string) error

	// LeaveCall marks a participant as having left the call.
	LeaveCall(ctx context.Context, callID string, userID int64) error

	// GetTargetUser returns user info for a direct call target.
	// Returns user_id and username, or error if not found.
	GetTargetUser(ctx context.Context, userID int64) (username string, err error)

	// ListRoomMembers returns user IDs of all members in a room.
	ListRoomMembers(ctx context.Context, roomID int64) ([]int64, error)

	// GetRoomInfo returns room information.
	GetRoomInfo(ctx context.Context, roomID int64) (name string, err error)
}
