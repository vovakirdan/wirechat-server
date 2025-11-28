package config

import "time"

// Config holds server configuration values.
type Config struct {
	Addr              string        `mapstructure:"addr" yaml:"addr"`
	ReadHeaderTimeout time.Duration `mapstructure:"read_header_timeout" yaml:"read_header_timeout"`
	ShutdownTimeout   time.Duration `mapstructure:"shutdown_timeout" yaml:"shutdown_timeout"`
	MaxMessageBytes   int64         `mapstructure:"max_message_bytes" yaml:"max_message_bytes"`
}

// Default returns configuration with reasonable starter defaults.
func Default() Config {
	return Config{
		Addr:              ":8080",
		ReadHeaderTimeout: 5 * time.Second,
		ShutdownTimeout:   5 * time.Second,
		MaxMessageBytes:   1 << 20, // 1MB
	}
}

// UpdateFrom overwrites non-zero values from other config into receiver.
func (c *Config) UpdateFrom(other Config) {
	if other.Addr != "" {
		c.Addr = other.Addr
	}
	if other.ReadHeaderTimeout != 0 {
		c.ReadHeaderTimeout = other.ReadHeaderTimeout
	}
	if other.ShutdownTimeout != 0 {
		c.ShutdownTimeout = other.ShutdownTimeout
	}
	if other.MaxMessageBytes != 0 {
		c.MaxMessageBytes = other.MaxMessageBytes
	}
}
