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
	OwnerID   *int64 // nil for public rooms, set for private/direct
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
}

// RoomStore handles room persistence.
type RoomStore interface {
	// CreateRoom creates a new room.
	CreateRoom(ctx context.Context, name string, roomType RoomType, ownerID *int64) (*Room, error)

	// GetRoomByID retrieves a room by ID.
	GetRoomByID(ctx context.Context, id int64) (*Room, error)

	// GetRoomByName retrieves a room by name.
	GetRoomByName(ctx context.Context, name string) (*Room, error)

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

// Store aggregates all storage interfaces.
type Store interface {
	UserStore
	RoomStore
	MessageStore

	// Close closes the underlying database connection.
	Close() error
}
