// Command gateway is the Gotra tunnel gateway: it accepts agent WebSocket
// connections, registers tunnels, and forwards public HTTP traffic to the
// matching agent (Backend Bible — Gateway Architecture).
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/gotra/gotra/internal/config"
	"github.com/gotra/gotra/internal/gateway"
	"github.com/gotra/gotra/pkg/cache"
	"github.com/gotra/gotra/pkg/database"
	"github.com/gotra/gotra/pkg/logger"
)

func main() {
	cfg := config.Load()
	log := logger.New(cfg.Env)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Redis enables multi-instance coordination; absence degrades to a single
	// instance rather than failing.
	var redisCache *cache.Cache
	if c, err := cache.Connect(ctx, cfg.RedisURL); err != nil {
		log.Warn("gateway running single-instance (redis unavailable)", "error", err)
	} else {
		redisCache = c
		defer redisCache.Close()
	}

	srv := gateway.New(cfg, log, db, redisCache)
	if err := srv.Run(ctx); err != nil {
		log.Error("gateway error", "error", err)
		os.Exit(1)
	}
}
