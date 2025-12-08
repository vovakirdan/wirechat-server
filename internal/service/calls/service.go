package calls

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vovakirdan/wirechat-server/internal/callengine"
	"github.com/vovakirdan/wirechat-server/internal/service/friends"
	"github.com/vovakirdan/wirechat-server/internal/store"
)

// Common errors for call operations.
var (
	ErrUserNotFound      = errors.New("user not found")
	ErrCallNotFound      = errors.New("call not found")
	ErrCallEnded         = errors.New("call has ended")
	ErrNotParticipant    = errors.New("not a participant in this call")
	ErrNotFriend         = errors.New("cannot call user: not friends")
	ErrCallsNotAllowed   = errors.New("user does not accept calls from non-friends")
	ErrRoomNotFound      = errors.New("room not found")
	ErrNotRoomMember     = errors.New("not a member of this room")
	ErrCannotCallSelf    = errors.New("cannot call yourself")
	ErrLiveKitNotEnabled = errors.New("livekit is not enabled")
)

// Service provides call management business logic.
type Service struct {
	store   store.Store
	engine  callengine.Engine
	friends *friends.Service
}

// New creates a new CallService.
// engine can be nil if LiveKit is not enabled.
func New(st store.Store, engine callengine.Engine, friendsSvc *friends.Service) *Service {
	return &Service{
		store:   st,
		engine:  engine,
		friends: friendsSvc,
	}
}

// CreateDirectCall creates a direct call between two users.
func (s *Service) CreateDirectCall(ctx context.Context, fromUserID, toUserID int64) (*store.Call, error) {
	if s.engine == nil {
		return nil, ErrLiveKitNotEnabled
	}

	if fromUserID == toUserID {
		return nil, ErrCannotCallSelf
	}

	// Check if target user exists
	toUser, err := s.store.GetUserByID(ctx, toUserID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	// Check call privacy settings
	setting, settingErr := s.store.GetUserCallSettings(ctx, toUserID)
	if settingErr != nil {
		return nil, fmt.Errorf("get call settings: %w", settingErr)
	}

	if setting == store.AllowCallsFromFriendsOnly {
		// Check if they are friends
		isFriend, friendErr := s.friends.IsFriend(ctx, fromUserID, toUserID)
		if friendErr != nil {
			return nil, fmt.Errorf("check friendship: %w", friendErr)
		}
		if !isFriend {
			return nil, ErrCallsNotAllowed
		}
	}

	// Generate call ID
	callID := uuid.New().String()

	// Create call record
	call := &store.Call{
		ID:              callID,
		Type:            store.CallTypeDirect,
		Mode:            store.CallModeLiveKit,
		InitiatorUserID: fromUserID,
		Status:          store.CallStatusRinging,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Create external room via engine
	externalRoomID, err := s.engine.CreateCall(ctx, call)
	if err != nil {
		return nil, fmt.Errorf("create external room: %w", err)
	}
	call.ExternalRoomID = &externalRoomID

	// Save call to database
	if err := s.store.CreateCall(ctx, call); err != nil {
		return nil, fmt.Errorf("save call: %w", err)
	}

	// Add both users as participants
	participants := []*store.CallParticipant{
		{CallID: callID, UserID: fromUserID},
		{CallID: callID, UserID: toUserID},
	}

	for _, p := range participants {
		if err := s.store.AddParticipant(ctx, p); err != nil {
			return nil, fmt.Errorf("add participant %d: %w", p.UserID, err)
		}
	}

	_ = toUser // suppress unused variable warning

	return call, nil
}

// CreateRoomCall creates a call in a chat room.
func (s *Service) CreateRoomCall(ctx context.Context, initiatorUserID, roomID int64) (*store.Call, error) {
	if s.engine == nil {
		return nil, ErrLiveKitNotEnabled
	}

	// Check if room exists
	room, err := s.store.GetRoomByID(ctx, roomID)
	if err != nil {
		return nil, ErrRoomNotFound
	}

	// Check if user is a member of the room
	isMember, err := s.store.IsMember(ctx, initiatorUserID, roomID)
	if err != nil {
		return nil, fmt.Errorf("check membership: %w", err)
	}
	if !isMember {
		return nil, ErrNotRoomMember
	}

	// Generate call ID
	callID := uuid.New().String()

	// Create call record
	call := &store.Call{
		ID:              callID,
		Type:            store.CallTypeRoom,
		Mode:            store.CallModeLiveKit,
		InitiatorUserID: initiatorUserID,
		RoomID:          &room.ID,
		Status:          store.CallStatusRinging,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Create external room via engine
	externalRoomID, err := s.engine.CreateCall(ctx, call)
	if err != nil {
		return nil, fmt.Errorf("create external room: %w", err)
	}
	call.ExternalRoomID = &externalRoomID

	// Save call to database
	if err := s.store.CreateCall(ctx, call); err != nil {
		return nil, fmt.Errorf("save call: %w", err)
	}

	// Add initiator as participant
	p := &store.CallParticipant{
		CallID: callID,
		UserID: initiatorUserID,
	}
	if err := s.store.AddParticipant(ctx, p); err != nil {
		return nil, fmt.Errorf("add initiator participant: %w", err)
	}

	return call, nil
}

// GetCall retrieves a call by ID.
func (s *Service) GetCall(ctx context.Context, callID string) (*store.Call, error) {
	call, err := s.store.GetCall(ctx, callID)
	if err != nil {
		return nil, ErrCallNotFound
	}
	return call, nil
}

// GetJoinInfo returns join information for a user to join a call.
func (s *Service) GetJoinInfo(ctx context.Context, callID string, userID int64) (*callengine.JoinInfo, error) {
	if s.engine == nil {
		return nil, ErrLiveKitNotEnabled
	}

	// Get the call
	call, err := s.store.GetCall(ctx, callID)
	if err != nil {
		return nil, ErrCallNotFound
	}

	// Check if call is still active
	if call.Status == store.CallStatusEnded || call.Status == store.CallStatusFailed {
		return nil, ErrCallEnded
	}

	// Check if user is a participant
	participant, participantErr := s.store.GetParticipant(ctx, callID, userID)
	if participantErr != nil {
		// For room calls, check if user is a room member and add them as participant
		if call.Type == store.CallTypeRoom && call.RoomID != nil {
			isMember, memberErr := s.store.IsMember(ctx, userID, *call.RoomID)
			if memberErr != nil {
				return nil, ErrNotParticipant
			}
			if !isMember {
				return nil, ErrNotParticipant
			}

			// Add user as participant
			participant = &store.CallParticipant{
				CallID: callID,
				UserID: userID,
			}
			if addErr := s.store.AddParticipant(ctx, participant); addErr != nil {
				return nil, fmt.Errorf("add participant: %w", addErr)
			}
		} else {
			return nil, ErrNotParticipant
		}
	}

	// Update joined_at if not set
	if participant.JoinedAt == nil {
		now := time.Now()
		participant.JoinedAt = &now
		//nolint:errcheck // Non-fatal error, best effort update
		s.store.UpdateParticipant(ctx, participant)
	}

	// Update call status to active if it was ringing
	if call.Status == store.CallStatusRinging {
		call.Status = store.CallStatusActive
		call.UpdatedAt = time.Now()
		//nolint:errcheck // Non-fatal error, best effort update
		s.store.UpdateCall(ctx, call)
	}

	// Get user info for display name
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	// Generate join info via engine
	joinInfo, err := s.engine.GenerateJoinInfo(ctx, call, userID, user.Username)
	if err != nil {
		return nil, fmt.Errorf("generate join info: %w", err)
	}

	return joinInfo, nil
}

// EndCall ends an active call.
func (s *Service) EndCall(ctx context.Context, callID string, byUserID int64) error {
	// Get the call
	call, err := s.store.GetCall(ctx, callID)
	if err != nil {
		return ErrCallNotFound
	}

	// Check if call is already ended
	if call.Status == store.CallStatusEnded || call.Status == store.CallStatusFailed {
		return nil // Already ended, idempotent
	}

	// Check if user is a participant or initiator
	_, participantErr := s.store.GetParticipant(ctx, callID, byUserID)
	if participantErr != nil && call.InitiatorUserID != byUserID {
		return ErrNotParticipant
	}

	// Update call status
	now := time.Now()
	call.Status = store.CallStatusEnded
	call.EndedAt = &now
	call.UpdatedAt = now

	if updateErr := s.store.UpdateCall(ctx, call); updateErr != nil {
		return fmt.Errorf("update call: %w", updateErr)
	}

	// End the external room if engine is available
	if s.engine != nil {
		//nolint:errcheck // Non-fatal error, best effort cleanup
		s.engine.EndCall(ctx, call)
	}

	return nil
}

// ListActiveCalls returns active calls for a user.
func (s *Service) ListActiveCalls(ctx context.Context, userID int64) ([]*store.Call, error) {
	calls, err := s.store.ListActiveCalls(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list active calls: %w", err)
	}
	return calls, nil
}

// RejectCall rejects an incoming call.
func (s *Service) RejectCall(ctx context.Context, callID string, byUserID int64, reason string) error {
	call, err := s.store.GetCall(ctx, callID)
	if err != nil {
		return ErrCallNotFound
	}

	if call.Status != store.CallStatusRinging {
		return ErrCallEnded
	}

	// Verify user is a participant
	_, err = s.store.GetParticipant(ctx, callID, byUserID)
	if err != nil {
		return ErrNotParticipant
	}

	// Update call status to ended
	now := time.Now()
	call.Status = store.CallStatusEnded
	call.EndedAt = &now
	call.UpdatedAt = now

	if err := s.store.UpdateCall(ctx, call); err != nil {
		return fmt.Errorf("update call: %w", err)
	}

	// Update participant left_at
	participant, _ := s.store.GetParticipant(ctx, callID, byUserID)
	if participant != nil {
		participant.LeftAt = &now
		if reason != "" {
			participant.Reason = &reason
		}
		//nolint:errcheck // Non-fatal
		s.store.UpdateParticipant(ctx, participant)
	}

	return nil
}

// LeaveCall marks a participant as having left the call.
func (s *Service) LeaveCall(ctx context.Context, callID string, userID int64) error {
	participant, err := s.store.GetParticipant(ctx, callID, userID)
	if err != nil {
		return ErrNotParticipant
	}

	now := time.Now()
	participant.LeftAt = &now
	reason := "left"
	participant.Reason = &reason

	if err := s.store.UpdateParticipant(ctx, participant); err != nil {
		return fmt.Errorf("update participant: %w", err)
	}

	// Check if all participants have left
	// If so, end the call
	participants, _ := s.store.ListParticipants(ctx, callID)
	allLeft := true
	for _, p := range participants {
		if p.LeftAt == nil {
			allLeft = false
			break
		}
	}

	if allLeft {
		call, err := s.store.GetCall(ctx, callID)
		if err == nil && call.Status != store.CallStatusEnded {
			call.Status = store.CallStatusEnded
			call.EndedAt = &now
			call.UpdatedAt = now
			//nolint:errcheck // Non-fatal
			s.store.UpdateCall(ctx, call)
		}
	}

	return nil
}

// GetTargetUser returns the username for a user ID.
func (s *Service) GetTargetUser(ctx context.Context, userID int64) (string, error) {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return "", ErrUserNotFound
	}
	return user.Username, nil
}

// ListRoomMembers returns user IDs of all members in a room.
func (s *Service) ListRoomMembers(ctx context.Context, roomID int64) ([]int64, error) {
	members, err := s.store.ListMembers(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	return members, nil
}

// GetRoomInfo returns the room name for a room ID.
func (s *Service) GetRoomInfo(ctx context.Context, roomID int64) (string, error) {
	room, err := s.store.GetRoomByID(ctx, roomID)
	if err != nil {
		return "", ErrRoomNotFound
	}
	return room.Name, nil
}
