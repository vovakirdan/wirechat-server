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
			return run(cmd.Context(), flags, cfg)
		},
	}

	rootCmd.Flags().StringVar(&flags.addr, "addr", flags.addr, "HTTP listen address")
	rootCmd.Flags().DurationVar(&flags.readHeaderTimeout, "read-header-timeout", flags.readHeaderTimeout, "HTTP read header timeout")
	rootCmd.Flags().DurationVar(&flags.shutdownTimeout, "shutdown-timeout", flags.shutdownTimeout, "graceful shutdown timeout")
	rootCmd.Flags().StringVar(&flags.logLevel, "log-level", flags.logLevel, "log level: debug|info|warn|error")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	rootCmd.SetContext(ctx)

	if err := rootCmd.Execute(); err != nil {
		stop()
		os.Exit(1)
	}
	stop()
}

func run(ctx context.Context, flags serverFlags, cfg config.Config) error {
	cfg.Addr = flags.addr
	cfg.ReadHeaderTimeout = flags.readHeaderTimeout
	cfg.ShutdownTimeout = flags.shutdownTimeout

	logger := intlog.New(flags.logLevel)
	zerolog.DefaultContextLogger = logger

	application := app.New(cfg, logger)

	logger.Info().Str("addr", cfg.Addr).Msg("starting wirechat server")
	if err := application.Run(ctx); err != nil {
		logger.Error().Err(err).Msg("server exited with error")
		return err
	}
	logger.Info().Msg("server stopped")
	return nil
}
