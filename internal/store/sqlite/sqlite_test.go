package sqlite

import (
	"context"
	"database/sql"
	"testing"
)

func TestSearchUsers(t *testing.T) {
	// Setup in-memory DB
	s, err := NewWithSetup(":memory:", func(db *sql.DB) error {
		query := `
		CREATE TABLE users (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			username      TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			is_guest      BOOLEAN NOT NULL DEFAULT 0,
			session_id    TEXT,
			allow_calls_from TEXT DEFAULT 'everyone',
			created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		`
		_, err := db.Exec(query)
		return err
	})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	// Seed users
	users := []string{"alice", "alex", "alan", "bob", "charlie"}
	for _, u := range users {
		t.Logf("Creating user %s", u)
		_, err := s.CreateUser(ctx, u, "hash")
		if err != nil {
			t.Fatalf("failed to create user %s: %v", u, err)
		}
	}

	// Create a guest user (should be excluded)
	_, err = s.CreateGuestUser(ctx, "session1")
	if err != nil {
		t.Fatalf("failed to create guest user: %v", err)
	}

	tests := []struct {
		name     string
		query    string
		expected []string
	}{
		{
			name:     "search 'al'",
			query:    "al",
			expected: []string{"alan", "alex", "alice"},
		},
		{
			name:     "search 'li'",
			query:    "li",
			expected: []string{"alice", "charlie"},
		},
		{
			name:     "search non-existent",
			query:    "z",
			expected: []string{},
		},
		{
			name:     "search case sensitive (sqlite default is often case insensitive for ASCII, but let's check basic match)",
			query:    "Bob",
			expected: []string{"bob"}, // Assuming default collation is case-insensitive for LIKE
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := s.SearchUsers(ctx, tt.query)
			if err != nil {
				t.Fatalf("SearchUsers failed: %v", err)
			}

			// Extract usernames
			var names []string
			for _, u := range results {
				names = append(names, u.Username)
			}

			// Check simplified results (ignoring case for this broad check if needed, but strict check is better)
			// SQLite LIKE is case-insensitive for ASCII characters by default.
			if len(results) != len(tt.expected) {
				t.Errorf("expected %d results, got %d: %v", len(tt.expected), len(results), names)
				return
			}

			for i, name := range names {
				// simple lower case check to avoid test flakiness on system collation
				// but strictly we expect exact matches if we inserted lowercase.
				if name != tt.expected[i] {
					t.Errorf("expected %s at index %d, got %s", tt.expected[i], i, name)
				}
			}
		})
	}
}
