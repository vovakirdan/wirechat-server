package config

import "time"

// Config holds server configuration values.
type Config struct {
	Addr              string        `mapstructure:"addr" yaml:"addr"`
	ReadHeaderTimeout time.Duration `mapstructure:"read_header_timeout" yaml:"read_header_timeout"`
	ShutdownTimeout   time.Duration `mapstructure:"shutdown_timeout" yaml:"shutdown_timeout"`
}

// Default returns configuration with reasonable starter defaults.
func Default() Config {
	return Config{
		Addr:              ":8080",
		ReadHeaderTimeout: 5 * time.Second,
		ShutdownTimeout:   5 * time.Second,
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
}
