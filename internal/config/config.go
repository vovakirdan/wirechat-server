package config

import "time"

// Config holds server configuration values.
type Config struct {
	Addr              string
	ReadHeaderTimeout time.Duration
	ShutdownTimeout   time.Duration
}

// Default returns configuration with reasonable starter defaults.
func Default() Config {
	return Config{
		Addr:              ":8080",
		ReadHeaderTimeout: 5 * time.Second,
		ShutdownTimeout:   5 * time.Second,
	}
}
