package auth

import (
	"context"
	"log/slog"

	"github.com/gotra/gotra/internal/config"
)

// Mailer sends transactional auth emails. The production implementation will
// integrate an email provider; for now LogMailer records the links so the flows
// are fully testable without SMTP.
type Mailer interface {
	SendVerificationEmail(ctx context.Context, to, token string) error
	SendPasswordResetEmail(ctx context.Context, to, token string) error
}

// LogMailer logs the verification / reset links instead of sending email.
type LogMailer struct {
	cfg *config.Config
	log *slog.Logger
}

// NewLogMailer constructs a LogMailer.
func NewLogMailer(cfg *config.Config, log *slog.Logger) *LogMailer {
	return &LogMailer{cfg: cfg, log: log}
}

// SendVerificationEmail logs the email-verification link.
func (m *LogMailer) SendVerificationEmail(_ context.Context, to, token string) error {
	link := m.cfg.AppBaseURL + "/verify-email?token=" + token
	m.log.Info("email verification link", "to", to, "link", link)
	return nil
}

// SendPasswordResetEmail logs the password-reset link.
func (m *LogMailer) SendPasswordResetEmail(_ context.Context, to, token string) error {
	link := m.cfg.AppBaseURL + "/reset-password?token=" + token
	m.log.Info("password reset link", "to", to, "link", link)
	return nil
}
