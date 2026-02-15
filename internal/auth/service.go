package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/vovakirdan/wirechat-server/internal/store"
)

var (
	// ErrInvalidCredentials is returned when username/password don't match.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrUserExists is returned when trying to register with existing username.
	ErrUserExists = errors.New("user already exists")
	// ErrInvalidUsername is returned when username doesn't meet constraints.
	ErrInvalidUsername = errors.New("invalid username")
	// ErrInvalidPassword is returned when password doesn't meet constraints.
	ErrInvalidPassword = errors.New("invalid password")
)

// Service provides authentication operations.
type Service struct {
	store     store.UserStore
	jwtConfig *JWTConfig
}

// NewService creates a new authentication service.
func NewService(userStore store.UserStore, jwtConfig *JWTConfig) *Service {
	return &Service{
		store:     userStore,
		jwtConfig: jwtConfig,
	}
}

// Register creates a new user with hashed password and returns a JWT token.
func (s *Service) Register(ctx context.Context, username, password string) (string, error) {
	username = strings.TrimSpace(username)
	if len(username) < 3 || len(username) > 32 {
		return "", ErrInvalidUsername
	}
	if len(password) < 6 {
		return "", ErrInvalidPassword
	}

	// Check if user already exists
	existing, err := s.store.GetUserByUsername(ctx, username)
	if err == nil && existing != nil {
		return "", ErrUserExists
	}

	// Hash password
	hashedPassword, err := HashPassword(password)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	// Create user
	user, err := s.store.CreateUser(ctx, username, hashedPassword)
	if err != nil {
		return "", fmt.Errorf("create user: %w", err)
	}

	// Generate JWT token
	token, err := GenerateToken(s.jwtConfig, user.ID, user.Username, false)
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	return token, nil
}

// Login validates credentials and returns a JWT token.
func (s *Service) Login(ctx context.Context, username, password string) (string, error) {
	// Get user by username
	user, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		return "", ErrInvalidCredentials
	}

	// Compare password
	if errPwd := ComparePassword(user.PasswordHash, password); errPwd != nil {
		return "", ErrInvalidCredentials
	}

	// Generate JWT token
	token, err := GenerateToken(s.jwtConfig, user.ID, user.Username, false)
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	return token, nil
}

// CreateGuestUser creates a temporary guest user and returns a JWT token.
func (s *Service) CreateGuestUser(ctx context.Context) (token, sessionID string, err error) {
	// Generate random session ID
	sessionID, err = generateSessionID()
	if err != nil {
		return "", "", fmt.Errorf("generate session ID: %w", err)
	}

	// Create guest user
	user, err := s.store.CreateGuestUser(ctx, sessionID)
	if err != nil {
		return "", "", fmt.Errorf("create guest user: %w", err)
	}

	// Generate JWT token
	token, err = GenerateToken(s.jwtConfig, user.ID, user.Username, true)
	if err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}

	return token, sessionID, nil
}

// ValidateToken validates a JWT token and returns the claims.
func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	return ValidateToken(s.jwtConfig, tokenString)
}

// generateSessionID generates a random session ID for guest users.
func generateSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
