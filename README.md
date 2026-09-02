# HQuizlet Platform

Nền tảng học tập dạng flashcard – viết lại bằng Go (backend) và React (frontend).

## Tech Stack

| Layer    | Tech |
|----------|------|
| Gateway  | Go 1.23 |
| Auth     | Go 1.23 + PostgreSQL |
| Study    | Go 1.23 + PostgreSQL |
| Quiz     | Go 1.23 + deterministic engine |
| Frontend | React + Vite + TypeScript |
| DB       | PostgreSQL 16 |
| Cache    | Redis 7 |
| Broker   | NATS 2 |
| Rust     | quiz-core crate (algorithm spec) |
| Infra    | Docker Compose |

---

## Setup Dev (Docker – nhanh nhất)

### Yêu cầu

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) ≥ 24
- Git

### Chạy lần đầu

```bash
git clone https://github.com/hunguyen1324/hquizlet-platform.git
cd hquizlet-platform

# Copy env và điền giá trị (bắt buộc)
cp .env.example .env

# Build và chạy toàn bộ stack
docker compose -f infra/docker/docker-compose.yml up --build
```

Sau khi các container healthy:

| Service  | URL |
|----------|-----|
| Gateway  | http://localhost:8080 |
| Auth     | http://localhost:8081 |
| Study    | http://localhost:8082 |
| Quiz     | http://localhost:8083 |
| Web      | http://localhost:5173 |
| Postgres | localhost:5432 |

### Kiểm tra stack

```bash
# Gateway health
curl http://localhost:8080/healthz

# Tất cả service
curl http://localhost:8080/healthz/services
```

---

## Setup Dev (Local – không dùng Docker)

### Yêu cầu

- Go ≥ 1.22
- Node.js ≥ 20
- pnpm ≥ 9
- PostgreSQL 16 đang chạy local

### Chạy từng service

```bash
# 1. Postgres – tạo DB (nếu chưa có)
createdb hquizlet

# 2. Gateway
cd services/gateway
go run . &

# 3. Auth service
cd services/auth
DATABASE_URL="postgres://localhost/hquizlet?sslmode=disable" \
JWT_SECRET="dev-secret" \
go run ./cmd/server &

# 4. Study service
cd services/study
DATABASE_URL="postgres://localhost/hquizlet?sslmode=disable" \
go run ./cmd/server &

# 5. Frontend
cd apps/web
pnpm install
pnpm dev
```

---

## Migration

Migration SQL nằm trong từng service:

```
services/auth/migrations/
services/study/migrations/
```

Chạy migration (ví dụ dùng `golang-migrate`):

```bash
migrate -path services/auth/migrations \
        -database "postgres://hquizlet:hquizlet@localhost:5432/hquizlet?sslmode=disable" \
        up
```

---

## API Contract

File OpenAPI đầy đủ: [`packages/api-contracts/openapi.yaml`](packages/api-contracts/openapi.yaml) (v1.4.0)

Xem online: paste nội dung vào [editor.swagger.io](https://editor.swagger.io)

Golden JSON examples: [`packages/api-contracts/examples/quiz/`](packages/api-contracts/examples/quiz/)

Internal API docs: [`packages/api-contracts/quiz-study-internal-api.md`](packages/api-contracts/quiz-study-internal-api.md)

---

## Cấu trúc thư mục

```
hquizlet-platform/
├── apps/
│   └── web/               # React + Vite frontend
├── services/
│   ├── gateway/           # API Gateway (Go)
│   ├── auth/              # Auth service (Go)
│   ├── study/             # Study + Flashcard + Progress service (Go)
│   └── quiz/              # Quiz engine service (Go)
├── crates/
│   ├── quiz-core/         # Rust deterministic quiz engine (spec)
│   └── import-core/       # Rust import engine
├── packages/
│   └── api-contracts/     # OpenAPI spec + golden examples (nguồn sự thật)
├── infra/
│   └── docker/            # Dockerfile + docker-compose.yml
├── docs/
│   └── phase/             # Phase execution plans
├── .env.example
└── README.md
```

---

## Phase 4 – Quiz Architecture

Phase 4 triển khai 4 learning modes (Flashcards, Learn, Test, Match) trên dữ liệu thật.

### Sequence Flow

```
Frontend → Gateway → Auth (verify token)
                  → Quiz Service → Study Service (GET /internal/.../flashcards)
                  → Quiz engine (shuffle/generate/evaluate)
                  → Frontend renders items
                  → Frontend sends answers → Quiz evaluate → score + cardResults
                  → Frontend saves progress via Progress API (Phase 3)
```

### Key Design Decisions

- **Contract-first**: OpenAPI v1.4.0 freezes before any implementation code.
- **Server-side scoring**: Frontend never computes its own score. Quiz service is the single source of truth.
- **Deterministic**: Same seed + same study set = same output (Rust and Go must match).
- **No Rust FFI in request path**: Go port of the algorithm runs in HTTP. Rust crate is the spec.

### Curl Examples

```bash
# Generate flashcards quiz
curl -X POST http://localhost:8080/v1/study-sets/101/quiz/generate \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"mode": "flashcards", "seed": 42, "limit": 100}'

# Generate test questions
curl -X POST http://localhost:8080/v1/study-sets/101/quiz/generate \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"mode": "test", "seed": 42, "limit": 100}'

# Evaluate answers (learn mode)
curl -X POST http://localhost:8080/v1/study-sets/101/quiz/evaluate \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"mode": "learn", "seed": 7, "answers": [{"flashcardId": 2001, "submitted": "GET", "attempts": 1}]}'

# Generate match pairs
curl -X POST http://localhost:8080/v1/study-sets/101/quiz/generate \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"mode": "match", "seed": 42}'
```

---

## Team Phase 4

| Dev | Role | Phạm vi |
|-----|------|---------|
| Dev 1 | Contract, Gateway & Integration Owner | OpenAPI, golden examples, gateway routing, CI gate |
| Dev 2 | Rust quiz-core Owner | Deterministic engine, golden vectors, benchmarks |
| Dev 3 | Go Quiz Service Owner | API handlers, Go port, ownership, validation |
| Dev 4 | Frontend Flashcards & Learn Owner | Flashcards/Learn UX, API integration |
| Dev 5 | Frontend Match/Test & E2E Owner | Match/Test UX, Docker E2E, release gate |

---

## Troubleshooting

**Docker build lỗi Go module:**
```bash
docker compose -f infra/docker/docker-compose.yml build --no-cache
```

**Postgres không kết nối được:**
```bash
docker compose -f infra/docker/docker-compose.yml logs postgres
```

**Port đang bị dùng:**
```bash
lsof -i :8080   # tìm process
kill -9 <PID>
```

---

> Không commit `.env`, secret, hay token vào git. Dùng `.env.example` làm template.

## Phase 5 – Folder API

```bash
# Create a folder
curl -X POST http://localhost:8080/v1/folders \
  -H "Authorization: Bearer <token>" -H "Content-Type: application/json" \
  -d '{"title":"English","description":"IELTS vocabulary"}'

# Add/remove a study set (neither operation changes the source study set)
curl -X POST http://localhost:8080/v1/folders/<folderId>/study-sets \
  -H "Authorization: Bearer <token>" -H "Content-Type: application/json" \
  -d '{"studySetId":10}'
curl -X DELETE http://localhost:8080/v1/folders/<folderId>/study-sets/10 \
  -H "Authorization: Bearer <token>"
```

Run the fresh-stack API gate with `bash infra/scripts/phase5-e2e.sh`.
