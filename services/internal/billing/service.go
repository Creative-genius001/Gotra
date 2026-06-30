package billing

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/gotra/gotra/internal/config"
	"github.com/gotra/gotra/pkg/database"
)

// Billing errors.
var (
	ErrInvalidPlan   = errors.New("billing: unknown plan")
	ErrQuotaExceeded = errors.New("billing: plan limit reached")
	ErrNoWebhook     = errors.New("billing: processor has no webhook")
)

// Processor abstracts payment processing so a real provider (Stripe) can be
// plugged in.
type Processor interface {
	// StartCheckout returns a redirect URL for paid plans (the plan is applied
	// later via webhook), or "" to apply the change immediately.
	StartCheckout(ctx context.Context, workspaceID uuid.UUID, plan string) (string, error)
}

// WebhookProcessor is implemented by processors that confirm plan changes via a
// webhook (e.g. Stripe). It returns the workspace and plan to apply.
type WebhookProcessor interface {
	ParseWebhook(payload []byte, signature string) (uuid.UUID, string, error)
}

// StubProcessor changes plans immediately without charging (development default).
type StubProcessor struct{}

// StartCheckout always applies immediately.
func (StubProcessor) StartCheckout(context.Context, uuid.UUID, string) (string, error) {
	return "", nil
}

// Service implements billing use cases.
type Service struct {
	repo      *Repository
	processor Processor
}

// NewService constructs a billing Service, selecting the Stripe processor when
// configured and falling back to the stub otherwise.
func NewService(cfg *config.Config, db *database.DB) *Service {
	var p Processor = StubProcessor{}
	if cfg.Stripe.Enabled() {
		p = NewStripeProcessor(cfg)
	}
	return &Service{repo: NewRepository(db.Pool), processor: p}
}

// Info is the billing snapshot for a workspace.
type Info struct {
	Plan           string `json:"plan"`
	Status         string `json:"status"`
	Limits         Plan   `json:"limits"`
	Usage          Usage  `json:"usage"`
	AvailablePlans []Plan `json:"available_plans"`
}

// Current returns the caller's plan, limits and usage.
func (s *Service) Current(ctx context.Context, userID uuid.UUID) (*Info, error) {
	sub, err := s.repo.PrimarySubscription(ctx, userID)
	if err != nil {
		return nil, err
	}
	usage, err := s.repo.WorkspaceUsage(ctx, sub.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return &Info{
		Plan:           sub.Plan,
		Status:         sub.Status,
		Limits:         PlanFor(sub.Plan),
		Usage:          usage,
		AvailablePlans: catalog(),
	}, nil
}

// ChangeResult is the outcome of a plan change: either an updated snapshot
// (applied immediately) or a checkout URL the client must visit to pay.
type ChangeResult struct {
	Info        *Info  `json:"info,omitempty"`
	CheckoutURL string `json:"checkout_url,omitempty"`
}

// ChangePlan switches the workspace to a new plan. Paid plans on a real
// processor return a checkout URL; the plan is applied on webhook confirmation.
func (s *Service) ChangePlan(ctx context.Context, userID uuid.UUID, plan string) (*ChangeResult, error) {
	if _, ok := Plans[plan]; !ok {
		return nil, ErrInvalidPlan
	}
	sub, err := s.repo.PrimarySubscription(ctx, userID)
	if err != nil {
		return nil, err
	}

	url, err := s.processor.StartCheckout(ctx, sub.WorkspaceID, plan)
	if err != nil {
		return nil, err
	}
	if url != "" {
		return &ChangeResult{CheckoutURL: url}, nil
	}

	if err := s.repo.SetPlan(ctx, sub.WorkspaceID, plan); err != nil {
		return nil, err
	}
	info, err := s.Current(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &ChangeResult{Info: info}, nil
}

// ApplyWebhook validates a processor webhook and applies the resulting plan.
func (s *Service) ApplyWebhook(payload []byte, signature string) error {
	wp, ok := s.processor.(WebhookProcessor)
	if !ok {
		return ErrNoWebhook
	}
	workspaceID, plan, err := wp.ParseWebhook(payload, signature)
	if err != nil {
		return err
	}
	if _, ok := Plans[plan]; !ok {
		return ErrInvalidPlan
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.repo.SetPlan(ctx, workspaceID, plan)
}

// CheckTunnelQuota enforces the per-plan active-tunnel limit. It satisfies the
// tunnels package's quota-checker dependency.
func (s *Service) CheckTunnelQuota(ctx context.Context, userID uuid.UUID) error {
	active, plan, err := s.repo.ActiveTunnelsForUser(ctx, userID)
	if err != nil {
		return err
	}
	if !withinLimit(active, PlanFor(plan).MaxTunnels) {
		return ErrQuotaExceeded
	}
	return nil
}

func catalog() []Plan {
	out := make([]Plan, 0, len(Plans))
	for _, p := range Plans {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PriceUSD < out[j].PriceUSD })
	return out
}
