//go:build integration

// Package integration holds end-to-end tests that exercise the service layer
// against a real PostgreSQL database. They are gated behind the `integration`
// build tag and skipped unless TEST_DATABASE_URL is set.
//
//	createdb gotra_test
//	TEST_DATABASE_URL=postgres://gotra:gotra@localhost:5432/gotra_test?sslmode=disable \
//	  go test -tags=integration ./internal/integration/...
package integration

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gotra/gotra/internal/auth"
	"github.com/gotra/gotra/internal/billing"
	"github.com/gotra/gotra/internal/config"
	"github.com/gotra/gotra/internal/migrate"
	"github.com/gotra/gotra/internal/projects"
	"github.com/gotra/gotra/internal/tunnels"
	"github.com/gotra/gotra/pkg/database"
	"github.com/gotra/gotra/pkg/security"
)

// captureMailer records the tokens that would have been emailed.
type captureMailer struct {
	mu       sync.Mutex
	verify   string
	reset    string
}

func (m *captureMailer) SendVerificationEmail(_ context.Context, _, token string) error {
	m.mu.Lock()
	m.verify = token
	m.mu.Unlock()
	return nil
}

func (m *captureMailer) SendPasswordResetEmail(_ context.Context, _, token string) error {
	m.mu.Lock()
	m.reset = token
	m.mu.Unlock()
	return nil
}

type env struct {
	ctx      context.Context
	cfg      *config.Config
	db       *database.DB
	mailer   *captureMailer
	authSvc  *auth.Service
	projSvc  *projects.Service
	billSvc  *billing.Service
	tunSvc   *tunnels.Service
}

func setup(t *testing.T) *env {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration tests")
	}

	ctx := context.Background()
	db, err := database.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(db.Close)

	if err := migrate.Up(ctx, db.Pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cfg := &config.Config{
		AppBaseURL:      "http://localhost:3000",
		JWTSecret:       "integration-secret",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 720 * time.Hour,
	}
	tm := security.NewTokenManager(cfg.JWTSecret, cfg.AccessTokenTTL)
	mailer := &captureMailer{}

	billSvc := billing.NewService(cfg, db)
	return &env{
		ctx:     ctx,
		cfg:     cfg,
		db:      db,
		mailer:  mailer,
		authSvc: auth.NewService(cfg, auth.NewRepository(db.Pool), tm, mailer),
		projSvc: projects.NewService(projects.NewRepository(db.Pool)),
		billSvc: billSvc,
		tunSvc:  tunnels.NewService(tunnels.NewRepository(db.Pool), billSvc),
	}
}

func uniqueEmail() string {
	return fmt.Sprintf("itest-%s@example.com", uuid.NewString()[:8])
}

func TestAuthLifecycle(t *testing.T) {
	e := setup(t)
	email := uniqueEmail()
	client := auth.ClientInfo{IP: "127.0.0.1", UserAgent: "integration"}

	// Register → provisions workspace and issues tokens.
	res, err := e.authSvc.Register(e.ctx, email, "Integration User", "supersecret1", client)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Fatal("expected tokens from register")
	}
	if res.Role != string(security.RoleOwner) {
		t.Errorf("role = %q, want owner", res.Role)
	}

	// Duplicate registration is rejected.
	if _, err := e.authSvc.Register(e.ctx, email, "Dup", "supersecret1", client); err == nil {
		t.Fatal("expected duplicate registration to fail")
	}

	// Login: wrong then right.
	if _, err := e.authSvc.Login(e.ctx, email, "wrongpass", client); err == nil {
		t.Fatal("expected wrong-password login to fail")
	}
	if _, err := e.authSvc.Login(e.ctx, email, "supersecret1", client); err != nil {
		t.Fatalf("login: %v", err)
	}

	// Refresh rotates the token.
	rot, err := e.authSvc.Refresh(e.ctx, res.RefreshToken, client)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if rot.RefreshToken == res.RefreshToken {
		t.Error("refresh token was not rotated")
	}
	if _, err := e.authSvc.Refresh(e.ctx, res.RefreshToken, client); err == nil {
		t.Fatal("old refresh token should be revoked after rotation")
	}

	// Email verification (token captured by the mailer during register).
	if e.mailer.verify == "" {
		t.Fatal("no verification token captured")
	}
	if err := e.authSvc.VerifyEmail(e.ctx, e.mailer.verify); err != nil {
		t.Fatalf("verify email: %v", err)
	}
	if err := e.authSvc.VerifyEmail(e.ctx, e.mailer.verify); err == nil {
		t.Fatal("verification token should be single-use")
	}

	// Password reset.
	token, err := e.authSvc.RequestPasswordReset(e.ctx, email)
	if err != nil {
		t.Fatalf("request reset: %v", err)
	}
	if err := e.authSvc.ResetPassword(e.ctx, token, "brandnewpass2"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if _, err := e.authSvc.Login(e.ctx, email, "supersecret1", client); err == nil {
		t.Fatal("old password should no longer work")
	}
	if _, err := e.authSvc.Login(e.ctx, email, "brandnewpass2", client); err != nil {
		t.Fatalf("login with new password: %v", err)
	}
}

func TestProjectsAndTunnelQuota(t *testing.T) {
	e := setup(t)
	res, err := e.authSvc.Register(e.ctx, uniqueEmail(), "Proj User", "supersecret1", auth.ClientInfo{})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	userID := res.User.ID

	// Provisioning created one default project; create another.
	p, err := e.projSvc.Create(e.ctx, userID, "My API")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	list, err := e.projSvc.List(e.ctx, userID)
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(list) < 2 {
		t.Errorf("expected >=2 projects, got %d", len(list))
	}

	// Free plan allows 2 active tunnels; the 3rd is rejected by the quota.
	if _, err := e.tunSvc.Create(e.ctx, userID, p.ID, 3001); err != nil {
		t.Fatalf("tunnel 1: %v", err)
	}
	if _, err := e.tunSvc.Create(e.ctx, userID, p.ID, 3002); err != nil {
		t.Fatalf("tunnel 2: %v", err)
	}
	_, err = e.tunSvc.Create(e.ctx, userID, p.ID, 3003)
	if err == nil || err.Error() != billing.ErrQuotaExceeded.Error() {
		t.Fatalf("tunnel 3 = %v, want quota exceeded", err)
	}

	// Upgrade to pro lifts the limit; the 3rd tunnel now succeeds.
	if _, err := e.billSvc.ChangePlan(e.ctx, userID, "pro"); err != nil {
		t.Fatalf("change plan: %v", err)
	}
	if _, err := e.tunSvc.Create(e.ctx, userID, p.ID, 3003); err != nil {
		t.Fatalf("tunnel 3 after upgrade: %v", err)
	}
}

func TestBillingInfo(t *testing.T) {
	e := setup(t)
	res, err := e.authSvc.Register(e.ctx, uniqueEmail(), "Bill User", "supersecret1", auth.ClientInfo{})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	userID := res.User.ID

	info, err := e.billSvc.Current(e.ctx, userID)
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	if info.Plan != "free" {
		t.Errorf("default plan = %q, want free", info.Plan)
	}
	if info.Limits.MaxTunnels != 2 {
		t.Errorf("free tunnels limit = %d, want 2", info.Limits.MaxTunnels)
	}
	if len(info.AvailablePlans) != 3 {
		t.Errorf("expected 3 available plans, got %d", len(info.AvailablePlans))
	}

	// Invalid plan rejected.
	if _, err := e.billSvc.ChangePlan(e.ctx, userID, "enterprise-xl"); err == nil {
		t.Fatal("expected invalid plan to fail")
	}
}
