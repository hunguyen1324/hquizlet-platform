# Private video uploads

The browser uploads MP4/WebM in sequential 8 MiB requests. The default total
limit is 1 GiB (`VIDEO_MAX_UPLOAD_MB=1024`). Each request stays below Cloudflare's
request-body limit; increasing the total does not increase individual requests.
Nginx's existing 50 MiB default also accommodates these requests.

## Upload lifecycle

- `POST /api/v1/classes/{classId}/videos/uploads` creates a 24-hour session and
  returns `uploadId` and `chunkSize`. Metadata includes title, audience, viewerIds,
  contentType and size. Only the owner or a teacher may upload.
- `PUT .../uploads/{uploadId}?offset={bytes}` sends raw bytes. Every nonfinal part
  is exactly 8 MiB. The final part has the exact remaining length.
- `GET .../uploads/{uploadId}` returns the committed `received` offset. The client
  checks it after a lost response before retrying. Progress is durable in Postgres;
  each operation rechecks the authenticated uploader and current class permissions.
- `POST .../uploads/{uploadId}/complete` atomically publishes the source file and
  queues the existing encrypted HLS conversion. Repeating completion returns the
  same video ID. Files are still served only through the authorized playback API.
- `DELETE .../uploads/{uploadId}` removes an incomplete upload. The regular cleanup
  worker removes abandoned sessions and files after 24 hours. An abandoned browser
  tab does not automatically resume after reloading; its reservation expires.

Parts are written into a private `.upload` file over SFTP. A row lock serializes
requests, and a retry truncates uncommitted bytes to the durable offset. Completion
renames the file without copying the whole video through the App server. The App
buffers at most one part per active request, rather than an entire video.

The video server must support the OpenSSH `statvfs` extension. Admission reserves
space for active uploads and queued conversions: approximately three times the
input on storage and five times on the Linux App, with a 512 MiB safety margin.
HLS conversion runs one job at a time, checks temporary free space while encoding,
and has a 60-minute conversion timeout. Large or complex videos can take longer
than their upload; exceeding limits marks processing failed rather than ready.

## Deployment and rollback

Deploy the class service before frontend assets. The class migration runner creates
`class_video_uploads` automatically. Set `VIDEO_MAX_UPLOAD_MB=1024` on App and
recreate only `class`; the gateway and Nginx require no larger request limit.

An emergency local image can be selected using `CLASS_IMAGE`. Production's initial
rollout used `hquizlet-class:chunked-20261105-161558`, with the previous environment,
Compose file and image name recorded under
`/opt/hquizlet/releases/chunked-video-20261105-161558/`. Keep that directory private
because the environment backup contains secrets. Remove the `CLASS_IMAGE` override
when deploying a registry image that includes this implementation; otherwise the
local image remains pinned. The additive upload table can remain on rollback.

If rolling back the class service, roll back frontend assets too: the previous
frontend's single-request upload cannot accept large videos. Do not remove the
upload table or staged files until pending sessions have been inspected.

Logs include upload ID, total bytes and committed bytes for each chunk. A completed
upload is not yet playable until `processingState` becomes `ready`.
