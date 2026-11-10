# Production performance review — 10 November 2026

## Scope and evidence

Read-only SSH checks of App and Data at approximately 17:20 Asia/Bangkok.
No production configuration, containers, data, firewall or credentials were changed.
The code changes below are local and have not been deployed.
These are idle/light-load snapshots, not a concurrency benchmark or an availability certification.

## Live machines

| Machine | Logical CPUs | RAM | Available RAM | Swap used | Root disk | Available disk |
| --- | ---: | ---: | ---: | ---: | --- | ---: |
| App | 1 | 951 MiB | 435 MiB | 155 MiB | 15 GiB, 58% used | 5.8 GiB |
| Data | 2 | 1956 MiB | 1220 MiB | 91 MiB | 25 GiB, 39% used | 15 GiB |

Three-second vmstat samples showed no ongoing swap-in/out at that instant.
App 1/5/15-minute load averages were 1.59/0.52/0.32; Data 0.58/0.35/0.22.
This does not establish peak CPU or memory usage. All main API containers were healthy,
with zero recorded restarts. PostgreSQL reported max_connections=100, 16 active/open
backend connections at the snapshot (including idle/maintenance connections), database size 55 MB.

The four video shard servers were not separately audited with administrator access in this pass.
The 256 MiB RAM and disk figures in deployment/four-vps-video-plan.md are historical,
not current measurements. Shards store single segment copies, so loss of one node can
make part of a video unavailable; distributing files is not replication.

## Changes implemented

- Lazy-load editors, study detail, classes, Live Quiz, payments and secondary pages.
  Keep the app shell visible while a selected feature loads. Add an error boundary
  with an explicit reload action for failed page/chunk loads.
- Lazy-load the authenticated HLS player when a private video is opened, rather than
  on entry to the video library.
- Service-status polling changes from every 5 seconds to every 60 seconds after the
  previous request completes. Hidden tabs stop polling, requests do not overlap,
  cleanup aborts outstanding work, and late responses are ignored.
  This reduces the steady-state probe frequency by about 92% for visible idle tabs.
- Quiz generation aborts previous work when parameters change, on regeneration and
  on unmount. Cancelled requests cannot overwrite newer state.
- Bound pools in the actual Auth/Study/Class/File/Payment entrypoints to 10 open,
  3 idle connections, 5-minute idle timeout and 30-minute lifetime. Quiz already has
  a 25-open limit. One instance of each application pool therefore budgets at most
  75 connections, leaving headroom against the measured PostgreSQL limit of 100.
  This budget must be recalculated before adding replicas. Monitor pool wait time
  and ensure transactions do not wait recursively for another pooled connection.
- Emit Vite's manifest and enforce a startup bundle budget in every web build:
  at most 110 KiB gzip of statically reachable JS and 32 KiB gzip of CSS.
  Dynamic route chunks are excluded from startup totals, not hidden from reporting.

## Local before/after evidence

Vite production builds, same dependencies and working tree UI changes:

| Artifact | Before | After route splitting |
| --- | ---: | ---: |
| Main JavaScript entry | 1119.41 kB | approximately 263.6 kB |
| Main JavaScript gzip | 338.08 kB | approximately 82.7 kB |
| Startup CSS | 222.32 kB | approximately 92.4 kB |

The main JS entry is approximately 76% smaller. This is an artifact comparison;
it is not a measured 76% reduction in page load time. Dashboard chart code and HLS
remain separate on-demand downloads. The final build budget checks static imports
recursively using the manifest, so shared eager chunks cannot bypass the check.

Validation: frontend tests including hidden-tab, non-overlapping polling and late
response scenarios; production web build and budget check; Go tests across
Auth, Study, Class, File and Payment. Integration cases requiring external services
may skip when their test database configuration is absent.

## Operational issues to resolve before production sign-off

1. **File backup is unhealthy.** docker-backup-1 repeatedly reports a storage
   operation failure. The deployed version only emits generic diagnostics, so this
   does NOT prove the R2 quota is full. Read the existing state and use the current
   source's redacted error classification in a controlled rebuild to distinguish
   credentials, network, bucket, transfer limit and quota errors. Do not increase
   quota or remove backup versions speculatively. PostgreSQL-to-Neon backup is healthy
   in this snapshot, which does not prove a successful restore.
2. **MinIO initialization restart loop.** docker-minio-init-1 has 15,275 recorded
   restarts and a live restart policy of unless-stopped. Its logs confirm lifecycle
   configuration reached the maximum of 1000 rules. The initializer runs `mc ilm rule
   add` repeatedly. The repository's data Compose already says restart: no, so live
   configuration has drifted. Prepare a reviewed cleanup of duplicate rules after
   exporting the current lifecycle configuration; retain all distinct policies.
   Make initialization check for the existing imports/ expiration policy before
   adding another. Do not delete all lifecycle rules or recreate storage volumes.
3. **Quick Tunnel in the production path.** App runs docker-cloudflared-quick-1.
   Confirm the Worker upstream, then replace Quick Tunnel with a named Cloudflare
   Tunnel and stable origin hostname. Cloudflare documents Quick Tunnels as testing
   and development tools, with a 200 in-flight request limit. Do not infer that this
   means 200 users. Keep the current origin as rollback while verifying authentication,
   uploads, media ranges, HLS and Live Quiz SSE on the new route.
4. **Video processing competes with requests.** FFmpeg runs on the 1-CPU App host.
   Move conversion to a separate bounded worker/queue before scaling concurrent
   uploads; keep the API responsible for authorization and job status.
5. **Protected data must remain private.** Keep authenticated quiz responses,
   audio/media keys and user-specific content out of shared CDN caches. Existing
   fingerprinted static assets can retain immutable caching and HTML revalidation.
   Verify revocation and ticket expiry if segment delivery is redesigned.
6. **Large decks still need a separate learning-mode pagination change.** The list
   is paged, but FlashcardsMode requests generation with the full set count.
   Change this to bounded on-demand learning batches while preserving seed,
   progress, next/previous navigation and server delivery budgets. Non-owner
   content policy currently caps pages independently of the UI's requested size.

## Resource and architecture recommendation

Priority order, based on these measurements rather than a claimed user capacity:

- Keep Data at 2 CPU / 2 GiB for the first measured rollout; there is no evidence
  from this snapshot that a bigger database host is the first purchase needed.
- For regular production use, target App at 2 CPU / 4 GiB and 30–40 GiB SSD.
  This adds headroom for deployments and transient allocations; it does not replace
  load testing. If retaining 1 CPU / 1 GiB temporarily, serialize video conversion
  and monitor available RAM and swap activity.
- If video upload frequency grows, use a separate 2 CPU / 2–4 GiB worker with a
  disk budget based on source + temporary conversion + output. Limit concurrency
  to one conversion until measured; the existing NATS/job storage can be evaluated
  instead of introducing another broker.
- Retain Cloudflare-hosted static frontend. For larger video usage, compare a
  managed streaming service or private object storage plus authorized CDN delivery
  against maintaining four tiny single-copy SFTP shards. R2 presigned URLs use its
  S3 API hostname and are not automatically a custom-domain private CDN design.
- Do not add Kubernetes, Kafka, more database replicas or Redis nodes yet. First
  collect p95 API latency, pool waits, peak RAM, CPU, first video frame time, errors,
  backup freshness and restore success. Place monitoring on another host/service
  if a full Prometheus/Grafana stack would exhaust the small production machines.

Acceptance targets for a staging load test, not claims about current production:
LCP p75 <= 2.5s; INP p75 <= 200ms; metadata API p95 <= 500ms; error rate <1%;
no OOM or sustained swap churn; successful backup restoration; stable Live Quiz
connections while video processing runs. Test realistic authenticated traffic
using disposable fixtures on staging and increase concurrency gradually.

## References

- https://vite.dev/guide/build (lazy chunk deployments and HTML revalidation)
- https://go.dev/doc/database/manage-connections (pool limits and DB.Stats)
- https://developers.cloudflare.com/tunnel/get-started/quick-tunnels/ (production tunnel choice)
- https://developers.cloudflare.com/r2/api/s3/presigned-urls/ (private object delivery constraints)
