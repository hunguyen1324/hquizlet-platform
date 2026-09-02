# HQuizlet Platform - Phase 9 Execution Plan (5 Developers)

## 1. Muc Tieu Phase 9

Phase 9 xay dung File Upload domain end-to-end: avatar user, anh minh hoa
flashcard, thumbnail study set. Backend dung presigned URL (MinIO/S3/R2 tuong
thich), file service luu metadata vao PostgreSQL, gateway route va bao ve
upload. Frontend upload truc tiep len storage, khong qua gateway, giam tai
server.

Ket qua cuoi phase:

- User upload avatar → hien thi tren header va trang profile.
- Flashcard co anh minh hoa upload tu may tinh hoac URL.
- Study set co thumbnail tuy chinh.
- File service validate MIME type, kich thuoc, so luong file per user.
- Storage co the la MinIO (dev local) hoac S3/Cloudflare R2 (production).
- Presigned URL co TTL ngan (15 phut), chong phep upload nguoi khac.
- File metadata luu PostgreSQL: owner, MIME, kich thuoc, trang thai.
- `services/file` thay the stub hien tai bang clean architecture day du.
- Gateway route toan bo `/v1/files/*` qua File service voi auth va header
  stripping.
- OpenAPI, migrations, backend/frontend tests, Docker fresh-volume E2E deu
  xanh truoc khi danh dau GO.

---

## 2. Baseline Va Dieu Kien Bat Dau

### 2.1 Trang thai repo hien tai

- Phase 8 GO gate: Phase 9 chi duoc bat dau khi Phase 8 release gate da dong
  (fresh-volume E2E co evidence day du).
- Gateway hien co auth (8081), study (8082), quiz (8083), class (8084),
  payment (8085).
- `services/file` hien chi la stub `main.go` voi 1 route placeholder
  `POST /v1/files/presign-upload` tra hardcoded JSON. Phase 9 replace toan bo.
- PostgreSQL chua co bang file. Phase 9 them migration moi.
- Frontend chua co upload component nao.
- OpenAPI hien o version `1.6.0`; Phase 9 tang len `1.7.0`.
- `infra/docker/docker-compose.yml` chua co MinIO va chua co `file` service
  trong depends_on cua gateway.

### 2.2 Prerequisite tu Phase 8

- Dong Phase 8 fresh-volume gate: gan commit SHA + output vao Phase 8 release
  gate report truoc khi merge bat ky production code Phase 9 nao.
- Xac nhan Auth service internal API `/internal/auth/verify` on dinh.
- Xac nhan Study service co `PUT /v1/study-sets/{id}` de cap nhat
  `thumbnail_url`.
- Xac nhan Auth service co `PUT /v1/auth/profile` de cap nhat `image`.
- Giu toan bo Phase 4–8 golden tests xanh. Phase 9 khong duoc tao regression.

---

## 3. Pham Vi

### 3.1 In scope

- `services/file`: service Go moi voi clean architecture day du (config,
  http/handler, service, repository, model, migration, middleware).
- PostgreSQL migrations: bang `uploaded_file` luu metadata.
- MinIO trong Docker Compose cho dev local (S3-compatible, khong can cloud).
- Presigned URL flow: client xin URL → upload truc tiep len MinIO/S3 →
  confirm len file service → file service verify va active metadata.
- Upload types: `avatar` (user), `flashcard_image` (study), `study_set_thumbnail`
  (study).
- Validation: MIME whitelist, max file size (5MB images, 20MB audio/video
  neu co sau), max files per user (100 active files).
- Soft delete: `DELETE /v1/files/{id}` danh dau `deleted_at`, khong xoa S3
  object ngay (cleanup batch sau).
- Study service: them column `image_url` vao `flashcards`, `thumbnail_url` vao
  `study_sets`, cap nhat API cho phep set URL sau khi upload confirm.
- Auth service: them column `image` (avatar) vao `users` neu chua co, cap nhat
  `GET /v1/auth/me` tra ve URL day du.
- Gateway routes cho `/v1/files/*`.
- Docker Compose: them `minio` service va `file` service (port 8086).
- MinIO console expose port 9001 cho dev.
- OpenAPI `1.7.0`: them tat ca File schemas va endpoints, kem examples.
- Frontend: FileUpload component tai su dung, AvatarUpload, FlashcardImageUpload,
  StudySetThumbnailUpload.
- Backend tests, frontend tests, security tests, integration test, E2E test.
- Phase 9 release gate report.
- Environment variables documentation cho STORAGE_*.

### 3.2 Out of scope

- Video/audio upload (Phase 10+ neu can).
- Image resize/thumbnail generation tren server (CDN hoac client-side).
- Virus scan (chi validate MIME + magic bytes).
- CDN integration (CloudFront, Cloudflare) — chi direct MinIO/S3 URL.
- Mobile upload UI.
- Bulk import tu URL nhu repo cu `packages/api/src/router/studySet.ts`
  (importFromUrl) — Phase rieng sau.
- File sharing giua users — file la private, gan vao entity cu the.

---

## 4. Kien Truc File Service

### 4.1 Cau truc thu muc

```
services/file/
  cmd/server/main.go              -- entry point (thay the root main.go hien tai)
  internal/
    config/
      config.go                   -- doc env, validate STORAGE_*, DATABASE_URL
    http/
      handler/
        file.go                   -- POST /v1/files/presign, POST /v1/files/confirm
                                  -- GET /v1/files/{id}, DELETE /v1/files/{id}
                                  -- GET /v1/files (list files cua user)
      middleware/
        auth.go                   -- doc X-User-Id tu gateway header
        request_id.go
        logging.go
      router.go                   -- wiring tat ca routes
    model/
      file.go                     -- UploadedFile, PresignRequest, ConfirmRequest
    repository/
      file_repo.go                -- CreateFile, GetFile, ListByUser, SoftDelete,
                                  -- MarkActive, CountActiveByUser
    service/
      file_svc.go                 -- business logic: validate, presign, confirm,
                                  -- delete, quota check
    storage/
      client.go                   -- interface Storage, factory (minio/s3/r2)
      minio.go                    -- MinIO implementation (aws-sdk-go-v2 + path-style)
      s3.go                       -- AWS S3 / R2 implementation (same SDK)
    store/
      db.go                       -- pgx pool init + ping
  migrations/
    001_uploaded_file.sql
  go.mod
  Dockerfile
```

### 4.2 Port map (cap nhat)

| Service  | Port  |
|----------|-------|
| gateway  | 8080  |
| auth     | 8081  |
| study    | 8082  |
| quiz     | 8083  |
| class    | 8084  |
| payment  | 8085  |
| **file** | **8086** |
| MinIO API | 9000 |
| MinIO Console | 9001 |

### 4.3 Upload flow

```
Client                  Gateway             File Service        MinIO/S3
  |                        |                     |                  |
  |-- POST /v1/files/presign ------------------>|                  |
  |   { upload_type, filename, content_type,    |                  |
  |     file_size }                             |                  |
  |                                             |-- validate ------>|
  |                                             |<- presigned URL --|
  |                                             |-- INSERT file (pending)
  |<----------- { file_id, upload_url } --------|                  |
  |                                             |                  |
  |-- PUT <upload_url> (truc tiep len MinIO/S3) ------------------>|
  |<------------------------------------------------ 200 OK --------|
  |                                             |                  |
  |-- POST /v1/files/{id}/confirm ------------->|                  |
  |                                             |-- HEAD object --->|
  |                                             |<- content-length -|
  |                                             |-- UPDATE file (active)
  |<----------- { file_id, url } --------------|                  |
```

---

## 5. PostgreSQL Migration

### `001_uploaded_file.sql`

```sql
CREATE TABLE IF NOT EXISTS "uploaded_file" (
  "id"           uuid      PRIMARY KEY DEFAULT gen_random_uuid(),
  "user_id"      uuid      NOT NULL,  -- no FK cross-service, validate qua header
  "upload_type"  text      NOT NULL
                           CHECK ("upload_type" IN (
                             'avatar',
                             'flashcard_image',
                             'study_set_thumbnail'
                           )),
  -- Key trong S3/MinIO: "uploads/{user_id}/{upload_type}/{id}/{filename}"
  "storage_key"  text      NOT NULL UNIQUE,
  "filename"     text      NOT NULL,
  "content_type" text      NOT NULL,
  "size_bytes"   bigint    NOT NULL,
  -- public URL sau khi confirm (hoac presigned URL co han neu private)
  "public_url"   text,
  -- pending: presign xong, chua upload
  -- active: da confirm, dang dung
  -- deleted: soft delete
  "status"       text      NOT NULL DEFAULT 'pending'
                           CHECK ("status" IN ('pending', 'active', 'deleted')),
  "created_at"   timestamp NOT NULL DEFAULT now(),
  "confirmed_at" timestamp,
  "deleted_at"   timestamp
);

CREATE INDEX IF NOT EXISTS "idx_uf_user_status"
  ON "uploaded_file" ("user_id", "status");

CREATE INDEX IF NOT EXISTS "idx_uf_user_type_active"
  ON "uploaded_file" ("user_id", "upload_type")
  WHERE "status" = 'active';

-- Cleanup: tim cac file pending qua han de xoa S3 object
CREATE INDEX IF NOT EXISTS "idx_uf_pending_created"
  ON "uploaded_file" ("created_at")
  WHERE "status" = 'pending';
```

---

## 6. Storage Client Interface

### `internal/storage/client.go`

```go
package storage

import (
    "context"
    "time"
)

// Storage la interface thong nhat cho MinIO, S3, R2.
// Cho phep swap backend ma khong doi code service.
type Storage interface {
    // PresignPut sinh presigned URL de client PUT truc tiep.
    // Tra ve URL va thoi diem het han.
    PresignPut(ctx context.Context, key string, contentType string, ttl time.Duration) (url string, expiresAt time.Time, err error)

    // HeadObject kiem tra object ton tai va lay metadata.
    HeadObject(ctx context.Context, key string) (size int64, contentType string, err error)

    // PublicURL tra ve URL truy cap cong khai (neu bucket public)
    // hoac presigned GET URL ngan han (neu bucket private).
    PublicURL(ctx context.Context, key string, ttl time.Duration) (string, error)

    // DeleteObject xoa object khi can (cleanup batch).
    DeleteObject(ctx context.Context, key string) error
}

// New tra ve implementation dua tren STORAGE_PROVIDER env.
// "minio" → MinIO (dev), "s3" → AWS S3, "r2" → Cloudflare R2.
func New(cfg Config) (Storage, error) {
    switch cfg.Provider {
    case "minio":
        return newMinIO(cfg)
    case "s3", "r2":
        return newS3(cfg)
    default:
        return nil, fmt.Errorf("unknown storage provider: %s", cfg.Provider)
    }
}

type Config struct {
    Provider        string // minio | s3 | r2
    Endpoint        string // MinIO: http://minio:9000, R2: https://<account>.r2.cloudflarestorage.com
    Region          string // s3: us-east-1, r2: auto
    Bucket          string
    AccessKeyID     string
    SecretAccessKey string
    PublicBaseURL   string // URL public de sinh link, vi du http://localhost:9000/hquizlet
    PathStyle       bool   // true cho MinIO (path-style), false cho S3
}
```

### `internal/storage/minio.go`

```go
package storage

import (
    "context"
    "fmt"
    "time"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
    "github.com/aws/smithy-go/transport/http"
)

type minioStorage struct {
    client        *s3.Client
    presignClient *s3.PresignClient
    bucket        string
    publicBaseURL string
}

func newMinIO(cfg Config) (Storage, error) {
    customResolver := aws.EndpointResolverWithOptionsFunc(
        func(service, region string, options ...interface{}) (aws.Endpoint, error) {
            return aws.Endpoint{URL: cfg.Endpoint, HostnameImmutable: true}, nil
        },
    )
    awsCfg, err := config.LoadDefaultConfig(context.Background(),
        config.WithRegion("us-east-1"),
        config.WithEndpointResolverWithOptions(customResolver),
        config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
            cfg.AccessKeyID, cfg.SecretAccessKey, "",
        )),
    )
    if err != nil {
        return nil, fmt.Errorf("minio config: %w", err)
    }
    client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
        o.UsePathStyle = true // bat buoc cho MinIO
    })
    return &minioStorage{
        client:        client,
        presignClient: s3.NewPresignClient(client),
        bucket:        cfg.Bucket,
        publicBaseURL: cfg.PublicBaseURL,
    }, nil
}

func (m *minioStorage) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, time.Time, error) {
    req, err := m.presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
        Bucket:      aws.String(m.bucket),
        Key:         aws.String(key),
        ContentType: aws.String(contentType),
    }, s3.WithPresignExpires(ttl))
    if err != nil {
        return "", time.Time{}, fmt.Errorf("presign put: %w", err)
    }
    return req.URL, time.Now().Add(ttl), nil
}

func (m *minioStorage) HeadObject(ctx context.Context, key string) (int64, string, error) {
    resp, err := m.client.HeadObject(ctx, &s3.HeadObjectInput{
        Bucket: aws.String(m.bucket),
        Key:    aws.String(key),
    })
    if err != nil {
        return 0, "", fmt.Errorf("head object: %w", err)
    }
    ct := ""
    if resp.ContentType != nil {
        ct = *resp.ContentType
    }
    size := int64(0)
    if resp.ContentLength != nil {
        size = *resp.ContentLength
    }
    return size, ct, nil
}

func (m *minioStorage) PublicURL(_ context.Context, key string, _ time.Duration) (string, error) {
    return fmt.Sprintf("%s/%s", m.publicBaseURL, key), nil
}

func (m *minioStorage) DeleteObject(ctx context.Context, key string) error {
    _, err := m.client.DeleteObject(ctx, &s3.DeleteObjectInput{
        Bucket: aws.String(m.bucket),
        Key:    aws.String(key),
    })
    return err
}
```

---

## 7. Validation Rules

### 7.1 Upload validation

| Upload type         | MIME whitelist                          | Max size |
|---------------------|------------------------------------------|----------|
| `avatar`            | `image/jpeg`, `image/png`, `image/webp` | 5 MB     |
| `flashcard_image`   | `image/jpeg`, `image/png`, `image/webp`, `image/gif` | 10 MB |
| `study_set_thumbnail` | `image/jpeg`, `image/png`, `image/webp` | 5 MB  |

### 7.2 Quota per user

- Toi da **100 active files** per user (tra 429 neu vuot).
- Tong dung luong toi da **500 MB** per user (tu tinh qua SUM size_bytes WHERE
  status='active').
- Pending file het han sau 15 phut → cleanup batch xoa S3 object + soft delete
  DB row.

### 7.3 Storage key format

```
uploads/{user_id}/{upload_type}/{file_id}/{sanitized_filename}
```

Vi du:
```
uploads/550e8400-e29b-41d4-a716-446655440000/avatar/7f3e1234-abcd-4567-8901-fedcba987654/avatar.jpg
```

- `sanitized_filename`: lowercase, chi `[a-z0-9._-]`, max 100 ky tu.
- `file_id` la UUID tu PostgreSQL, dam bao khong co path traversal.

---

## 8. API Contract (OpenAPI 1.7.0)

### 8.1 File endpoints

| Method | Route                    | Auth     | Mo ta                                     |
|--------|--------------------------|----------|-------------------------------------------|
| POST   | `/v1/files/presign`      | required | Xin presigned URL de upload               |
| POST   | `/v1/files/{id}/confirm` | required | Confirm da upload xong, active metadata   |
| GET    | `/v1/files/{id}`         | required | Lay metadata file (chi owner)             |
| DELETE | `/v1/files/{id}`         | required | Soft delete file (chi owner)              |
| GET    | `/v1/files`              | required | List files cua user (phan trang)          |

### 8.2 Request/Response

**POST /v1/files/presign request:**
```json
{
  "upload_type": "avatar",
  "filename": "my-photo.jpg",
  "content_type": "image/jpeg",
  "file_size": 204800
}
```

**POST /v1/files/presign response:**
```json
{
  "file_id": "7f3e1234-abcd-4567-8901-fedcba987654",
  "upload_url": "http://minio:9000/hquizlet/uploads/...?X-Amz-Signature=...",
  "upload_method": "PUT",
  "expires_at": "2026-09-02T10:15:00Z",
  "headers_required": {
    "Content-Type": "image/jpeg"
  }
}
```

**POST /v1/files/{id}/confirm response:**
```json
{
  "file_id": "7f3e1234-abcd-4567-8901-fedcba987654",
  "url": "http://localhost:9000/hquizlet/uploads/.../my-photo.jpg",
  "upload_type": "avatar",
  "size_bytes": 204800,
  "confirmed_at": "2026-09-02T10:14:30Z"
}
```

**GET /v1/files/{id} response:**
```json
{
  "file_id": "7f3e1234-abcd-4567-8901-fedcba987654",
  "upload_type": "avatar",
  "filename": "my-photo.jpg",
  "content_type": "image/jpeg",
  "size_bytes": 204800,
  "url": "http://localhost:9000/hquizlet/uploads/.../my-photo.jpg",
  "status": "active",
  "created_at": "2026-09-02T10:13:00Z",
  "confirmed_at": "2026-09-02T10:14:30Z"
}
```

**GET /v1/files response:**
```json
{
  "items": [...],
  "total": 12,
  "quota": {
    "active_files": 12,
    "max_files": 100,
    "used_bytes": 2048000,
    "max_bytes": 524288000
  }
}
```

**DELETE /v1/files/{id} response:** HTTP 204 No Content.

### 8.3 Loi tieu biet

| HTTP | Code                    | Khi nao                                     |
|------|-------------------------|---------------------------------------------|
| 400  | `invalid_upload_type`   | upload_type khong hop le                    |
| 400  | `invalid_content_type`  | MIME type khong duoc phep                   |
| 400  | `file_too_large`        | file_size vuot gioi han                     |
| 404  | `file_not_found`        | file_id khong ton tai hoac da xoa           |
| 403  | `not_owner`             | User khong phai owner cua file              |
| 409  | `already_confirmed`     | File da duoc confirm (khong confirm lai)    |
| 409  | `not_yet_uploaded`      | HEAD object khong thay tren MinIO           |
| 429  | `quota_exceeded`        | Vuot gioi han 100 files hoac 500 MB         |

---

## 9. Tich Hop Cac Service Hien Co

### 9.1 Auth service: cap nhat avatar

Sau khi user confirm upload avatar (`file_id`), frontend goi:

```
PUT /v1/auth/profile
Body: { "image_url": "http://...minio.../avatar.jpg" }
```

Auth service can them:
- Column `image text` vao bang `users` neu chua co (Phase 2 da co hoac them migration moi).
- Handler `PUT /v1/auth/profile`: cap nhat `name`, `image` cua user hien tai.
- `GET /v1/auth/me` tra ve `image` field.

Migration auth (neu chua co):
```sql
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS "image" text;
```

### 9.2 Study service: cap nhat flashcard image va set thumbnail

Study service can them:
- Column `image_url text` vao bang `flashcards` (migration moi).
- Column `thumbnail_url text` vao bang `study_sets` (migration moi).
- `PUT /v1/flashcards/{id}` cap nhat `image_url`.
- `PUT /v1/study-sets/{id}` cap nhat `thumbnail_url`.
- `GET /v1/study-sets/{id}` tra ve `thumbnail_url` trong response.
- `GET /v1/study-sets/{id}` tra ve flashcards kem `image_url`.

Migration study:
```sql
ALTER TABLE "flashcards" ADD COLUMN IF NOT EXISTS "image_url" text;
ALTER TABLE "study_sets" ADD COLUMN IF NOT EXISTS "thumbnail_url" text;
```

---

## 10. Phan Chia Cong Viec 5 Developer

### Dev 1: File Service Core + Storage Client (4 ngay)

**Ngay 1-2: Service skeleton + MinIO client**
- [ ] Xoa `services/file/main.go` cu, tao `services/file/cmd/server/main.go`
- [ ] `internal/config/config.go`: doc + validate STORAGE_*, DB_URL, PORT
- [ ] `internal/store/db.go`: pgx pool, ping health
- [ ] `internal/storage/client.go`: interface Storage + Config + factory New()
- [ ] `internal/storage/minio.go`: MinIO implementation day du:
  - `PresignPut` (15 phut TTL mac dinh)
  - `HeadObject` (verify sau upload)
  - `PublicURL` (path-style: `{endpoint}/{bucket}/{key}`)
  - `DeleteObject`
- [ ] `internal/storage/s3.go`: AWS S3 / Cloudflare R2 implementation (same SDK,
  khac endpoint + path style config)
- [ ] `internal/http/middleware/auth.go`, `request_id.go`, `logging.go`
- [ ] `internal/http/router.go`: wiring + CORS
- [ ] Chay duoc: `GET /healthz` → 200, `GET /healthz/storage` → ping MinIO

**Ngay 3-4: File handler + service logic**
- [ ] `internal/model/file.go`: UploadedFile, PresignRequest, PresignResponse,
  ConfirmResponse, FileListResponse, QuotaInfo
- [ ] `internal/repository/file_repo.go`:
  - `CreateFile`: INSERT status='pending'
  - `GetFile`: SELECT by id
  - `MarkActive`: UPDATE status='active', confirmed_at=NOW(), public_url=?
  - `SoftDelete`: UPDATE status='deleted', deleted_at=NOW()
  - `ListByUser`: SELECT WHERE user_id + status != 'deleted', phan trang
  - `CountActiveByUser`: SELECT COUNT + SUM(size_bytes) WHERE status='active'
  - `ListExpiredPending`: SELECT WHERE status='pending' AND created_at < NOW()-15min
- [ ] `internal/service/file_svc.go`:
  - `Presign`: validate upload_type, MIME, size, quota → sinh key → presign S3 → INSERT DB
  - `Confirm`: GetFile → kiem tra owner → HeadObject → MarkActive → tra URL
  - `GetFile`: GetFile → kiem tra owner → tra metadata
  - `Delete`: GetFile → kiem tra owner → SoftDelete (KHONG xoa S3 ngay)
  - `List`: ListByUser + CountActiveByUser → tra items + quota
  - `sanitizeFilename`: lowercase, strip ky tu nguy hiem
  - `buildStorageKey`: `uploads/{user_id}/{type}/{file_id}/{filename}`
- [ ] `internal/http/handler/file.go`: tat ca 5 endpoint
- [ ] Test: `TestPresign_ValidAvatar`, `TestPresign_InvalidMIME`,
  `TestPresign_FileTooLarge`, `TestConfirm_Success`,
  `TestConfirm_NotUploaded`, `TestQuota_Exceeded`

---

### Dev 2: Infrastructure + Docker + MinIO Setup (4 ngay)

**Ngay 1: Docker Compose + MinIO**
- [ ] Them `minio` service vao `docker-compose.yml`:
  ```yaml
  minio:
    image: minio/minio:latest
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: "${MINIO_ROOT_USER:-minioadmin}"
      MINIO_ROOT_PASSWORD: "${MINIO_ROOT_PASSWORD:-minioadmin}"
    ports:
      - "9000:9000"
      - "9001:9001"
    volumes:
      - minio-data:/data
    healthcheck:
      test: ["CMD", "mc", "ready", "local"]
      interval: 10s
      timeout: 5s
      retries: 5
    restart: on-failure
  ```
- [ ] Them `minio-init` job (chay 1 lan):
  ```yaml
  minio-init:
    image: minio/mc:latest
    depends_on:
      minio:
        condition: service_healthy
    entrypoint: >
      /bin/sh -c "
        mc alias set local http://minio:9000 minioadmin minioadmin &&
        mc mb --ignore-existing local/hquizlet &&
        mc anonymous set download local/hquizlet
      "
    restart: "no"
  ```
- [ ] Them volume `minio-data` vao danh sach volumes cuoi file
- [ ] Them `file` service vao `docker-compose.yml`:
  ```yaml
  file:
    build:
      context: ../..
      dockerfile: infra/docker/go-service.Dockerfile
      args:
        SERVICE: file
    environment:
      PORT: "8086"
      DATABASE_URL: "${DATABASE_URL:-...}"
      STORAGE_PROVIDER: "${STORAGE_PROVIDER:-minio}"
      STORAGE_ENDPOINT: "${STORAGE_ENDPOINT:-http://minio:9000}"
      STORAGE_BUCKET: "${STORAGE_BUCKET:-hquizlet}"
      STORAGE_ACCESS_KEY: "${STORAGE_ACCESS_KEY:-minioadmin}"
      STORAGE_SECRET_KEY: "${STORAGE_SECRET_KEY:-minioadmin}"
      STORAGE_PUBLIC_BASE_URL: "${STORAGE_PUBLIC_BASE_URL:-http://localhost:9000/hquizlet}"
      STORAGE_REGION: "${STORAGE_REGION:-us-east-1}"
      STORAGE_PATH_STYLE: "${STORAGE_PATH_STYLE:-true}"
      AUTH_SERVICE_URL: "${AUTH_SERVICE_URL:-http://auth:8081}"
    ports:
      - "${FILE_PORT:-8086}:8086"
    depends_on:
      postgres:
        condition: service_healthy
      minio-init:
        condition: service_completed_successfully
    restart: on-failure
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8086/healthz"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 15s
  ```

**Ngay 2: Gateway routes**
- [ ] Them `FILE_SERVICE_URL` vao gateway config:
  `FILE_SERVICE_URL: "${FILE_SERVICE_URL:-http://file:8086}"`
- [ ] Gateway routes (authenticatedProxy pattern):
  - `POST /v1/files/presign` → file (auth required)
  - `POST /v1/files/{id}/confirm` → file (auth required)
  - `GET /v1/files/{id}` → file (auth required)
  - `DELETE /v1/files/{id}` → file (auth required)
  - `GET /v1/files` → file (auth required)
- [ ] Cap nhat gateway `depends_on` them `file: condition: service_healthy`
- [ ] Cap nhat `GET /healthz/services` tra ve `file` status
- [ ] Cap nhat `FILE_SERVICE_URL` trong env cua gateway

**Ngay 3: OpenAPI 1.7.0 + .env.example**
- [ ] Tang OpenAPI version len `1.7.0` trong `packages/api-contracts/openapi.yaml`
- [ ] Them tag `files` vao danh sach tags
- [ ] Them schemas:
  - `PresignUploadRequest`, `PresignUploadResponse`
  - `ConfirmUploadResponse`, `FileMetadata`, `FileListResponse`
  - `QuotaInfo`, `FileErrorCode`
- [ ] Them tat ca 5 endpoints voi examples day du
- [ ] Them STORAGE_* vao `.env.example`:
  ```env
  # Storage (MinIO dev / S3 prod / Cloudflare R2)
  STORAGE_PROVIDER=minio
  STORAGE_ENDPOINT=http://localhost:9000
  STORAGE_BUCKET=hquizlet
  STORAGE_ACCESS_KEY=minioadmin
  STORAGE_SECRET_KEY=minioadmin
  STORAGE_PUBLIC_BASE_URL=http://localhost:9000/hquizlet
  STORAGE_REGION=us-east-1
  STORAGE_PATH_STYLE=true
  # FILE_PORT=8086
  # MINIO_ROOT_USER=minioadmin
  # MINIO_ROOT_PASSWORD=minioadmin
  ```
- [ ] `redocly lint` pass

**Ngay 4: E2E script + Makefile**
- [ ] `infra/scripts/phase9-e2e.sh`: script E2E day du (xem Section 12)
- [ ] Them vao `Makefile`:
  ```makefile
  test-phase9: test-go test-frontend lint-openapi
  	@echo "Phase 9 gate OK"

  e2e-phase9:
  	bash infra/scripts/phase9-e2e.sh
  ```
- [ ] Them `ci-gate` cap nhat chay ca file service tests

---

### Dev 3: Study Service Integration (flashcard image + thumbnail) (4 ngay)

**Ngay 1-2: Study service migration + API update**
- [ ] Them migration trong `services/study/migrations/`:
  ```sql
  -- 00X_add_image_columns.sql
  ALTER TABLE "flashcards" ADD COLUMN IF NOT EXISTS "image_url" text;
  ALTER TABLE "study_sets" ADD COLUMN IF NOT EXISTS "thumbnail_url" text;
  ```
- [ ] Cap nhat model `Flashcard` them `ImageURL *string`
- [ ] Cap nhat model `StudySet` them `ThumbnailURL *string`
- [ ] Cap nhat `PUT /v1/flashcards/{id}` handler: cho phep set `image_url`
  (validate la URL hop le hoac null)
- [ ] Cap nhat `PUT /v1/study-sets/{id}` handler: cho phep set `thumbnail_url`
- [ ] Cap nhat `GET /v1/study-sets/{id}` response: tra ve `thumbnail_url`
- [ ] Cap nhat `GET /v1/study-sets/{id}/flashcards` response: tra ve `image_url`
  trong moi flashcard
- [ ] Cap nhat OpenAPI examples: them `thumbnail_url`, `image_url` vao
  StudySet va Flashcard schemas
- [ ] Test: `TestFlashcard_SetImageURL`, `TestStudySet_SetThumbnailURL`,
  `TestFlashcard_ClearImageURL` (set null)

**Ngay 3-4: Repository + URL validation**
- [ ] `repository/flashcard_repo.go`: cap nhat `Update` query them `image_url`
- [ ] `repository/study_set_repo.go`: cap nhat `Update` query them `thumbnail_url`
- [ ] Service layer: validate URL format (phai la `http://` hoac `https://`,
  max 2000 chars, hoac null de xoa)
- [ ] Them golden test cho flashcard voi image: phat sinh quiz question
  cho flashcard co anh (Rust crate `quiz-core` kiem tra `image_url` truyen qua
  neu co, khong thay doi logic chon cau hoi)
- [ ] Test integration: tao study set → upload thumbnail → confirm → cap nhat
  study set thumbnail_url → GET study set → verify thumbnail_url hien dung

---

### Dev 4: Auth Service Integration (avatar) (4 ngay)

**Ngay 1-2: Auth service migration + profile API**
- [ ] Kiem tra bang `users` da co column `image` chua (Phase 2):
  - Neu chua: them migration `ALTER TABLE "user" ADD COLUMN IF NOT EXISTS "image" text;`
  - Neu roi: bo qua, chi cap nhat handler
- [ ] Cap nhat model `User` them `Image *string json:"image"`
- [ ] Them handler `PUT /v1/auth/profile`:
  ```
  Body: { "name": "string (optional)", "image_url": "string (optional)" }
  Response: { user object day du }
  ```
- [ ] Handler chi cap nhat cac field duoc truyen (partial update):
  - `name`: validate khong trong, max 100 ky tu
  - `image`: validate URL format hoac null
- [ ] Them route trong auth service router va gateway
- [ ] Cap nhat `GET /v1/auth/me` tra ve `image` field day du
- [ ] Test: `TestUpdateProfile_Name`, `TestUpdateProfile_Image`,
  `TestUpdateProfile_ClearImage`, `TestMe_ReturnsImage`

**Ngay 3-4: Repository + OpenAPI update**
- [ ] `repository/user_repo.go`: them method `UpdateProfile(userID, name, image *string)`
  dung partial update (chi UPDATE cac field khong nil)
- [ ] Service layer: `UpdateProfile` validate → call repo
- [ ] Cap nhat OpenAPI: them `PUT /v1/auth/profile` endpoint + `image` field
  vao User schema
- [ ] Cap nhat gateway routes: them `PUT /v1/auth/profile` → auth (auth required)
- [ ] Test security: User A khong the update profile User B (gateway chi forward
  X-User-Id tu session cua requester)
- [ ] Integration test: register → login → upload avatar → confirm → update
  profile image_url → GET /v1/auth/me → verify image hien dung

---

### Dev 5: Frontend Upload Components (4 ngay)

**Ngay 1: FileUpload base component + API client**

- [ ] `apps/web/src/lib/api/files.ts`: API client methods:
  ```typescript
  export async function presignUpload(req: PresignRequest): Promise<PresignResponse>
  export async function confirmUpload(fileId: string): Promise<ConfirmResponse>
  export async function deleteFile(fileId: string): Promise<void>
  export async function listFiles(): Promise<FileListResponse>
  // Upload truc tiep len MinIO: fetch PUT voi Content-Type header
  export async function uploadToStorage(uploadUrl: string, file: File): Promise<void>
  ```
- [ ] Types: `PresignRequest`, `PresignResponse`, `ConfirmResponse`,
  `FileMetadata`, `FileListResponse`, `QuotaInfo`
- [ ] `apps/web/src/components/upload/FileUpload.tsx`: component tai su dung:
  - Props: `uploadType`, `accept` (MIME), `maxSizeMB`, `onSuccess(url, fileId)`, `onError`
  - State: idle → selecting → uploading → confirming → done | error
  - Flow: chon file → validate client-side → presign → PUT MinIO → confirm → callback
  - Progress bar trong khi upload (XMLHttpRequest.onprogress)
  - Hien thi loi ro rang tung buoc
  - Khong dung form tag (dung onClick + input ref)

**Ngay 2: AvatarUpload component + Profile page**
- [ ] `apps/web/src/components/upload/AvatarUpload.tsx`:
  - Hien thi avatar hien tai (tu `GET /v1/auth/me`) hoac placeholder
  - Nut "Doi anh dai dien" → mo FileUpload
  - Sau confirm → goi `PUT /v1/auth/profile` voi `image_url`
  - Hien thi avatar moi ngay lap tuc (optimistic update)
  - Accept: `image/jpeg,image/png,image/webp`, max 5MB
- [ ] `apps/web/src/app/profile/page.tsx`:
  - Thong tin user: avatar, name, email
  - AvatarUpload tich hop
  - Form doi ten (cap nhat `PUT /v1/auth/profile`)
  - Nut Save
- [ ] Header component: hien thi avatar tu session (update sau khi doi)
- [ ] Test: `TestAvatarUpload_Success`, `TestAvatarUpload_FileTooLarge`,
  `TestAvatarUpload_InvalidType`

**Ngay 3: FlashcardImageUpload + StudySetThumbnailUpload**
- [ ] `apps/web/src/components/upload/FlashcardImageUpload.tsx`:
  - Hien thi anh hien tai (neu co) hoac nut "Them anh"
  - Upload → confirm → goi `PUT /v1/flashcards/{id}` voi `image_url`
  - Nut xoa anh (set `image_url: null`)
  - Preview anh truoc khi upload
  - Accept: `image/jpeg,image/png,image/webp,image/gif`, max 10MB
- [ ] Tich hop vao `FlashcardEditor` component tren Create/Edit study set page
- [ ] `apps/web/src/components/upload/StudySetThumbnailUpload.tsx`:
  - Hien thi thumbnail hien tai hoac placeholder
  - Upload → confirm → goi `PUT /v1/study-sets/{id}` voi `thumbnail_url`
  - Nut xoa thumbnail
  - Accept: `image/jpeg,image/png,image/webp`, max 5MB
- [ ] Tich hop vao `StudySetEditor` component
- [ ] Hien thi thumbnail tren Study Set list va detail page
- [ ] Test: `TestFlashcardImageUpload_Success`, `TestStudySetThumbnail_Success`,
  `TestFlashcardImageUpload_ClearImage`

**Ngay 4: File Manager page + Error handling**
- [ ] `apps/web/src/app/files/page.tsx` (optionnal, nice-to-have):
  - Grid cac files da upload (avatar, flashcard images, thumbnails)
  - Quota usage bar: "12/100 files, 2.1MB/500MB"
  - Nut xoa file (voi confirm dialog)
  - Empty state
- [ ] Error handling toan dien tren cac upload components:
  - Network error → retry button
  - MinIO timeout → clear pending, bao user thu lai
  - Quota exceeded → "Ban da dung het quota (100 files). Xoa bot de tiep tuc."
  - MIME/size error → hien thi thong bao cu the
- [ ] Loading states cho tat ca trang co anh (skeleton, blur-up)
- [ ] Test: `TestFileManager_QuotaDisplay`, `TestFileUpload_NetworkError_RetryButton`,
  `TestFileUpload_QuotaExceeded`

---

## 11. Test Plan

### 11.1 Unit Tests (backend — file service)

| File | Test |
|------|------|
| `storage/minio_test.go` | PresignPut URL format, HeadObject parse, PublicURL format |
| `service/file_svc_test.go` | ValidateMIME: accepted/rejected, ValidateSize: limits, sanitizeFilename: strip chars |
| `service/file_svc_test.go` | buildStorageKey: format dung, khong co path traversal |
| `service/file_svc_test.go` | Quota check: duoi, bung, vuot gioi han |
| `repository/file_repo_test.go` | CreateFile, GetFile, MarkActive, SoftDelete, CountActiveByUser |

### 11.2 Unit Tests (backend — auth + study service)

| File | Test |
|------|------|
| `auth: handler/profile_test.go` | UpdateProfile name, UpdateProfile image, partial update, clear image |
| `study: handler/flashcard_test.go` | SetImageURL valid, invalid URL, null |
| `study: handler/study_set_test.go` | SetThumbnailURL valid, null |

### 11.3 Integration Tests

- `TestPresignAndConfirm_FullFlow`: presign → PUT MinIO (real container) → confirm → GET metadata
- `TestQuota_FullFlow`: upload 100 files → 101st → 429
- `TestSoftDelete`: upload → confirm → delete → GET → 404
- `TestAvatarFlow`: presign avatar → confirm → update profile → GET me → image hien dung
- `TestFlashcardImageFlow`: presign → confirm → PUT flashcard image_url → GET flashcard
- `TestExpiredPending`: tao file pending → doi 15 phut (mock time) → cleanup → file deleted

### 11.4 Security Tests

- Presign voi `Content-Type: application/x-executable` → 400 `invalid_content_type`
- Presign `file_size` = 100MB → 400 `file_too_large`
- Confirm file cua nguoi khac → 403 `not_owner`
- GET file cua nguoi khac → 403 `not_owner`
- DELETE file cua nguoi khac → 403 `not_owner`
- `filename: "../../etc/passwd"` → sanitized → khong co path traversal trong key
- `filename: "'; DROP TABLE uploaded_file; --"` → sanitized thanh cong
- Upload khong co token → 401
- Presign dung upload_url cua nguoi khac → MinIO tra 403 (presigned URL bind voi credentials)

### 11.5 Frontend Tests

- `FileUpload`: flow thanh cong, network error retry, MIME invalid, size invalid
- `AvatarUpload`: render avatar hien tai, upload moi, optimistic update
- `FlashcardImageUpload`: render co anh, render khong anh, upload, xoa
- `StudySetThumbnailUpload`: upload, xoa, hien thumbnail tren list
- `FileManager`: quota display, xoa file voi confirm

### 11.6 E2E (Docker fresh-volume)

Xem Section 12.

---

## 12. E2E Script

```bash
#!/bin/bash
# infra/scripts/phase9-e2e.sh
set -euo pipefail

echo "=== Phase 9 E2E: File Upload ==="
echo "Rebuilding stack on fresh volumes..."

docker compose -f infra/docker/docker-compose.yml down -v
docker compose -f infra/docker/docker-compose.yml up --build -d
echo "Waiting for services to be healthy..."
sleep 25

GW="http://localhost:8080"

# --- Health check ---
echo "[1] Health check"
curl -sf "$GW/healthz/services" | python3 -c "
import sys, json
d = json.load(sys.stdin)
assert d['file'] == 'ok', f'file service not ok: {d}'
assert d['payment'] == 'ok', f'payment service not ok: {d}'
print('  healthz OK:', list(d.keys()))
"

# --- Register + login ---
echo "[2] Auth"
REG=$(curl -sf -X POST "$GW/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"name":"FileTest","email":"filetest@e2e.local","password":"Password123!"}')
TOKEN=$(echo "$REG" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
USER_ID=$(echo "$REG" | python3 -c "import sys,json; print(json.load(sys.stdin)['user']['id'])")
AUTH="-H 'Authorization: Bearer $TOKEN'"
echo "  User registered: $USER_ID"

# --- Presign avatar ---
echo "[3] Presign avatar"
PRESIGN=$(curl -sf -X POST "$GW/v1/files/presign" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"upload_type":"avatar","filename":"test.jpg","content_type":"image/jpeg","file_size":1024}')
FILE_ID=$(echo "$PRESIGN" | python3 -c "import sys,json; print(json.load(sys.stdin)['file_id'])")
UPLOAD_URL=$(echo "$PRESIGN" | python3 -c "import sys,json; print(json.load(sys.stdin)['upload_url'])")
echo "  file_id: $FILE_ID"

# --- Upload fake image truc tiep len MinIO ---
echo "[4] Upload to MinIO"
# Tao file JPEG gia (header magic bytes JPEG: FF D8 FF)
printf '\xff\xd8\xff\xe0\x00\x10JFIF\x00' > /tmp/test-avatar.jpg
curl -sf -X PUT "$UPLOAD_URL" \
  -H "Content-Type: image/jpeg" \
  --data-binary @/tmp/test-avatar.jpg
echo "  Upload OK"

# --- Confirm ---
echo "[5] Confirm upload"
CONFIRM=$(curl -sf -X POST "$GW/v1/files/$FILE_ID/confirm" \
  -H "Authorization: Bearer $TOKEN")
URL=$(echo "$CONFIRM" | python3 -c "import sys,json; print(json.load(sys.stdin)['url'])")
echo "  Confirmed URL: $URL"

# --- Update profile avatar ---
echo "[6] Update profile image"
ME=$(curl -sf -X PUT "$GW/v1/auth/profile" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"image_url\":\"$URL\"}")
IMG=$(echo "$ME" | python3 -c "import sys,json; print(json.load(sys.stdin)['image'])")
echo "  Profile image: $IMG"

# --- GET /v1/auth/me verify ---
echo "[7] Verify GET /me"
ME2=$(curl -sf "$GW/v1/auth/me" -H "Authorization: Bearer $TOKEN")
echo "$ME2" | python3 -c "
import sys,json
d=json.load(sys.stdin)
assert d.get('image'), 'image missing from /me response'
print('  image OK:', d['image'])
"

# --- GET /v1/files ---
echo "[8] List files + quota"
FILES=$(curl -sf "$GW/v1/files" -H "Authorization: Bearer $TOKEN")
echo "$FILES" | python3 -c "
import sys,json
d=json.load(sys.stdin)
assert d['total'] >= 1, 'expected at least 1 file'
assert d['quota']['active_files'] >= 1
print('  total files:', d['total'], '| quota:', d['quota'])
"

# --- Security: confirm nguoi khac ---
echo "[9] Security: confirm another user's file"
REG2=$(curl -sf -X POST "$GW/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"name":"Attacker","email":"attacker@e2e.local","password":"Password123!"}')
TOKEN2=$(echo "$REG2" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -X POST "$GW/v1/files/$FILE_ID/confirm" \
  -H "Authorization: Bearer $TOKEN2")
[ "$STATUS" == "403" ] && echo "  403 OK (not_owner)" || (echo "  FAIL: expected 403, got $STATUS" && exit 1)

# --- Invalid MIME ---
echo "[10] Security: invalid MIME type"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -X POST "$GW/v1/files/presign" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"upload_type":"avatar","filename":"evil.exe","content_type":"application/x-executable","file_size":512}')
[ "$STATUS" == "400" ] && echo "  400 OK (invalid_content_type)" || (echo "  FAIL: expected 400, got $STATUS" && exit 1)

# --- Soft delete ---
echo "[11] Soft delete"
curl -sf -X DELETE "$GW/v1/files/$FILE_ID" -H "Authorization: Bearer $TOKEN" -o /dev/null
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  "$GW/v1/files/$FILE_ID" -H "Authorization: Bearer $TOKEN")
[ "$STATUS" == "404" ] && echo "  404 OK (deleted)" || (echo "  FAIL: expected 404, got $STATUS" && exit 1)

echo ""
echo "==================================================="
echo "  Phase 9 E2E: ALL CHECKS PASSED"
echo "==================================================="
```

---

## 13. Environment Variables

Them vao `.env.example` va `infra/env.template`:

```env
# ── File Service (services/file) ────────────────────────────────────────────
FILE_PORT=8086

# Storage provider: minio (dev) | s3 (AWS) | r2 (Cloudflare)
STORAGE_PROVIDER=minio

# MinIO / S3 / R2 endpoint
# MinIO local: http://localhost:9000
# AWS S3: https://s3.amazonaws.com (hoac bo trong, dung default)
# R2: https://<account_id>.r2.cloudflarestorage.com
STORAGE_ENDPOINT=http://localhost:9000

# Bucket name (tao san trong MinIO init job)
STORAGE_BUCKET=hquizlet

# Access credentials
STORAGE_ACCESS_KEY=minioadmin
STORAGE_SECRET_KEY=minioadmin

# URL public de generate links (khong ket thuc bang /)
# MinIO local: http://localhost:9000/hquizlet
# S3: https://<bucket>.s3.<region>.amazonaws.com
# R2: https://pub-<hash>.r2.dev
STORAGE_PUBLIC_BASE_URL=http://localhost:9000/hquizlet

# AWS region (MinIO dung us-east-1, R2 dung auto)
STORAGE_REGION=us-east-1

# Path-style URL (true cho MinIO, false cho AWS S3)
STORAGE_PATH_STYLE=true

# MinIO root credentials (chi cho docker-compose dev)
MINIO_ROOT_USER=minioadmin
MINIO_ROOT_PASSWORD=minioadmin
```

---

## 14. Huong Dan Cai Hinh Tung Provider

### 14.1 MinIO (dev local — mac dinh)

Khong can cau hinh them. `docker compose up` tu dong:
1. Khoi dong MinIO container.
2. `minio-init` job tao bucket `hquizlet` voi quyền download public.
3. File service ket noi vao `http://minio:9000`.
4. URL public dang `http://localhost:9000/hquizlet/uploads/...`.

Truy cap MinIO Console: http://localhost:9001 (user: minioadmin / minioadmin).

### 14.2 AWS S3

```env
STORAGE_PROVIDER=s3
STORAGE_ENDPOINT=            # de trong, dung SDK default
STORAGE_BUCKET=hquizlet-prod
STORAGE_ACCESS_KEY=AKIAIOSFODNN7EXAMPLE
STORAGE_SECRET_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
STORAGE_PUBLIC_BASE_URL=https://hquizlet-prod.s3.ap-southeast-1.amazonaws.com
STORAGE_REGION=ap-southeast-1
STORAGE_PATH_STYLE=false
```

### 14.3 Cloudflare R2

```env
STORAGE_PROVIDER=r2
STORAGE_ENDPOINT=https://<account_id>.r2.cloudflarestorage.com
STORAGE_BUCKET=hquizlet
STORAGE_ACCESS_KEY=<R2 Access Key ID>
STORAGE_SECRET_KEY=<R2 Secret Access Key>
STORAGE_PUBLIC_BASE_URL=https://pub-<hash>.r2.dev
STORAGE_REGION=auto
STORAGE_PATH_STYLE=false
```

---

## 15. Definition of Done

Phase 9 chi duoc danh dau GO khi tat ca dieu kien sau deu dat:

### Backend — File Service
- [ ] `GET /healthz/services` tra ve `file: ok`
- [ ] Presign → upload → confirm flow chay dung tren MinIO that
- [ ] MIME/size validation tu choi dung loai
- [ ] Quota check hoat dong (100 files, 500 MB)
- [ ] Soft delete: file khong tra ve sau khi xoa
- [ ] Security: owner check enforce cho GET, DELETE, confirm
- [ ] All unit tests pass
- [ ] All integration tests pass
- [ ] All security tests pass

### Backend — Auth + Study Service
- [ ] `PUT /v1/auth/profile` cap nhat name + image thanh cong
- [ ] `GET /v1/auth/me` tra ve `image` field
- [ ] `PUT /v1/flashcards/{id}` cap nhat `image_url`
- [ ] `PUT /v1/study-sets/{id}` cap nhat `thumbnail_url`
- [ ] GET study set tra ve `thumbnail_url`, flashcards tra ve `image_url`
- [ ] Migration idempotent (ALTER COLUMN IF NOT EXISTS)

### Frontend
- [ ] FileUpload base component: full flow + error states
- [ ] AvatarUpload: upload → profile update → header update
- [ ] FlashcardImageUpload: upload + xoa tich hop vao editor
- [ ] StudySetThumbnailUpload: upload + hien thi tren list/detail
- [ ] All frontend tests pass

### Infrastructure
- [ ] `docker compose up --build` chay sach tren fresh volume
- [ ] Phase 9 E2E script xanh (gan commit SHA + output vao release gate)
- [ ] OpenAPI 1.7.0 validated (redocly lint pass)
- [ ] STORAGE_* documented trong `.env.example`
- [ ] MinIO tao bucket tu dong qua init job (khong can can thiep thu cong)
- [ ] Dockerfile file service: multi-stage, non-root

### Documentation
- [ ] `docs/storage/minio-setup.md`: huong dan dev local + S3 + R2
- [ ] `docs/storage/upload-flow.md`: presign flow + security notes
- [ ] Phase 9 release gate report: `docs/phase/phase-9-release-gate-YYYY-MM-DD.md`
- [ ] README.md cap nhat: them File service + MinIO vao service list

---

## 16. Rui Ro Va Giam Thieu

| Rui ro | Giam thieu |
|--------|------------|
| User upload xong nhung quen goi confirm → file rac pending | Cleanup job xoa pending > 15 phut; cron hoac check khi presign |
| Presigned URL bi leak → nguoi khac upload vao slot cua minh | URL gan voi `Content-Type` + TTL 15 phut; confirm check owner_id tu DB khong phu thuoc URL |
| Path traversal trong filename | `buildStorageKey` dung UUID lam prefix, sanitize filename strip tat ca ky tu nguy hiem |
| MinIO chua san, file service start truoc | `depends_on: minio-init: condition: service_completed_successfully` dam bao thu tu |
| S3 presign URL khac host trong Docker → client goi sai endpoint | `STORAGE_PUBLIC_BASE_URL` set rieng cho presign expose ra ngoai vs internal; xem docs/storage/ |
| User spam presign → tao nhieu file pending tiep thu quota | Quota check COUNT pending + active; rate limit 20 presign/5min/user (tuy chon them) |
| File service sap → khong upload duoc anh, nhung app van chay binh thuong | File URL nullable tren moi entity; frontend fallback avatar placeholder |
| Concurrent confirm cung file_id | `UPDATE WHERE status='pending'` atomic; second confirm tra 409 `already_confirmed` |

---

## 17. Timeline Tong Hop (4 Ngay, 5 Dev Song Song)

```
Ngay 1: [Dev1] file service skeleton + MinIO client (PresignPut, HeadObject, PublicURL)
         [Dev2] docker-compose MinIO + minio-init + file service entry
         [Dev3] study service migration + flashcard/study_set column + API update
         [Dev4] auth service migration + PUT /v1/auth/profile + GET /me image
         [Dev5] API client methods + FileUpload base component (presign → upload → confirm)

Ngay 2: [Dev1] file_svc (validate, quota, key builder) + repository + handler skeleton
         [Dev2] gateway routes + healthz/services update + .env.example
         [Dev3] repository update (image_url, thumbnail_url) + URL validation service
         [Dev4] profile repository partial update + OpenAPI auth update
         [Dev5] AvatarUpload component + Profile page

Ngay 3: [Dev1] unit tests (storage, service, repo) + Dockerfile non-root
         [Dev2] OpenAPI 1.7.0 + redocly lint + Makefile update
         [Dev3] Study integration test + golden test flashcard image
         [Dev4] auth integration test + security test (cross-user)
         [Dev5] FlashcardImageUpload + StudySetThumbnailUpload tich hop editor

Ngay 4: [All]  security tests + bug fix + E2E script chay tren fresh volume
         [All]  docker fresh-volume E2E evidence thu thap
         [All]  code review + merge
         [All]  cap nhat docs/storage/
         [All]  phase 9 release gate report
         [All]  danh dau Phase 9 GO
```
