package livekit

import (
	"context"
	"fmt"
	"time"

	"github.com/livekit/protocol/auth"
	"github.com/vovakirdan/wirechat-server/internal/callengine"
	"github.com/vovakirdan/wirechat-server/internal/store"
)

// LiveKitEngine implements callengine.Engine using LiveKit as the media backend.
type LiveKitEngine struct {
	apiKey    string
	apiSecret string
	wsURL     string
}

// New creates a new LiveKitEngine.
func New(apiKey, apiSecret, wsURL string) *LiveKitEngine {
	return &LiveKitEngine{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		wsURL:     wsURL,
	}
}

// CreateCall creates a LiveKit room for the call.
// LiveKit creates rooms on-demand when the first user joins,
// so we just generate the room name here.
func (e *LiveKitEngine) CreateCall(_ context.Context, call *store.Call) (string, error) {
	// Room name format: wirechat-{type}-{callID}
	roomName := fmt.Sprintf("wirechat-%s-%s", call.Type, call.ID)
	return roomName, nil
}

// EndCall terminates the LiveKit room.
// For dev environment, this is a no-op as rooms auto-expire.
// In production, this would call the LiveKit API to delete the room.
func (e *LiveKitEngine) EndCall(_ context.Context, _ *store.Call) error {
	// For dev: no-op, rooms auto-expire when empty
	// Production would use: lksdk.NewRoomServiceClient(host, apiKey, apiSecret).DeleteRoom(...)
	return nil
}

// GenerateJoinInfo creates join credentials for a user to join the call.
func (e *LiveKitEngine) GenerateJoinInfo(_ context.Context, call *store.Call, userID int64, username string) (*callengine.JoinInfo, error) {
	if call.ExternalRoomID == nil {
		return nil, fmt.Errorf("call has no external room ID")
	}

	identity := fmt.Sprintf("user-%d", userID)

	at := auth.NewAccessToken(e.apiKey, e.apiSecret)
	grant := &auth.VideoGrant{
		RoomJoin: true,
		Room:     *call.ExternalRoomID,
	}
	at.SetVideoGrant(grant).
		SetIdentity(identity).
		SetName(username).
		SetValidFor(time.Hour)

	token, err := at.ToJWT()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &callengine.JoinInfo{
		URL:      e.wsURL,
		Token:    token,
		RoomName: *call.ExternalRoomID,
		Identity: identity,
	}, nil
}

// Ensure LiveKitEngine implements callengine.Engine
var _ callengine.Engine = (*LiveKitEngine)(nil)
