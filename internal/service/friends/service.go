package friends

import (
	"context"
	"errors"
	"fmt"

	"github.com/vovakirdan/wirechat-server/internal/store"
)

// Common errors for friend operations.
var (
	ErrCannotFriendSelf     = errors.New("cannot send friend request to yourself")
	ErrAlreadyFriends       = errors.New("already friends")
	ErrRequestAlreadyExists = errors.New("friend request already exists")
	ErrRequestNotFound      = errors.New("friend request not found")
	ErrUserNotFound         = errors.New("user not found")
	ErrNotBlocked           = errors.New("user is not blocked")
)

// Service provides friend management business logic.
type Service struct {
	store store.Store
}

// New creates a new FriendService.
func New(st store.Store) *Service {
	return &Service{
		store: st,
	}
}

// SendRequest sends a friend request from one user to another.
func (s *Service) SendRequest(ctx context.Context, fromUserID, toUserID int64) (*store.Friend, error) {
	// Cannot friend yourself
	if fromUserID == toUserID {
		return nil, ErrCannotFriendSelf
	}

	// Check if target user exists
	_, err := s.store.GetUserByID(ctx, toUserID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	// Check if friendship already exists
	existing, err := s.store.GetFriendship(ctx, fromUserID, toUserID)
	if err == nil {
		// Friendship exists
		switch existing.Status {
		case store.FriendStatusAccepted:
			return nil, ErrAlreadyFriends
		case store.FriendStatusPending:
			return nil, ErrRequestAlreadyExists
		case store.FriendStatusBlocked:
			// If blocked by the target, still allow creating a request
			// (it won't be visible to them until unblocked)
			if existing.UserID == toUserID {
				return nil, fmt.Errorf("you are blocked by this user")
			}
			// If we blocked them, unblock first
			return nil, fmt.Errorf("unblock user first before sending friend request")
		}
	}

	// Create the friend request
	friend, err := s.store.CreateFriendRequest(ctx, fromUserID, toUserID)
	if err != nil {
		return nil, fmt.Errorf("create friend request: %w", err)
	}

	return friend, nil
}

// AcceptRequest accepts a pending friend request.
func (s *Service) AcceptRequest(ctx context.Context, userID, fromUserID int64) error {
	// Get the friendship - must be a pending request TO userID
	existing, err := s.store.GetFriendship(ctx, fromUserID, userID)
	if err != nil {
		return ErrRequestNotFound
	}

	// Must be pending and directed to the accepting user
	if existing.Status != store.FriendStatusPending {
		return ErrRequestNotFound
	}
	if existing.FriendID != userID {
		return ErrRequestNotFound
	}

	// Update status to accepted
	err = s.store.UpdateFriendStatus(ctx, existing.UserID, existing.FriendID, store.FriendStatusAccepted)
	if err != nil {
		return fmt.Errorf("accept request: %w", err)
	}

	return nil
}

// RejectRequest rejects a pending friend request.
func (s *Service) RejectRequest(ctx context.Context, userID, fromUserID int64) error {
	// Get the friendship - must be a pending request TO userID
	existing, err := s.store.GetFriendship(ctx, fromUserID, userID)
	if err != nil {
		return ErrRequestNotFound
	}

	// Must be pending and directed to the rejecting user
	if existing.Status != store.FriendStatusPending {
		return ErrRequestNotFound
	}
	if existing.FriendID != userID {
		return ErrRequestNotFound
	}

	// Delete the friendship record
	err = s.store.DeleteFriendship(ctx, existing.UserID, existing.FriendID)
	if err != nil {
		return fmt.Errorf("reject request: %w", err)
	}

	return nil
}

// BlockUser blocks another user.
func (s *Service) BlockUser(ctx context.Context, userID, targetUserID int64) error {
	if userID == targetUserID {
		return ErrCannotFriendSelf
	}

	// Check if target user exists
	_, userErr := s.store.GetUserByID(ctx, targetUserID)
	if userErr != nil {
		return ErrUserNotFound
	}

	// Check if friendship exists
	existing, err := s.store.GetFriendship(ctx, userID, targetUserID)
	if err == nil {
		// Update existing record
		// If it's our record (user_id = userID), update it
		if existing.UserID == userID {
			return s.store.UpdateFriendStatus(ctx, userID, targetUserID, store.FriendStatusBlocked)
		}
		// If it's their record (user_id = targetUserID), delete it and create our own
		if deleteErr := s.store.DeleteFriendship(ctx, existing.UserID, existing.FriendID); deleteErr != nil {
			return fmt.Errorf("delete existing friendship: %w", deleteErr)
		}
	}

	// Create a new blocked record
	_, err = s.store.CreateFriendRequest(ctx, userID, targetUserID)
	if err != nil {
		return fmt.Errorf("create block record: %w", err)
	}

	return s.store.UpdateFriendStatus(ctx, userID, targetUserID, store.FriendStatusBlocked)
}

// UnblockUser unblocks a previously blocked user.
func (s *Service) UnblockUser(ctx context.Context, userID, targetUserID int64) error {
	existing, err := s.store.GetFriendship(ctx, userID, targetUserID)
	if err != nil {
		return ErrNotBlocked
	}

	// Must be blocked by the current user
	if existing.Status != store.FriendStatusBlocked || existing.UserID != userID {
		return ErrNotBlocked
	}

	// Delete the block record
	return s.store.DeleteFriendship(ctx, userID, targetUserID)
}

// ListFriends returns all accepted friends for a user.
func (s *Service) ListFriends(ctx context.Context, userID int64) ([]*store.Friend, error) {
	status := store.FriendStatusAccepted
	friends, err := s.store.ListFriends(ctx, userID, &status)
	if err != nil {
		return nil, fmt.Errorf("list friends: %w", err)
	}
	return friends, nil
}

// ListPendingRequests returns incoming pending friend requests for a user.
func (s *Service) ListPendingRequests(ctx context.Context, userID int64) ([]*store.Friend, error) {
	status := store.FriendStatusPending
	all, err := s.store.ListFriends(ctx, userID, &status)
	if err != nil {
		return nil, fmt.Errorf("list pending requests: %w", err)
	}

	// Filter to only incoming requests (friend_id = userID)
	var incoming []*store.Friend
	for _, f := range all {
		if f.FriendID == userID {
			incoming = append(incoming, f)
		}
	}

	return incoming, nil
}

// IsFriend checks if two users are friends (accepted status).
func (s *Service) IsFriend(ctx context.Context, userID, friendID int64) (bool, error) {
	return s.store.IsFriend(ctx, userID, friendID)
}
