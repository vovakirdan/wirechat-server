package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const (
	envConfigDefaultPath = "WIRECHAT_CONFIG_DEFAULT_PATH"
	defaultConfigName    = "config.yaml"
)

// Load builds configuration from defaults, optional config file, env vars, and returns the resolved path.
// Precedence: defaults < config file < env vars < caller overrides.
func Load(logger *zerolog.Logger, explicitPath string) (Config, string, error) {
	cfg := Default()

	v := viper.New()
	v.SetConfigType("yaml")
	v.SetDefault("addr", cfg.Addr)
	v.SetDefault("read_header_timeout", cfg.ReadHeaderTimeout)
	v.SetDefault("shutdown_timeout", cfg.ShutdownTimeout)

	v.SetEnvPrefix("WIRECHAT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	configPath := resolveConfigPath(explicitPath)
	v.SetConfigFile(configPath)

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) || errors.Is(err, os.ErrNotExist) {
			if writeErr := writeDefaultConfig(configPath, cfg); writeErr != nil && logger != nil {
				logger.Warn().Err(writeErr).Str("path", configPath).Msg("failed to write default config")
			} else if logger != nil {
				logger.Info().Str("path", configPath).Msg("created default config")
			}
			// try reading again in case it was just written
			if readErr := v.ReadInConfig(); readErr != nil && logger != nil {
				logger.Warn().Err(readErr).Str("path", configPath).Msg("failed to read config after writing default")
			}
		} else {
			return cfg, configPath, fmt.Errorf("read config: %w", err)
		}
	}

	if err := v.Unmarshal(&cfg); err != nil {
		return cfg, configPath, fmt.Errorf("unmarshal config: %w", err)
	}

	return cfg, configPath, nil
}

func resolveConfigPath(explicitPath string) string {
	if explicitPath != "" {
		return explicitPath
	}

	if base := os.Getenv(envConfigDefaultPath); base != "" {
		if err := os.MkdirAll(base, 0o755); err == nil {
			return filepath.Join(base, defaultConfigName)
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return defaultConfigName
	}
	return filepath.Join(cwd, defaultConfigName)
}

func writeDefaultConfig(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
