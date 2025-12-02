package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	"github.com/vovakirdan/wirechat-server/internal/store"
)

// SQLiteStore implements store.Store for SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// New creates a new SQLite store.
// dbPath is the path to the SQLite database file.
func New(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	// Set connection pool limits
	db.SetMaxOpenConns(1) // SQLite works best with single connection
	db.SetMaxIdleConns(1)

	return &SQLiteStore{db: db}, nil
}

// NewWithSetup creates a new SQLite store and runs a setup function.
// Useful for tests to apply schema without migrations.
func NewWithSetup(dbPath string, setup func(*sql.DB) error) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Set connection pool limits before setup
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Run setup function (e.g., apply schema)
	if setup != nil {
		if err := setup(db); err != nil {
			db.Close()
			return nil, fmt.Errorf("setup: %w", err)
		}
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// ==== UserStore implementation ====

// CreateUser creates a new user with hashed password.
func (s *SQLiteStore) CreateUser(ctx context.Context, username, passwordHash string) (*store.User, error) {
	query := `
		INSERT INTO users (username, password_hash, is_guest)
		VALUES (?, ?, 0)
	`
	result, err := s.db.ExecContext(ctx, query, username, passwordHash)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get last insert id: %w", err)
	}

	return s.GetUserByID(ctx, id)
}

// CreateGuestUser creates a temporary guest user with session ID.
func (s *SQLiteStore) CreateGuestUser(ctx context.Context, sessionID string) (*store.User, error) {
	query := `
		INSERT INTO users (username, password_hash, is_guest, session_id)
		VALUES (?, '', 1, ?)
	`
	// Generate unique guest username
	guestUsername := "guest_" + sessionID[:8]

	result, err := s.db.ExecContext(ctx, query, guestUsername, sessionID)
	if err != nil {
		return nil, fmt.Errorf("insert guest user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get last insert id: %w", err)
	}

	return s.GetUserByID(ctx, id)
}

// GetUserByID retrieves a user by ID.
func (s *SQLiteStore) GetUserByID(ctx context.Context, id int64) (*store.User, error) {
	query := `
		SELECT id, username, password_hash, is_guest, COALESCE(session_id, ''), created_at
		FROM users
		WHERE id = ?
	`
	var user store.User
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.IsGuest,
		&user.SessionID,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found: %w", err)
		}
		return nil, fmt.Errorf("query user: %w", err)
	}

	return &user, nil
}

// GetUserByUsername retrieves a user by username.
func (s *SQLiteStore) GetUserByUsername(ctx context.Context, username string) (*store.User, error) {
	query := `
		SELECT id, username, password_hash, is_guest, COALESCE(session_id, ''), created_at
		FROM users
		WHERE username = ? AND is_guest = 0
	`
	var user store.User
	err := s.db.QueryRowContext(ctx, query, username).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.IsGuest,
		&user.SessionID,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found: %w", err)
		}
		return nil, fmt.Errorf("query user: %w", err)
	}

	return &user, nil
}

// GetUserBySessionID retrieves a guest user by session ID.
func (s *SQLiteStore) GetUserBySessionID(ctx context.Context, sessionID string) (*store.User, error) {
	query := `
		SELECT id, username, password_hash, is_guest, COALESCE(session_id, ''), created_at
		FROM users
		WHERE session_id = ? AND is_guest = 1
	`
	var user store.User
	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.IsGuest,
		&user.SessionID,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("guest user not found: %w", err)
		}
		return nil, fmt.Errorf("query guest user: %w", err)
	}

	return &user, nil
}

// ==== RoomStore implementation ====

// CreateRoom creates a new room.
func (s *SQLiteStore) CreateRoom(ctx context.Context, name string, roomType store.RoomType, ownerID *int64) (*store.Room, error) {
	query := `
		INSERT INTO rooms (name, type, owner_id)
		VALUES (?, ?, ?)
	`
	result, err := s.db.ExecContext(ctx, query, name, roomType, ownerID)
	if err != nil {
		return nil, fmt.Errorf("insert room: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get last insert id: %w", err)
	}

	return s.GetRoomByID(ctx, id)
}

// GetRoomByID retrieves a room by ID.
func (s *SQLiteStore) GetRoomByID(ctx context.Context, id int64) (*store.Room, error) {
	query := `
		SELECT id, name, type, owner_id, direct_key, created_at
		FROM rooms
		WHERE id = ?
	`
	var room store.Room
	var ownerID sql.NullInt64
	var directKey sql.NullString
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&room.ID,
		&room.Name,
		&room.Type,
		&ownerID,
		&directKey,
		&room.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("room not found: %w", err)
		}
		return nil, fmt.Errorf("query room: %w", err)
	}

	if ownerID.Valid {
		room.OwnerID = &ownerID.Int64
	}
	if directKey.Valid {
		room.DirectKey = &directKey.String
	}

	return &room, nil
}

// GetRoomByName retrieves a room by name.
func (s *SQLiteStore) GetRoomByName(ctx context.Context, name string) (*store.Room, error) {
	query := `
		SELECT id, name, type, owner_id, direct_key, created_at
		FROM rooms
		WHERE name = ?
	`
	var room store.Room
	var ownerID sql.NullInt64
	var directKey sql.NullString
	err := s.db.QueryRowContext(ctx, query, name).Scan(
		&room.ID,
		&room.Name,
		&room.Type,
		&ownerID,
		&directKey,
		&room.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("room not found: %w", err)
		}
		return nil, fmt.Errorf("query room: %w", err)
	}

	if ownerID.Valid {
		room.OwnerID = &ownerID.Int64
	}
	if directKey.Valid {
		room.DirectKey = &directKey.String
	}

	return &room, nil
}

// ListRooms lists all accessible rooms for a user.
func (s *SQLiteStore) ListRooms(ctx context.Context, userID int64) ([]*store.Room, error) {
	query := `
		SELECT DISTINCT r.id, r.name, r.type, r.owner_id, r.direct_key, r.created_at
		FROM rooms r
		LEFT JOIN room_members rm ON r.id = rm.room_id
		WHERE r.type = 'public'
		   OR rm.user_id = ?
		   OR r.owner_id = ?
		ORDER BY r.created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query, userID, userID)
	if err != nil {
		return nil, fmt.Errorf("query rooms: %w", err)
	}
	defer rows.Close()

	var rooms []*store.Room
	for rows.Next() {
		var room store.Room
		var ownerID sql.NullInt64
		var directKey sql.NullString
		if err := rows.Scan(&room.ID, &room.Name, &room.Type, &ownerID, &directKey, &room.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan room: %w", err)
		}
		if ownerID.Valid {
			room.OwnerID = &ownerID.Int64
		}
		if directKey.Valid {
			room.DirectKey = &directKey.String
		}
		rooms = append(rooms, &room)
	}

	return rooms, rows.Err()
}

// GetRoomByDirectKey retrieves a direct room by its direct_key.
func (s *SQLiteStore) GetRoomByDirectKey(ctx context.Context, directKey string) (*store.Room, error) {
	query := `
		SELECT id, name, type, owner_id, direct_key, created_at
		FROM rooms
		WHERE direct_key = ?
	`
	var room store.Room
	var ownerID sql.NullInt64
	var directKeyNullable sql.NullString
	err := s.db.QueryRowContext(ctx, query, directKey).Scan(
		&room.ID,
		&room.Name,
		&room.Type,
		&ownerID,
		&directKeyNullable,
		&room.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("room not found: %w", err)
		}
		return nil, fmt.Errorf("query room: %w", err)
	}

	if ownerID.Valid {
		room.OwnerID = &ownerID.Int64
	}
	if directKeyNullable.Valid {
		room.DirectKey = &directKeyNullable.String
	}

	return &room, nil
}

// CreateDirectRoom creates a direct message room between two users.
// Handles deduplication via directKey and auto-adds both users as members.
func (s *SQLiteStore) CreateDirectRoom(ctx context.Context, directKey string, user1ID, user2ID int64) (*store.Room, error) {
	// Check if room already exists
	room, err := s.GetRoomByDirectKey(ctx, directKey)
	if err == nil {
		// Room already exists, return it
		return room, nil
	}
	// If error is not "not found", return the error
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("check existing room: %w", err)
	}

	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback() //nolint:errcheck // Rollback is called on defer, error is not critical here
	}()

	// Generate room name (dm-{user1ID}-{user2ID})
	roomName := fmt.Sprintf("dm-%d-%d", user1ID, user2ID)

	// Insert room
	query := `
		INSERT INTO rooms (name, type, owner_id, direct_key)
		VALUES (?, 'direct', NULL, ?)
	`
	result, err := tx.ExecContext(ctx, query, roomName, directKey)
	if err != nil {
		return nil, fmt.Errorf("insert room: %w", err)
	}

	roomID, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get last insert id: %w", err)
	}

	// Add both users as members
	memberQuery := `
		INSERT INTO room_members (user_id, room_id)
		VALUES (?, ?)
	`
	if _, err := tx.ExecContext(ctx, memberQuery, user1ID, roomID); err != nil {
		return nil, fmt.Errorf("add user1 to members: %w", err)
	}
	if _, err := tx.ExecContext(ctx, memberQuery, user2ID, roomID); err != nil {
		return nil, fmt.Errorf("add user2 to members: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	// Return the created room
	return s.GetRoomByID(ctx, roomID)
}

// AddMember adds a user to a room.
func (s *SQLiteStore) AddMember(ctx context.Context, userID, roomID int64) error {
	query := `
		INSERT OR IGNORE INTO room_members (user_id, room_id)
		VALUES (?, ?)
	`
	_, err := s.db.ExecContext(ctx, query, userID, roomID)
	if err != nil {
		return fmt.Errorf("insert room member: %w", err)
	}

	return nil
}

// RemoveMember removes a user from a room.
func (s *SQLiteStore) RemoveMember(ctx context.Context, userID, roomID int64) error {
	query := `
		DELETE FROM room_members
		WHERE user_id = ? AND room_id = ?
	`
	_, err := s.db.ExecContext(ctx, query, userID, roomID)
	if err != nil {
		return fmt.Errorf("delete room member: %w", err)
	}

	return nil
}

// IsMember checks if user is a member of the room.
func (s *SQLiteStore) IsMember(ctx context.Context, userID, roomID int64) (bool, error) {
	query := `
		SELECT 1 FROM room_members
		WHERE user_id = ? AND room_id = ?
	`
	var exists int
	err := s.db.QueryRowContext(ctx, query, userID, roomID).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("query membership: %w", err)
	}

	return true, nil
}

// ListMembers lists all members of a room.
func (s *SQLiteStore) ListMembers(ctx context.Context, roomID int64) ([]int64, error) {
	query := `
		SELECT user_id FROM room_members
		WHERE room_id = ?
		ORDER BY joined_at ASC
	`
	rows, err := s.db.QueryContext(ctx, query, roomID)
	if err != nil {
		return nil, fmt.Errorf("query members: %w", err)
	}
	defer rows.Close()

	var members []int64
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, userID)
	}

	return members, rows.Err()
}

// ==== MessageStore implementation ====

// SaveMessage persists a message to storage.
func (s *SQLiteStore) SaveMessage(ctx context.Context, msg *store.Message) error {
	query := `
		INSERT INTO messages (room_id, user_id, body, created_at)
		VALUES (?, ?, ?, ?)
	`
	result, err := s.db.ExecContext(ctx, query, msg.RoomID, msg.UserID, msg.Body, msg.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}

	msg.ID = id
	return nil
}

// ListMessages retrieves messages from a room with pagination.
func (s *SQLiteStore) ListMessages(ctx context.Context, roomID int64, limit int, beforeID *int64) ([]*store.Message, error) {
	var query string
	var args []interface{}

	if beforeID != nil {
		query = `
			SELECT id, room_id, user_id, body, created_at
			FROM messages
			WHERE room_id = ? AND id < ?
			ORDER BY id DESC
			LIMIT ?
		`
		args = []interface{}{roomID, *beforeID, limit}
	} else {
		query = `
			SELECT id, room_id, user_id, body, created_at
			FROM messages
			WHERE room_id = ?
			ORDER BY id DESC
			LIMIT ?
		`
		args = []interface{}{roomID, limit}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var messages []*store.Message
	for rows.Next() {
		var msg store.Message
		if err := rows.Scan(&msg.ID, &msg.RoomID, &msg.UserID, &msg.Body, &msg.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		messages = append(messages, &msg)
	}

	// Reverse to get chronological order
	for i := range len(messages) / 2 {
		messages[i], messages[len(messages)-1-i] = messages[len(messages)-1-i], messages[i]
	}

	return messages, rows.Err()
}
