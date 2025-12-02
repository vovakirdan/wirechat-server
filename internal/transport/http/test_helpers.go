package http

import (
	"database/sql"
	"testing"
	"time"

	"github.com/vovakirdan/wirechat-server/internal/auth"
	"github.com/vovakirdan/wirechat-server/internal/store"
	"github.com/vovakirdan/wirechat-server/internal/store/sqlite"
)

// createTestStore creates an in-memory SQLite store with schema applied.
func createTestStore(t *testing.T) store.Store {
	t.Helper()

	// Apply schema manually (instead of using goose migrations in tests)
	schema := `
	CREATE TABLE users (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		username      TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		is_guest      BOOLEAN NOT NULL DEFAULT 0,
		session_id    TEXT,
		created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE rooms (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		name       TEXT NOT NULL UNIQUE,
		type       TEXT NOT NULL DEFAULT 'public',
		owner_id   INTEGER,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (owner_id) REFERENCES users(id)
	);

	CREATE TABLE room_members (
		room_id    INTEGER NOT NULL,
		user_id    INTEGER NOT NULL,
		joined_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (room_id, user_id),
		FOREIGN KEY (room_id) REFERENCES rooms(id),
		FOREIGN KEY (user_id) REFERENCES users(id)
	);

	CREATE TABLE messages (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		room_id    INTEGER NOT NULL,
		user_id    INTEGER NOT NULL,
		text       TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (room_id) REFERENCES rooms(id),
		FOREIGN KEY (user_id) REFERENCES users(id)
	);

	CREATE INDEX idx_messages_room ON messages(room_id, created_at DESC);
	CREATE INDEX idx_room_members_user ON room_members(user_id);

	INSERT INTO rooms (name, type, owner_id) VALUES ('general', 'public', NULL);
	`

	st, err := sqlite.NewWithSetup(":memory:", func(db *sql.DB) error {
		_, err := db.Exec(schema)
		return err
	})
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}

	return st
}

// createTestAuthService creates an auth service for testing.
func createTestAuthService(t *testing.T, st store.Store, jwtSecret string) *auth.Service {
	t.Helper()

	jwtConfig := &auth.JWTConfig{
		Secret:   []byte(jwtSecret),
		Issuer:   "test",
		Audience: "test",
		TTL:      24 * time.Hour,
	}

	return auth.NewService(st, jwtConfig)
}
