# HQuizlet Platform

Nền tảng học tập dạng flashcard – viết lại bằng Go (backend) và React (frontend).

## Tech Stack

| Layer    | Tech |
|----------|------|
| Gateway  | Go 1.22 |
| Auth     | Go 1.22 + PostgreSQL |
| Study    | Go 1.22 + PostgreSQL |
| Frontend | React + Vite + TypeScript |
| DB       | PostgreSQL 16 |
| Cache    | Redis 7 |
| Broker   | NATS 2 |
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

File OpenAPI đầy đủ: [`packages/api-contracts/openapi.yaml`](packages/api-contracts/openapi.yaml)

Xem online: paste nội dung vào [editor.swagger.io](https://editor.swagger.io)

---

## Cấu trúc thư mục

```
hquizlet-platform/
├── apps/
│   └── web/               # React + Vite frontend
├── services/
│   ├── gateway/           # API Gateway (Go)
│   ├── auth/              # Auth service (Go)
│   ├── study/             # Study + Flashcard service (Go)
│   └── quiz/              # Live quiz service (Go)
├── packages/
│   └── api-contracts/     # OpenAPI spec (nguồn sự thật)
├── infra/
│   └── docker/            # Dockerfile + docker-compose.yml
├── docs/
│   └── migration/         # Kế hoạch Sprint
├── .env.example
└── README.md
```

---

## Team Sprint 1

| Dev | Role | Phạm vi |
|-----|------|---------|
| Dev 1 | Backend Go – Auth | `services/auth/**` |
| Dev 2 | Backend Go – Study | `services/study/**` |
| Dev 3 | Frontend React – Core | `apps/web/src/features/auth,dashboard,study-sets` |
| Dev 4 | Frontend React – Learning | `apps/web/src/features/learning` |
| Dev 5 | Fullstack/Integration | `services/gateway, infra, packages/api-contracts, docs` |

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
