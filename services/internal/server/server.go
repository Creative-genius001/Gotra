package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gin-contrib/cors"
	"github.com/gotra/gotra/internal/ai"
	"github.com/gotra/gotra/internal/analytics"
	"github.com/gotra/gotra/internal/auth"
	"github.com/gotra/gotra/internal/billing"
	"github.com/gotra/gotra/internal/config"
	"github.com/gotra/gotra/internal/projects"
	"github.com/gotra/gotra/internal/requests"
	"github.com/gotra/gotra/internal/tunnels"
	"github.com/gotra/gotra/pkg/cache"
	"github.com/gotra/gotra/pkg/database"
	"github.com/gotra/gotra/pkg/middleware"
	"github.com/gotra/gotra/pkg/security"
)

type Server struct {
	cfg       *config.Config
	log       *slog.Logger
	db        *database.DB
	cache     *cache.Cache
	analytics *analytics.Store
	engine    *gin.Engine
}

func New(cfg *config.Config, log *slog.Logger, db *database.DB, c *cache.Cache) *Server {
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Request-ID"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	engine.Use(
		middleware.RequestID(),
		middleware.RequestLogger(log),
		middleware.ErrorHandler(log),
		middleware.Recovery(log),
	)

	tokens := security.NewTokenManager(cfg.JWTSecret, cfg.AccessTokenTTL)

	// Health & readiness.
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "gotra-api"})
	})

	// Analytics (optional — disabled gracefully when ClickHouse is absent).
	analyticsStore := analytics.Open(context.Background(), cfg.ClickHouseURL, log)

	// API v1.
	v1 := engine.Group("/api/v1")

	billingService := billing.NewService(cfg, db)

	authHandler := auth.NewHandler(cfg, db, c, tokens, log)
	authHandler.RegisterRoutes(v1)
	billing.NewHandler(billingService).RegisterPublic(v1)

	authed := v1.Group("")
	authed.Use(middleware.Auth(tokens))
	{
		authHandler.RegisterProtected(authed)
		projects.NewHandler(db).RegisterRoutes(authed)
		tunnels.NewHandler(db, billingService).RegisterRoutes(authed)
		requests.NewHandler(cfg, db).RegisterRoutes(authed)
		ai.NewHandler(cfg, db, log).RegisterRoutes(authed)
		analytics.NewHandler(analyticsStore, db).RegisterRoutes(authed)
		billing.NewHandler(billingService).RegisterRoutes(authed)
	}

	return &Server{cfg: cfg, log: log, db: db, cache: c, analytics: analyticsStore, engine: engine}
}

func (s *Server) Run(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%s", s.cfg.APIHost, s.cfg.APIPort)
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.engine,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		s.log.Info("api listening", "addr", addr, "env", s.cfg.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		s.log.Info("shutting down api")
		s.analytics.Close()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
