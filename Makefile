.PHONY: help infra-up infra-down api gateway agent build-go vet web dev migrate-up

help:
	@echo "Gotra monorepo — common commands"
	@echo "  make infra-up     Start Postgres + Redis (docker compose)"
	@echo "  make infra-down   Stop infra"
	@echo "  make api          Run the core API (Gin)"
	@echo "  make gateway      Run the tunnel gateway"
	@echo "  make agent        Run the CLI agent"
	@echo "  make build-go     Build all Go binaries into services/bin"
	@echo "  make vet          go vet the backend"
	@echo "  make web          Run the Next.js dashboard (apps/web)"
	@echo "  make migrate-up   Apply database migrations"

infra-up:
	docker compose up -d

infra-down:
	docker compose down

api:
	cd services && go run ./cmd/api

gateway:
	cd services && go run ./cmd/gateway

agent:
	cd services && go run ./cmd/agent

build-go:
	cd services && go build -o bin/api ./cmd/api && go build -o bin/gateway ./cmd/gateway && go build -o bin/agent ./cmd/agent

vet:
	cd services && go vet ./...

web:
	pnpm --filter @gotra/web dev

migrate-up:
	cd services && go run ./cmd/api migrate up
