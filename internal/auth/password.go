package auth

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const (
	// bcryptCost is the default cost for bcrypt hashing.
	// Cost of 10 provides a good balance between security and performance.
	bcryptCost = 10
)

// HashPassword generates a bcrypt hash of the password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// ComparePassword compares a bcrypt hashed password with its plaintext version.
func ComparePassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}
