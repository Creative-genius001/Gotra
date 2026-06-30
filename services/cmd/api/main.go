package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/gotra/gotra/internal/config"
	"github.com/gotra/gotra/internal/migrate"
	"github.com/gotra/gotra/internal/server"
	"github.com/gotra/gotra/pkg/cache"
	"github.com/gotra/gotra/pkg/database"
	"github.com/gotra/gotra/pkg/logger"
)

func main() {
	cfg := config.Load()
	log := logger.New(cfg.Env)

	// Subcommand: migrations.
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		runMigrate(cfg, os.Args[2:])
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	redis, err := cache.Connect(ctx, cfg.RedisURL)
	if err != nil {
		log.Error("connect redis", "error", err)
		os.Exit(1)
	}
	defer redis.Close()

	srv := server.New(cfg, log, db, redis)
	if err := srv.Run(ctx); err != nil {
		log.Error("server error", "error", err)
		os.Exit(1)
	}
}

func runMigrate(cfg *config.Config, args []string) {
	log := logger.New(cfg.Env)
	ctx := context.Background()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	direction := "up"
	if len(args) > 0 {
		direction = args[0]
	}

	switch direction {
	case "up":
		if err := migrate.Up(ctx, db.Pool); err != nil {
			log.Error("migrate up", "error", err)
			os.Exit(1)
		}
		log.Info("migrations up to date")
	default:
		log.Error("unsupported migrate direction (only 'up' is implemented)", "direction", direction)
		os.Exit(1)
	}
}
