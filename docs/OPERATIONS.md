# Gotra — Operations Runbook

How to run, deploy, and operate Gotra. For architecture see
[`ARCHITECTURE.md`](ARCHITECTURE.md).

## Services

| Process | Binary | Default port | Role |
| --- | --- | --- | --- |
| API | `services/cmd/api` | 8080 | Core REST API (auth, projects, tunnels, requests, AI, analytics, billing) |
| Gateway | `services/cmd/gateway` | 8081 | Tunnel data plane: agent WebSocket + public HTTP forwarding |
| Agent | `services/cmd/agent` | — | CLI run on the developer's machine |
| Web | `apps/web` | 3000 | Next.js dashboard + marketing |

Datastores: **PostgreSQL** (core), **Redis** (sessions/cache/OAuth state),
**ClickHouse** (optional analytics).

## Prerequisites

- Go 1.25+, Node 20+, pnpm 10+
- Docker/Podman (compose) for local infra
- A registry mirror may be required for `pnpm install` (see repo `.npmrc`)

## Local quick start

```bash
cp .env.example .env
docker compose up -d                 # Postgres + Redis
cd services && go run ./cmd/api migrate up
make api        # :8080   (separate terminals)
make gateway    # :8081
pnpm install && pnpm --filter @gotra/web dev   # :3000
```

Enable analytics locally:

```bash
docker compose --profile analytics up -d clickhouse
# set in .env: CLICKHOUSE_URL=clickhouse://gotra:gotra@localhost:9000/gotra
```

## Configuration

All config is environment-driven (see `.env.example` for the full list). Key groups:

- **Core/API** — `APP_ENV`, `API_PORT`, `APP_BASE_URL`
- **Datastores** — `DATABASE_URL`, `REDIS_URL`, `CLICKHOUSE_URL` (empty = analytics off)
- **Auth** — `JWT_SECRET` (rotate per environment), `ACCESS_TOKEN_TTL`, `REFRESH_TOKEN_TTL`
- **OAuth/SSO** — `GOOGLE_*`, `GITHUB_*`, `OIDC_*` (issuer auto-discovery)
- **AI** — `AI_PRIMARY_PROVIDER` (gemini), `AI_SECONDARY_PROVIDER` (claude), `GEMINI_API_KEY`, `ANTHROPIC_API_KEY` (stub fallback if unset)
- **Gateway/TLS** — `GATEWAY_PORT`, `TUNNEL_BASE_DOMAIN`, `GATEWAY_TLS_*` / `GATEWAY_AUTOCERT_*`
- **Billing** — `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, `STRIPE_PRICE_PRO`, `STRIPE_PRICE_TEAM` (stub processor if unset)

> **Secrets:** never commit real values. Use a secrets manager in production; the
> API reads them from the environment.

## Database migrations

Forward-only SQL in `services/migrations`, tracked in `schema_migrations`.

```bash
cd services && go run ./cmd/api migrate up    # idempotent; applies pending only
```

Run migrations as a one-shot job before rolling out new API/gateway instances.

## Production deployment

1. **Build images** for `api`, `gateway` (and the web app), e.g. `make build-go`
   or container builds; the Backend Bible targets Docker + Kubernetes
   (Deployments/Services/Ingress/ConfigMaps/Secrets).
2. **Run migrations** as an init job.
3. **API** — stateless; scale horizontally behind a load balancer. Needs Postgres
   + Redis reachability.
4. **Gateway** — terminates TLS and holds agent WebSockets.
   - **TLS option A (wildcard cert):** set `GATEWAY_TLS_CERT_FILE` / `GATEWAY_TLS_KEY_FILE`
     to a `*.TUNNEL_BASE_DOMAIN` certificate.
   - **TLS option B (ACME autocert):** set `GATEWAY_AUTOCERT_ENABLED=true` and
     expose `:443`; a certificate is issued per tunnel hostname on first request.
   - **DNS:** point `*.TUNNEL_BASE_DOMAIN` (wildcard A/AAAA) at the gateway's
     public IP / load balancer.
   - The current registry is in-memory (single instance). For multi-instance
     gateways, front them with consistent hostname routing or add the Redis
     pub/sub registry described in the Backend Bible.
5. **Web** — deploy the Next.js app; set `NEXT_PUBLIC_API_URL` to the public API URL.

## Scaling

- **API**: horizontal, stateless.
- **Postgres**: read replicas for read-heavy endpoints.
- **Redis**: cluster for sessions/rate limits.
- **ClickHouse**: cluster for analytics volume.
- **Gateway**: horizontal once the shared registry (Redis pub/sub) is enabled.

## Backups & DR

- Postgres: daily backups + point-in-time recovery; test restores.
- ClickHouse: analytics is reconstructable from capture; back up if used as source of truth.
- Store backups multi-region; keep recovery runbooks current.

## Observability

- Structured JSON logs in production (`APP_ENV=production`).
- Wire OpenTelemetry/Prometheus/Grafana per the Backend Bible (request count,
  latency, tunnel count, error rate, replay/AI usage).
- Health endpoints: `GET /health` on both API and gateway.

## Billing (Stripe)

- Without `STRIPE_SECRET_KEY`, the **stub processor** applies plan changes
  immediately (dev).
- With Stripe configured, paid-plan changes return a Checkout URL; the plan is
  applied when Stripe calls `POST /api/v1/billing/webhook`
  (`checkout.session.completed`). Configure that webhook in the Stripe dashboard
  and set `STRIPE_WEBHOOK_SECRET`.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| 502 on a tunnel URL | Agent connected? `tunnels.status` in DB; gateway logs |
| 402 creating a tunnel | Plan tunnel quota reached — upgrade in Billing |
| Analytics empty | `CLICKHOUSE_URL` set + reachable; look for "analytics enabled" in logs |
| OAuth/SSO 503 | Provider not configured (missing client id/secret/issuer) |
| AI returns "stub" provider | No `GEMINI_API_KEY` / `ANTHROPIC_API_KEY` set |
| Migrations fail | Inspect `schema_migrations`; ensure DB user can create extensions |

## CI

`.github/workflows/ci.yml` runs Go lint/test/build and web typecheck/build on
every push/PR. Integration tests run with `-tags=integration` against a Postgres
service (see below).
