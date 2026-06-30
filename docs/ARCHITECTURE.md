# Gotra — Architecture Overview

Gotra is a **Go-native tunneling and local-development cloud platform** (securely
expose `localhost` to the public internet) with request capture, replay,
analytics, and an embedded **AI debugging layer**.

This document consolidates the five source bibles into one reference. The
detailed specs live in the `.docx` files at the repo root.

## Monorepo layout

```
gotra/
├── apps/
│   └── web/                 # Next.js App Router — dashboard + marketing site
├── services/                # Single Go module (module: github.com/gotra/gotra)
│   ├── cmd/
│   │   ├── api/             # Core API service (Gin)
│   │   ├── gateway/         # Tunnel gateway (TLS termination, routing)
│   │   └── agent/           # CLI agent (runs on the developer's machine)
│   ├── internal/            # Domain modules: auth, projects, tunnels, ai, ...
│   ├── pkg/                 # Reusable infra: database, cache, security, logger, middleware
│   └── migrations/          # SQL schema migrations
├── packages/
│   ├── design-tokens/       # Purple design system tokens (shared)
│   └── tsconfig/            # Shared TS config
├── docker-compose.yml       # Postgres + Redis (first-pass infra)
└── turbo.json               # Turborepo task pipeline
```

> The three backend binaries (`api`, `gateway`, `agent`) live in **one** Go
> module under `services/`, matching the Backend Bible's `cmd/* + internal/* + pkg/*`
> layout, rather than three separate modules.

## Backend stack

- **Language / framework:** Go + **Gin** (explicitly *not* Fiber)
- **Datastores (this pass):** PostgreSQL (core), Redis (sessions/cache)
- **Future datastores:** ClickHouse (analytics), S3-compatible blob storage
- **Auth:** Argon2id password hashing, JWT access tokens (15m) + rotating refresh tokens (30d), RBAC (Owner/Admin/Developer/Viewer)
- **Agent ↔ Gateway:** WebSocket over TLS

### Core API endpoints (from the bibles)

```
POST   /auth/register            POST /auth/login            POST /auth/refresh
GET    /auth/google  /github     (+ /callback)
GET    /projects                 POST /projects
GET    /tunnels                  POST /tunnels               DELETE /tunnels/:id
GET    /requests                 GET  /requests/:id          POST  /requests/:id/replay
POST   /ai/explain-error         POST /ai/explain-logs       POST  /ai/analyze-request
POST   /ai/analyze-replay        POST /ai/generate-incident
GET    /ai/analyses  /incidents  /reports  /history
```

## AI Debugging Service

Converts requests, responses, logs, replay data and metrics into actionable
insights. Pipeline: **Context Builder → Prompt Engine → LLM Orchestrator →
Analysis Engine → Confidence Engine → Result Store** (with a Cost Controller).

- **Provider order:** Gemini (primary) → Claude (secondary/fallback)
- **All model outputs are structured JSON**, with a 0–100 confidence score
- MVP features: Explain Error, Explain Logs, Analyze Request, Analyze Replay, Generate Incident, Suggest Fixes

## Frontend stack

- **Next.js App Router**, TypeScript strict, Tailwind, **shadcn/ui**, Zustand, TanStack Query
- **Design system:** purple-first, dark-mode default; reference dashboard density of an enterprise SaaS tool
- **Dashboard pages:** Overview, Projects, Tunnels, Tunnel Details, Request Inspector, Replay Center, Analytics, Teams, Billing, Settings, Notifications
- **AI surfaces:** Insights, Incidents, Investigations, Reports, History, Copilot, Settings
- **Marketing site:** premium, motion-driven landing experience (separate from the productivity-focused dashboard)

## Build status

This is the **foundation scaffold**: structure, both apps boot, full DB schema,
infra via docker-compose, and design tokens. Feature logic is stubbed and will
be filled in iteratively.
