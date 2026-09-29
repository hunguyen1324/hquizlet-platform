# Quiz audio

Both layouts use the protected quiz player. The browser obtains an expiring,
audio-only ticket using its bearer token, then plays a native media URL with
HTTP Range support (206 / Content-Range / 416). It no longer downloads a full
Blob before playback. Browser buffering determines request sizes; this is not
HLS transcoding or fixed-duration segmentation.

The ticket lasts two hours, is checked against the current session and access
rights on every request, and is invalidated on study-service restart. Use the
audio reload button to renew it. The process-local signing key currently assumes
one study-service replica. Tickets are bearer capabilities and can be shared
until they expire; they are not DRM. Nginx access logs omit query strings.

Set `QUIZ_AUDIO_ALLOWED_HOSTS` to comma-separated external HTTPS audio hosts.
Private/reserved network addresses, nonstandard ports and redirects are rejected.
Configure `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY` and
`MINIO_IMPORT_BUCKET`. Keep that bucket private. Audio uses `quiz-audio/`, outside
the temporary `imports/` lifecycle rule. Do not expire that prefix.

A worker synchronizes existing and newly imported question audio every minute;
first playback also synchronizes on demand. Failures retry on the next pass.
PostgreSQL retains the audio bytes as a recovery copy and records `source_url`,
`object_key`, `backed_up_at`, content type and owning study set. MinIO stores a
second copy. Playback currently reads the DB copy; server-side DB reads are still
whole-file (maximum 30 MB), while the browser receives requested byte ranges.
Without MinIO configured, development falls back to DB-only storage.

Text selection, copy, drag and context-menu suppression are only casual-copy
deterrents. Server-side question paging and access checks protect bulk delivery;
screenshots, OCR and browser inspection cannot be prevented absolutely.
