# syntax=docker/dockerfile:1
# Next.js standalone build for the Gotra dashboard.
# Build from the REPO ROOT so the pnpm workspace resolves:
#   docker build -t gotra-web -f apps/web/Dockerfile .

FROM node:22-alpine AS base
RUN corepack enable

FROM base AS deps
WORKDIR /app
COPY pnpm-lock.yaml pnpm-workspace.yaml package.json ./
COPY apps/web/package.json ./apps/web/
COPY packages ./packages
RUN pnpm install --frozen-lockfile

FROM base AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
ARG NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL}
RUN pnpm --filter @gotra/web build

FROM base AS run
WORKDIR /app
ENV NODE_ENV=production
ENV PORT=3000
# Standalone output mirrors the monorepo layout under .next/standalone.
COPY --from=build /app/apps/web/.next/standalone ./
COPY --from=build /app/apps/web/.next/static ./apps/web/.next/static
COPY --from=build /app/apps/web/public ./apps/web/public
USER node
EXPOSE 3000
CMD ["node", "apps/web/server.js"]
