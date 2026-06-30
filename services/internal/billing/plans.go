// Package billing implements subscription plans, usage metering and quota
// enforcement (Backend Bible — Billing Service). Payment processing is behind a
// Processor interface so a real provider (e.g. Stripe) can be dropped in; the
// default StubProcessor changes plans without charging.
package billing

// Plan describes a subscription tier and its limits. -1 means unlimited.
type Plan struct {
	Name              string `json:"name"`
	PriceUSD          int    `json:"price_usd"`
	MaxProjects       int    `json:"max_projects"`
	MaxTunnels        int    `json:"max_tunnels"`
	MaxRequestsPerDay int    `json:"max_requests_per_day"`
}

// Plans is the catalog of available tiers.
var Plans = map[string]Plan{
	"free": {Name: "free", PriceUSD: 0, MaxProjects: 3, MaxTunnels: 2, MaxRequestsPerDay: 10_000},
	"pro":  {Name: "pro", PriceUSD: 20, MaxProjects: 25, MaxTunnels: 10, MaxRequestsPerDay: 1_000_000},
	"team": {Name: "team", PriceUSD: 99, MaxProjects: 200, MaxTunnels: 50, MaxRequestsPerDay: -1},
}

// PlanFor returns the plan for a name, defaulting to free.
func PlanFor(name string) Plan {
	if p, ok := Plans[name]; ok {
		return p
	}
	return Plans["free"]
}

// withinLimit reports whether current usage is below a (possibly unlimited) cap.
func withinLimit(current, limit int) bool {
	if limit < 0 {
		return true // unlimited
	}
	return current < limit
}
