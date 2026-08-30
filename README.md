# HQuizlet Platform

Microservices-ready rewrite of HQuizlet.

This repository separates frontend, backend services, shared contracts, Rust
cores, and infrastructure. The first milestone is a working platform skeleton,
then modules can be migrated gradually from the existing TypeScript monorepo.

## Stack

- Web: React + Vite + TypeScript
- Backend services: Go
- Performance cores: Rust
- API contract: OpenAPI
- Data: PostgreSQL
- Cache/realtime state: Redis
- Events: NATS

## Repository Layout

```text
apps/
  web/
services/
  gateway/
  auth/
  study/
  quiz/
  payment/
  file/
crates/
  quiz-core/
  import-core/
packages/
  api-contracts/
infra/
  docker/
docs/
  architecture/
```

## Local Start

Start infra:

```bash
docker compose -f infra/docker/docker-compose.yml up -d
```

Run a Go service:

```bash
go run ./services/gateway
```

Run Rust checks:

```bash
cargo test --workspace
```

Run the web app:

```bash
cd apps/web
pnpm install
pnpm dev
```

## Migration Strategy

Use the Strangler Fig pattern:

1. Keep the current app running.
2. Define OpenAPI contracts for new service boundaries.
3. Add Go services beside the old app.
4. Migrate one module at a time.
5. Retire old tRPC/NextAuth paths only after replacement endpoints are stable.
