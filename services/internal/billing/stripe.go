package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/webhook"

	"github.com/gotra/gotra/internal/config"
)

// ErrUnhandledEvent is returned for Stripe events we don't act on.
var ErrUnhandledEvent = errors.New("billing: unhandled stripe event")

// StripeProcessor implements checkout + webhook confirmation via Stripe.
type StripeProcessor struct {
	webhookSecret string
	appBaseURL    string
	priceFor      map[string]string
}

// NewStripeProcessor configures the Stripe SDK and price mapping.
func NewStripeProcessor(cfg *config.Config) *StripeProcessor {
	stripe.Key = cfg.Stripe.SecretKey
	return &StripeProcessor{
		webhookSecret: cfg.Stripe.WebhookSecret,
		appBaseURL:    cfg.AppBaseURL,
		priceFor: map[string]string{
			"pro":  cfg.Stripe.PricePro,
			"team": cfg.Stripe.PriceTeam,
		},
	}
}

// StartCheckout creates a Stripe Checkout session for a paid plan. The free
// plan is a downgrade and applies immediately (empty URL).
func (p *StripeProcessor) StartCheckout(_ context.Context, workspaceID uuid.UUID, plan string) (string, error) {
	if plan == "free" {
		return "", nil
	}
	price := p.priceFor[plan]
	if price == "" {
		return "", fmt.Errorf("billing: no stripe price configured for plan %q", plan)
	}

	params := &stripe.CheckoutSessionParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: stripe.String(price), Quantity: stripe.Int64(1)},
		},
		SuccessURL:        stripe.String(p.appBaseURL + "/dashboard/billing?status=success"),
		CancelURL:         stripe.String(p.appBaseURL + "/dashboard/billing?status=cancelled"),
		ClientReferenceID: stripe.String(workspaceID.String()),
	}
	params.AddMetadata("workspace_id", workspaceID.String())
	params.AddMetadata("plan", plan)

	sess, err := session.New(params)
	if err != nil {
		return "", fmt.Errorf("stripe checkout: %w", err)
	}
	return sess.URL, nil
}

// ParseWebhook verifies the Stripe signature and returns the workspace/plan for
// a completed checkout session.
func (p *StripeProcessor) ParseWebhook(payload []byte, signature string) (uuid.UUID, string, error) {
	event, err := webhook.ConstructEvent(payload, signature, p.webhookSecret)
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("stripe webhook verify: %w", err)
	}
	if event.Type != "checkout.session.completed" {
		return uuid.Nil, "", ErrUnhandledEvent
	}

	var cs stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &cs); err != nil {
		return uuid.Nil, "", err
	}
	workspaceID, err := uuid.Parse(cs.Metadata["workspace_id"])
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("stripe webhook: bad workspace_id metadata")
	}
	return workspaceID, cs.Metadata["plan"], nil
}
