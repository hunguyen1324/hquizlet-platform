# ADR-004: Authorization model (IAM-style)

## Status

Accepted (2026-10-13)

## Context

Platform roles were a single `users.role` string (`user` / `admin`). Class permissions lived in `class_members`. Checks were duplicated across services.

## Decision

- Shared Go library `packages/authz` implements **default deny**, **Deny over Allow**, managed roles, and ARN-style resources (`hq:type/id`).
- **Platform** assignments live in auth DB (`iam_assignments`, scope `platform`).
- **Class** assignments remain in `class_members`, mapped to `ClassOwner` / `ClassTeacher` / `ClassStudent` at evaluation time.
- Gateway injects signed `X-User-Roles`; legacy `X-User-Role` remains (`admin` if `PlatformAdmin`).
- Services call `authz.Can(principal, action, resource)` at boundaries.

## Consequences

- One policy matrix to extend; org scope can reuse the same evaluator later.
- Auth service owns IAM tables and `/v1/me/permissions`.

## Enforcement and compatibility

Verified identity headers carry a timestamped HMAC including all platform and organization roles. Services validate that proof or resolve the bearer through auth; client-supplied role headers cannot establish a principal. Batched class progress uses a separate HMAC bound to method, exact path/query and timestamp.

Managed-role restrictions exclude a principal's PlatformAdmin grant where the permission matrix grants administrative access. Student price restrictions exclude an explicit Teacher grant. The evaluator still applies every matching explicit Deny before Allow; there is no unconditional admin bypass in the evaluator. This permits Teacher/Student accounts to receive administrative grants without their original roles accidentally vetoing administrative work.

Legacy role fallback applies to accounts without managed platform assignments. An account whose assignments exist but expired does not recover admin rights through `users.role`. Revoking the last legacy admin assignment downgrades the legacy role in the same transaction as its audit entry. Disabling users revokes sessions in the same transaction as the status change and audit.

Phase 5 organization workflows remain deferred by the project owner. Existing organization types and role definitions are extension points, not an accepted organization dashboard/invitation implementation.
