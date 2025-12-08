package callengine

import (
	"context"

	"github.com/vovakirdan/wirechat-server/internal/store"
)

// JoinInfo contains information needed to join a call.
type JoinInfo struct {
	URL      string `json:"url"`       // WebSocket URL (e.g., ws://localhost:7880)
	Token    string `json:"token"`     // JWT token for LiveKit
	RoomName string `json:"room_name"` // LiveKit room name
	Identity string `json:"identity"`  // User identity in the room
}

// Engine abstracts the media backend for calls.
type Engine interface {
	// CreateCall creates a media room for the call.
	// Returns external room ID to store in Call.ExternalRoomID.
	CreateCall(ctx context.Context, call *store.Call) (externalRoomID string, err error)

	// EndCall terminates the media room.
	EndCall(ctx context.Context, call *store.Call) error

	// GenerateJoinInfo creates join credentials for a user.
	GenerateJoinInfo(ctx context.Context, call *store.Call, userID int64, username string) (*JoinInfo, error)
}
