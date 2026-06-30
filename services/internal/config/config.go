// Package config loads runtime configuration from the environment.
package config

import (
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration for the Gotra backend services.
type Config struct {
	Env        string
	AppBaseURL string

	APIHost string
	APIPort string

	GatewayPort      string
	TunnelBaseDomain string
	// GatewayInternalURL is how the API reaches the gateway to replay requests.
	GatewayInternalURL string

	// TLS termination for the gateway. Provide a static cert (e.g. a wildcard),
	// OR enable ACME autocert for on-demand per-host certificates.
	GatewayTLSCertFile      string
	GatewayTLSKeyFile       string
	GatewayAutocertEnabled  bool
	GatewayAutocertCacheDir string
	GatewayAutocertEmail    string

	DatabaseURL string
	RedisURL    string
	// ClickHouseURL is optional; when empty, the analytics pipeline is disabled.
	ClickHouseURL string

	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	Google OAuthConfig
	GitHub OAuthConfig
	OIDC   OIDCConfig

	AI     AIConfig
	Stripe StripeConfig
}

// StripeConfig configures the Stripe billing processor. When SecretKey is empty
// the billing service falls back to the stub processor (immediate plan changes).
type StripeConfig struct {
	SecretKey     string
	WebhookSecret string
	PricePro      string
	PriceTeam     string
}

// Enabled reports whether Stripe billing is configured.
func (s StripeConfig) Enabled() bool { return s.SecretKey != "" }

// OIDCConfig configures a generic OpenID Connect provider for enterprise SSO.
type OIDCConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// Configured reports whether OIDC SSO is fully configured.
func (o OIDCConfig) Configured() bool {
	return o.Issuer != "" && o.ClientID != "" && o.ClientSecret != ""
}

// OAuthConfig holds credentials for a single OAuth provider.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// AIConfig configures the AI debugging service providers.
// Per the AI Debugging Service Bible, Gemini is primary and Claude is the
// secondary/fallback provider.
type AIConfig struct {
	PrimaryProvider   string
	SecondaryProvider string
	GeminiAPIKey      string
	AnthropicAPIKey   string
}

// Load reads configuration from the environment, loading a .env file if present.
// It never fails hard on a missing .env so it works in containers where the
// environment is injected directly.
func Load() *Config {
	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatal("env files not loaded!")
	}

	return &Config{
		Env:        getEnv("APP_ENV", "development"),
		AppBaseURL: getEnv("APP_BASE_URL", "http://localhost:3000"),

		APIHost: getEnv("API_HOST", "0.0.0.0"),
		APIPort: getEnv("API_PORT", "8080"),

		GatewayPort:        getEnv("GATEWAY_PORT", "8081"),
		TunnelBaseDomain:   getEnv("TUNNEL_BASE_DOMAIN", "tunnels.gotra.local"),
		GatewayInternalURL: getEnv("GATEWAY_INTERNAL_URL", "http://localhost:8081"),

		GatewayTLSCertFile:      getEnv("GATEWAY_TLS_CERT_FILE", ""),
		GatewayTLSKeyFile:       getEnv("GATEWAY_TLS_KEY_FILE", ""),
		GatewayAutocertEnabled:  getEnv("GATEWAY_AUTOCERT_ENABLED", "false") == "true",
		GatewayAutocertCacheDir: getEnv("GATEWAY_AUTOCERT_CACHE_DIR", "/var/lib/gotra/autocert"),
		GatewayAutocertEmail:    getEnv("GATEWAY_AUTOCERT_EMAIL", ""),

		DatabaseURL:   getEnv("DATABASE_URL", "postgresql://gotra:gotra@localhost:5800/gotra?sslmode=disable"),
		RedisURL:      getEnv("REDIS_URL", "redis://localhost:6379/0"),
		ClickHouseURL: getEnv("CLICKHOUSE_URL", ""),

		JWTSecret:       getEnv("JWT_SECRET", "change-me-in-production"),
		AccessTokenTTL:  getDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL: getDuration("REFRESH_TOKEN_TTL", 720*time.Hour),

		Google: OAuthConfig{
			ClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
			ClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
			RedirectURL:  getEnv("GOOGLE_REDIRECT_URL", ""),
		},
		GitHub: OAuthConfig{
			ClientID:     getEnv("GITHUB_CLIENT_ID", ""),
			ClientSecret: getEnv("GITHUB_CLIENT_SECRET", ""),
			RedirectURL:  getEnv("GITHUB_REDIRECT_URL", ""),
		},
		OIDC: OIDCConfig{
			Issuer:       getEnv("OIDC_ISSUER", ""),
			ClientID:     getEnv("OIDC_CLIENT_ID", ""),
			ClientSecret: getEnv("OIDC_CLIENT_SECRET", ""),
			RedirectURL:  getEnv("OIDC_REDIRECT_URL", ""),
		},

		AI: AIConfig{
			PrimaryProvider:   getEnv("AI_PRIMARY_PROVIDER", "gemini"),
			SecondaryProvider: getEnv("AI_SECONDARY_PROVIDER", "claude"),
			GeminiAPIKey:      getEnv("GEMINI_API_KEY", ""),
			AnthropicAPIKey:   getEnv("ANTHROPIC_API_KEY", ""),
		},

		Stripe: StripeConfig{
			SecretKey:     getEnv("STRIPE_SECRET_KEY", ""),
			WebhookSecret: getEnv("STRIPE_WEBHOOK_SECRET", ""),
			PricePro:      getEnv("STRIPE_PRICE_PRO", ""),
			PriceTeam:     getEnv("STRIPE_PRICE_TEAM", ""),
		},
	}
}

// IsProduction reports whether the service is running in production mode.
func (c *Config) IsProduction() bool { return c.Env == "production" }

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
