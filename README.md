# Gotra

> Go-native tunneling & local-development cloud platform, with an embedded AI debugging layer.

Securely expose your `localhost` to the public internet, capture and replay
every request, and let an AI debugging layer explain errors, investigate
incidents, and recommend fixes — built for developers.

This is a **Turborepo monorepo**: a Go backend (Gin) and a Next.js dashboard +
marketing site, sharing a purple design system.

## Layout

| Path | What |
| --- | --- |
| `apps/web` | Next.js App Router — dashboard + marketing site |
| `services` | Go module — `cmd/api`, `cmd/gateway`, `cmd/agent`, `internal/*`, `pkg/*` |
| `services/migrations` | PostgreSQL schema migrations |
| `packages/design-tokens` | Shared purple design tokens |
| `docs/` | [Architecture overview](docs/ARCHITECTURE.md) · [Original brief](docs/ORIGINAL_BRIEF.md) |

The `.docx` files at the repo root are the source specification bibles.

## Quick start

```bash
# 1. Environment
cp .env.example .env

# 2. Infra (Postgres + Redis)
make infra-up           # or: docker compose up -d

# 3. Backend (Gin API on :8080)
make migrate-up         # apply schema
make api

# 4. Frontend (Next.js on :3000)
pnpm install
pnpm --filter @gotra/web dev
```

## Run a tunnel

```bash
# Terminal 1 — tunnel gateway (:8081)
make gateway

# Terminal 2 — your local app (any port), e.g.
python3 -m http.server 9999

# Terminal 3 — expose it. Get a token via POST /auth/login and a project id
# via GET /projects, then:
cd services
go run ./cmd/agent http 9999 --token <access_jwt> --project <project_id>
#   → https://<sub>.tunnels.gotra.local  →  http://localhost:9999
```

Public requests hit the gateway and are matched to the agent by the tunnel's
Host subdomain. For local testing without DNS, set the Host header explicitly:

```bash
curl -H "Host: <sub>.tunnels.gotra.local" http://localhost:8081/
```

## Tech

- **Backend:** Go 1.24 · Gin · PostgreSQL · Redis · JWT + Argon2id · RBAC
- **Frontend:** Next.js (App Router) · TypeScript · Tailwind · shadcn/ui · Zustand · TanStack Query
- **AI:** Gemini (primary) → Claude (fallback), structured-JSON outputs with confidence scoring
- **Tooling:** Turborepo · pnpm · Docker Compose

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for the full picture.

## Status

Foundation scaffold — structure, both apps boot, full DB schema, infra, and
design tokens are in place. Feature logic is stubbed and filled in iteratively.
