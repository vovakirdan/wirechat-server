package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/vovakirdan/wirechat-server/internal/app"
	"github.com/vovakirdan/wirechat-server/internal/config"
)

func main() {
	cfg := config.Default()

	flag.StringVar(&cfg.Addr, "addr", cfg.Addr, "HTTP listen address")
	flag.DurationVar(&cfg.ReadHeaderTimeout, "read-header-timeout", cfg.ReadHeaderTimeout, "HTTP read header timeout")
	flag.DurationVar(&cfg.ShutdownTimeout, "shutdown-timeout", cfg.ShutdownTimeout, "graceful shutdown timeout")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application := app.New(cfg)

	log.Printf("starting wirechat server on %s", cfg.Addr)
	if err := application.Run(ctx); err != nil {
		log.Fatalf("server exited with error: %v", err)
	}
	log.Println("server stopped")
}
