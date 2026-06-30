# Deploying Gotra

## Images

```bash
# Go services (one image per binary)
docker build -t ghcr.io/gotra/gotra-api      --build-arg SERVICE=api      ./services
docker build -t ghcr.io/gotra/gotra-gateway  --build-arg SERVICE=gateway  ./services

# Web (build from repo root so the pnpm workspace resolves)
docker build -t ghcr.io/gotra/gotra-web -f apps/web/Dockerfile \
  --build-arg NEXT_PUBLIC_API_URL=https://api.gotra.dev .
```

## Local production-style stack

```bash
cp .env.example .env            # fill secrets
docker compose -f docker-compose.prod.yml up -d --build
# with analytics:
docker compose -f docker-compose.prod.yml --profile analytics up -d --build
```

The `migrate` service runs `migrate up` once; `api`/`gateway` wait for it.

## Kubernetes

```bash
kubectl apply -f deploy/k8s/00-namespace-config.yaml   # edit config; create the real Secret
kubectl apply -f deploy/k8s/10-migrate-job.yaml         # run before each rollout
kubectl apply -f deploy/k8s/20-api.yaml
kubectl apply -f deploy/k8s/30-gateway.yaml
kubectl apply -f deploy/k8s/40-web-ingress.yaml
```

- **API/Web**: behind the Ingress (cert-manager TLS) at `api.` / `app.` hosts.
- **Gateway**: its own `LoadBalancer`; point **wildcard DNS** `*.tunnels.gotra.dev`
  at it. Autocert issues a TLS cert per tunnel hostname. Multiple replicas
  coordinate tunnel routing through Redis (see the cluster registry).
- Provide managed **Postgres/Redis** (and **ClickHouse** if analytics is on) via
  the Secret/ConfigMap; the manifests assume external datastores.

See [`../docs/OPERATIONS.md`](../docs/OPERATIONS.md) for the full runbook.
