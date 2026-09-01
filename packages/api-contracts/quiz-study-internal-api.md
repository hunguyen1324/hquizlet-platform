# Quiz → Study Internal API Contract

**Phase:** 4  
**Task:** P4-INT-01  
**Date:** 2026-09-01  
**Owner:** Dev 1 (Contract & Integration)

---

## 1. Overview

The Quiz service needs to fetch flashcards for a study set to generate quizzes (shuffle, questions, match pairs). It calls the Study service via an internal HTTP API. The Quiz service **never** touches the database directly.

## 2. Endpoint

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/internal/study-sets/{studySetId}/flashcards` | Get all flashcards for a study set with ownership check |

## 3. Authentication

The request carries the verified `X-User-ID` header (injected by Gateway after auth verification). The Study service uses this to verify ownership — the user must own the study set.

**No bearer token is needed** for internal calls between backend services. The `X-User-ID` header is the identity source.

## 4. Request

```
GET /internal/study-sets/{studySetId}/flashcards HTTP/1.1
Host: study:8082
X-User-ID: 42
X-Request-ID: 20260901120000.000000001
```

| Header | Required | Description |
|--------|----------|-------------|
| `X-User-ID` | Yes | Verified user ID from Gateway |
| `X-Request-ID` | No | Request tracing ID |

| Path Parameter | Type | Description |
|----------------|------|-------------|
| `studySetId` | int64 | Study set ID |

## 5. Response — Success (200)

```json
{
  "id": 101,
  "userId": 42,
  "title": "Japanese Vocabulary",
  "description": "Basic words",
  "flashcards": [
    {
      "id": 1001,
      "studySetId": 101,
      "term": "Thủ đô Nhật Bản",
      "definition": "Tokyo",
      "starred": false,
      "position": 0,
      "createdAt": "2026-09-01T10:00:00Z",
      "updatedAt": "2026-09-01T10:00:00Z"
    }
  ],
  "createdAt": "2026-09-01T10:00:00Z",
  "updatedAt": "2026-09-01T10:00:00Z"
}
```

This is the same shape as the public `GET /v1/study-sets/{id}` response, reusing the existing `StudySet` model with embedded `flashcards`.

## 6. Response — Errors

| Status | Code | When |
|--------|------|------|
| `403` | `FORBIDDEN` | User does not own the study set |
| `404` | `NOT_FOUND` | Study set does not exist |

Error envelope:
```json
{
  "code": "FORBIDDEN",
  "message": "study set not owned by caller",
  "requestId": "...",
  "details": {}
}
```

## 7. Implementation Notes

- **Study service** adds a new route `GET /internal/study-sets/{id}/flashcards` that calls the existing `GetWithCards(ctx, setID, userID)` service method. This reuses the same ownership check as the public endpoint.
- **Quiz service** adds an HTTP client (`internal/studyclient/`) with:
  - 5-second timeout
  - Context cancellation support
  - Typed errors (not raw `err.Error()` exposed to clients)
- The internal route is **not** exposed through the Gateway — it's only accessible on the service mesh (Docker network or localhost in dev).

## 8. Security

- `X-User-ID` is trusted only because it comes from the same Docker network. In production, mutual TLS or service mesh auth would validate the caller.
- The Gateway never routes external requests to `/internal/*` paths.
- The Quiz service must not forward user-supplied identity headers to Study.

## 9. Cross-reference

- Public API: `openapi.yaml` → `GET /v1/study-sets/{id}` (same response shape)
- Internal auth API: `GET /internal/auth/verify` (existing pattern)
- Phase 4 plan: `[P4-INT-01]`
