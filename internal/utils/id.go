package utils

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"time"
)

// NewID returns a best-effort unique identifier.
func NewID() string {
	const size = 12

	buf := make([]byte, size)
	if _, err := rand.Read(buf); err == nil {
		return hex.EncodeToString(buf)
	}

	// Fallback to timestamp if crypto/rand is unavailable.
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}
