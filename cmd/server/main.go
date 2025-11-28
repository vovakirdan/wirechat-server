package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/vovakirdan/wirechat-server/internal/app"
	"github.com/vovakirdan/wirechat-server/internal/config"
	intlog "github.com/vovakirdan/wirechat-server/internal/log"
)

type serverFlags struct {
	addr              string
	readHeaderTimeout time.Duration
	shutdownTimeout   time.Duration
	logLevel          string
	configPath        string
}

func main() {
	cfg := config.Default()
	flags := serverFlags{
		addr:              cfg.Addr,
		readHeaderTimeout: cfg.ReadHeaderTimeout,
		shutdownTimeout:   cfg.ShutdownTimeout,
		logLevel:          "info",
	}

	rootCmd := &cobra.Command{
		Use:   "wirechat-server",
		Short: "WireChat server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, flags, cfg)
		},
	}

	rootCmd.Flags().StringVar(&flags.addr, "addr", flags.addr, "HTTP listen address")
	rootCmd.Flags().DurationVar(&flags.readHeaderTimeout, "read-header-timeout", flags.readHeaderTimeout, "HTTP read header timeout")
	rootCmd.Flags().DurationVar(&flags.shutdownTimeout, "shutdown-timeout", flags.shutdownTimeout, "graceful shutdown timeout")
	rootCmd.Flags().StringVar(&flags.logLevel, "log-level", flags.logLevel, "log level: debug|info|warn|error")
	rootCmd.Flags().StringVar(&flags.configPath, "config", "", "path to config file (optional)")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	rootCmd.SetContext(ctx)

	if err := rootCmd.Execute(); err != nil {
		stop()
		os.Exit(1)
	}
	stop()
}

func run(cmd *cobra.Command, flags serverFlags, cfg config.Config) error {
	ctx := cmd.Context()

	logger := intlog.New(flags.logLevel)
	zerolog.DefaultContextLogger = logger

	loadedCfg, cfgPath, err := config.Load(logger, flags.configPath)
	cfg = loadedCfg
	if err != nil {
		logger.Warn().Err(err).Msg("failed to load config, using defaults where possible")
	}

	// CLI flags override config if explicitly set.
	if cmd.Flags().Changed("addr") {
		cfg.Addr = flags.addr
	}
	if cmd.Flags().Changed("read-header-timeout") {
		cfg.ReadHeaderTimeout = flags.readHeaderTimeout
	}
	if cmd.Flags().Changed("shutdown-timeout") {
		cfg.ShutdownTimeout = flags.shutdownTimeout
	}

	application := app.New(cfg, logger)

	logger.Info().Str("addr", cfg.Addr).Str("config", cfgPath).Msg("starting wirechat server")
	if err := application.Run(ctx); err != nil {
		logger.Error().Err(err).Msg("server exited with error")
		return err
	}
	logger.Info().Msg("server stopped")
	return nil
}
