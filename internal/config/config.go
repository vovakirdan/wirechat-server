package config

import "time"

// Config holds server configuration values.
type Config struct {
	Addr                string        `mapstructure:"addr" yaml:"addr"`
	DatabasePath        string        `mapstructure:"database_path" yaml:"database_path"`
	ReadHeaderTimeout   time.Duration `mapstructure:"read_header_timeout" yaml:"read_header_timeout"`
	ShutdownTimeout     time.Duration `mapstructure:"shutdown_timeout" yaml:"shutdown_timeout"`
	MaxMessageBytes     int64         `mapstructure:"max_message_bytes" yaml:"max_message_bytes"`
	RateLimitJoinPerMin int           `mapstructure:"rate_limit_join_per_min" yaml:"rate_limit_join_per_min"`
	RateLimitMsgPerMin  int           `mapstructure:"rate_limit_msg_per_min" yaml:"rate_limit_msg_per_min"`
	PingInterval        time.Duration `mapstructure:"ping_interval" yaml:"ping_interval"`
	ClientIdleTimeout   time.Duration `mapstructure:"client_idle_timeout" yaml:"client_idle_timeout"`
	JWTSecret           string        `mapstructure:"jwt_secret" yaml:"jwt_secret"`
	JWTAudience         string        `mapstructure:"jwt_audience" yaml:"jwt_audience"`
	JWTIssuer           string        `mapstructure:"jwt_issuer" yaml:"jwt_issuer"`
	JWTRequired         bool          `mapstructure:"jwt_required" yaml:"jwt_required"`
}

// Default returns configuration with reasonable starter defaults.
func Default() Config {
	return Config{
		Addr:                ":8080",
		DatabasePath:        "data/wirechat.db",
		ReadHeaderTimeout:   5 * time.Second,
		ShutdownTimeout:     5 * time.Second,
		MaxMessageBytes:     1 << 20, // 1MB
		RateLimitJoinPerMin: 60,
		RateLimitMsgPerMin:  300,
		PingInterval:        30 * time.Second,
		ClientIdleTimeout:   90 * time.Second,                  // 3x ping interval - buffer for ping/pong cycles
		JWTSecret:           "dev-secret-change-in-production", // IMPORTANT: Change in production!
		JWTAudience:         "wirechat",
		JWTIssuer:           "wirechat-server",
		JWTRequired:         false,
	}
}

// UpdateFrom overwrites non-zero values from other config into receiver.
func (c *Config) UpdateFrom(other *Config) {
	if other == nil {
		return
	}
	if other.Addr != "" {
		c.Addr = other.Addr
	}
	if other.DatabasePath != "" {
		c.DatabasePath = other.DatabasePath
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
	if other.RateLimitJoinPerMin != 0 {
		c.RateLimitJoinPerMin = other.RateLimitJoinPerMin
	}
	if other.RateLimitMsgPerMin != 0 {
		c.RateLimitMsgPerMin = other.RateLimitMsgPerMin
	}
	if other.PingInterval != 0 {
		c.PingInterval = other.PingInterval
	}
	if other.ClientIdleTimeout != 0 {
		c.ClientIdleTimeout = other.ClientIdleTimeout
	}
	if other.JWTSecret != "" {
		c.JWTSecret = other.JWTSecret
	}
	if other.JWTAudience != "" {
		c.JWTAudience = other.JWTAudience
	}
	if other.JWTIssuer != "" {
		c.JWTIssuer = other.JWTIssuer
	}
	if other.JWTRequired {
		c.JWTRequired = other.JWTRequired
	}
}
