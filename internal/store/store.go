package store

import (
	"context"
	"time"
)

// User represents a user in the system.
type User struct {
	ID           int64
	Username     string
	PasswordHash string
	IsGuest      bool
	SessionID    string // For guest user session tracking
	CreatedAt    time.Time
}

// Room represents a chat room.
type Room struct {
	ID        int64
	Name      string
	Type      RoomType
	OwnerID   *int64  // nil for public rooms, set for private/direct
	DirectKey *string // for direct rooms: "dm:{minUserId}:{maxUserId}"
	CreatedAt time.Time
}

// RoomType defines different types of rooms.
type RoomType string

const (
	RoomTypePublic  RoomType = "public"
	RoomTypePrivate RoomType = "private"
	RoomTypeDirect  RoomType = "direct"
	RoomTypeChannel RoomType = "channel"
)

// Message represents a persisted chat message.
type Message struct {
	ID        int64
	RoomID    int64
	UserID    int64
	Body      string
	CreatedAt time.Time
}

// RoomMember represents room membership.
type RoomMember struct {
	UserID   int64
	RoomID   int64
	JoinedAt time.Time
}

// FriendStatus defines friend relationship status.
type FriendStatus string

const (
	FriendStatusPending  FriendStatus = "pending"
	FriendStatusAccepted FriendStatus = "accepted"
	FriendStatusBlocked  FriendStatus = "blocked"
)

// Friend represents a friend relationship.
type Friend struct {
	ID        int64
	UserID    int64
	FriendID  int64
	Status    FriendStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CallType defines the type of call.
type CallType string

const (
	CallTypeDirect CallType = "direct"
	CallTypeRoom   CallType = "room"
)

// CallStatus defines call status.
type CallStatus string

const (
	CallStatusRinging CallStatus = "ringing"
	CallStatusActive  CallStatus = "active"
	CallStatusEnded   CallStatus = "ended"
	CallStatusFailed  CallStatus = "failed"
)

// CallMode defines the media backend.
type CallMode string

const (
	CallModeLiveKit CallMode = "livekit"
)

// Call represents a voice/video call.
type Call struct {
	ID              string // UUID
	Type            CallType
	Mode            CallMode
	InitiatorUserID int64
	RoomID          *int64
	Status          CallStatus
	ExternalRoomID  *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	EndedAt         *time.Time
}

// CallParticipant represents a user in a call.
type CallParticipant struct {
	ID       int64
	CallID   string
	UserID   int64
	JoinedAt *time.Time
	LeftAt   *time.Time
	Reason   *string
}

// AllowCallsFrom defines who can call a user.
type AllowCallsFrom string

const (
	AllowCallsFromEveryone    AllowCallsFrom = "everyone"
	AllowCallsFromFriendsOnly AllowCallsFrom = "friends_only"
)

// UserStore handles user persistence.
type UserStore interface {
	// CreateUser creates a new user with hashed password.
	CreateUser(ctx context.Context, username, passwordHash string) (*User, error)

	// CreateGuestUser creates a temporary guest user with session ID.
	CreateGuestUser(ctx context.Context, sessionID string) (*User, error)

	// GetUserByID retrieves a user by ID.
	GetUserByID(ctx context.Context, id int64) (*User, error)

	// GetUserByUsername retrieves a user by username.
	GetUserByUsername(ctx context.Context, username string) (*User, error)

	// GetUserBySessionID retrieves a guest user by session ID.
	GetUserBySessionID(ctx context.Context, sessionID string) (*User, error)

	// GetUserCallSettings retrieves user's call privacy settings.
	GetUserCallSettings(ctx context.Context, userID int64) (AllowCallsFrom, error)

	// UpdateUserCallSettings updates user's call privacy settings.
	UpdateUserCallSettings(ctx context.Context, userID int64, setting AllowCallsFrom) error

	// SearchUsers searches for users by username.
	SearchUsers(ctx context.Context, query string) ([]*User, error)
}

// RoomStore handles room persistence.
type RoomStore interface {
	// CreateRoom creates a new room.
	CreateRoom(ctx context.Context, name string, roomType RoomType, ownerID *int64) (*Room, error)

	// CreateDirectRoom creates a direct message room between two users.
	// Handles deduplication via directKey and auto-adds both users as members.
	CreateDirectRoom(ctx context.Context, directKey string, user1ID, user2ID int64) (*Room, error)

	// GetRoomByID retrieves a room by ID.
	GetRoomByID(ctx context.Context, id int64) (*Room, error)

	// GetRoomByName retrieves a room by name.
	GetRoomByName(ctx context.Context, name string) (*Room, error)

	// GetRoomByDirectKey retrieves a direct room by its direct_key.
	GetRoomByDirectKey(ctx context.Context, directKey string) (*Room, error)

	// ListRooms lists all accessible rooms for a user.
	ListRooms(ctx context.Context, userID int64) ([]*Room, error)

	// AddMember adds a user to a room.
	AddMember(ctx context.Context, userID, roomID int64) error

	// RemoveMember removes a user from a room.
	RemoveMember(ctx context.Context, userID, roomID int64) error

	// IsMember checks if user is a member of the room.
	IsMember(ctx context.Context, userID, roomID int64) (bool, error)

	// ListMembers lists all members of a room.
	ListMembers(ctx context.Context, roomID int64) ([]int64, error)
}

// MessageStore handles message persistence.
type MessageStore interface {
	// SaveMessage persists a message to storage.
	SaveMessage(ctx context.Context, msg *Message) error

	// ListMessages retrieves messages from a room with pagination.
	// If beforeID is provided, returns messages older than that ID.
	// Limit determines max number of messages to return.
	ListMessages(ctx context.Context, roomID int64, limit int, beforeID *int64) ([]*Message, error)
}

// FriendStore handles friend persistence.
type FriendStore interface {
	// CreateFriendRequest creates a new friend request (pending status).
	CreateFriendRequest(ctx context.Context, userID, friendID int64) (*Friend, error)

	// UpdateFriendStatus updates the status of a friendship.
	UpdateFriendStatus(ctx context.Context, userID, friendID int64, status FriendStatus) error

	// GetFriendship retrieves a friendship between two users (in either direction).
	GetFriendship(ctx context.Context, userID, friendID int64) (*Friend, error)

	// ListFriends lists friendships for a user, optionally filtered by status.
	ListFriends(ctx context.Context, userID int64, status *FriendStatus) ([]*Friend, error)

	// IsFriend checks if two users are friends (accepted status in either direction).
	IsFriend(ctx context.Context, userID, friendID int64) (bool, error)

	// DeleteFriendship removes a friendship record.
	DeleteFriendship(ctx context.Context, userID, friendID int64) error
}

// CallStore handles call persistence.
type CallStore interface {
	// CreateCall creates a new call.
	CreateCall(ctx context.Context, call *Call) error

	// UpdateCall updates an existing call.
	UpdateCall(ctx context.Context, call *Call) error

	// GetCall retrieves a call by ID.
	GetCall(ctx context.Context, id string) (*Call, error)

	// ListActiveCalls lists active calls (ringing or active) for a user.
	ListActiveCalls(ctx context.Context, userID int64) ([]*Call, error)

	// GetActiveCallForRoom returns an active call for a room, or nil if none exists.
	GetActiveCallForRoom(ctx context.Context, roomID int64) (*Call, error)

	// AddParticipant adds a participant to a call.
	AddParticipant(ctx context.Context, p *CallParticipant) error

	// UpdateParticipant updates a participant record.
	UpdateParticipant(ctx context.Context, p *CallParticipant) error

	// GetParticipant retrieves a participant from a call.
	GetParticipant(ctx context.Context, callID string, userID int64) (*CallParticipant, error)

	// ListParticipants lists all participants in a call.
	ListParticipants(ctx context.Context, callID string) ([]*CallParticipant, error)
}

// Store aggregates all storage interfaces.
type Store interface {
	UserStore
	RoomStore
	MessageStore
	FriendStore
	CallStore

	// Close closes the underlying database connection.
	Close() error
}
