# ADR 001: Bulk import OOM fix (streaming + MinIO buffer)

## Status

Accepted — 2026-03-27

## Context

Async flashcard/quiz import from Excel duplicated memory several times:

1. `ParseMultipartForm(32MB)` buffered the upload in the handler.
2. `io.ReadAll` copied the file again in `ImportJobService`.
3. `excelize.GetRows` materialized the full sheet.
4. Slices for parsed rows and `BulkFlashcardItem` duplicated data again.
5. `ListByStudySet` loaded every existing card only to compute the next `position`.

On a 2 GiB host (e.g. `nat-mini`), two or three concurrent ~3200-row imports could OOM the study service.

## Decision

Three-stage **streaming pipeline**:

| Stage | Component | Behavior |
| --- | --- | --- |
| 1 | HTTP handler | `MultipartReader()` streams the file part; no `ParseMultipartForm`. |
| 2 | Import blob storage | Stream upload to MinIO (`hquizlet-imports`) via `PutObject`; worker reads with `GetObject`. Local disk spool when `MINIO_ENDPOINT` is unset. |
| 3 | Worker | `excelize.Rows()` iterator; insert flashcards in chunks of 200 rows; `NextFlashcardPosition()` instead of loading all cards. |

Schema migration **023** adds `import_jobs.minio_key` and `file_size_bytes`.

MinIO lifecycle: expire objects under `imports/` after 1 day (init container).

## Consequences

- Peak RAM per import drops from ~300MB to roughly excelize DOM + one chunk (~5–50MB depending on file).
- Disk on the data node holds short-lived import blobs; lifecycle prevents fill.
- Sync import endpoints still use the old path (lower priority; quiz cap 200 rows).
- Quiz async import streams to blob storage but still uses `GetRows` (≤200 rows).

## Configuration

| Variable | Default (dev compose) |
| --- | --- |
| `MINIO_ENDPOINT` | `http://minio:9000` |
| `MINIO_ACCESS_KEY` / `MINIO_SECRET_KEY` | same as MinIO root |
| `MINIO_IMPORT_BUCKET` | `hquizlet-imports` |
| `IMPORT_SPOOL_DIR` | `/tmp/hquizlet-imports` when MinIO disabled |
