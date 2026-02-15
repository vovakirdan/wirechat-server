package auth

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/vovakirdan/wirechat-server/internal/store/sqlite"
)

func newTestAuthService(t *testing.T) *Service {
	t.Helper()

	st, err := sqlite.NewWithSetup(":memory:", func(db *sql.DB) error {
		schema := `
		CREATE TABLE users (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			username      TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			is_guest      BOOLEAN NOT NULL DEFAULT 0,
			session_id    TEXT,
			created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		`
		_, err := db.Exec(schema)
		return err
	})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	jwtConfig := &JWTConfig{
		Secret:   []byte("test-secret-change-me"),
		Issuer:   "test",
		Audience: "test",
		TTL:      24 * time.Hour,
	}

	return NewService(st, jwtConfig)
}

func TestRegister_RejectsInvalidUsername(t *testing.T) {
	svc := newTestAuthService(t)
	ctx := context.Background()

	if _, err := svc.Register(ctx, "ab", "password123"); !errors.Is(err, ErrInvalidUsername) {
		t.Fatalf("expected ErrInvalidUsername, got %v", err)
	}

	// Should be validated after trimming whitespace.
	if _, err := svc.Register(ctx, " ab ", "password123"); !errors.Is(err, ErrInvalidUsername) {
		t.Fatalf("expected ErrInvalidUsername, got %v", err)
	}
}

func TestRegister_RejectsInvalidPassword(t *testing.T) {
	svc := newTestAuthService(t)
	ctx := context.Background()

	if _, err := svc.Register(ctx, "abc", "12345"); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("expected ErrInvalidPassword, got %v", err)
	}
}

func TestRegister_TrimsUsernameAndCreatesUser(t *testing.T) {
	svc := newTestAuthService(t)
	ctx := context.Background()

	token, err := svc.Register(ctx, " alice ", "password123")
	if err != nil {
		t.Fatalf("expected registration success, got %v", err)
	}
	if token == "" {
		t.Fatalf("expected non-empty token")
	}

	// Should collide because the stored username is trimmed.
	if _, err := svc.Register(ctx, "alice", "password123"); !errors.Is(err, ErrUserExists) {
		t.Fatalf("expected ErrUserExists, got %v", err)
	}
}

