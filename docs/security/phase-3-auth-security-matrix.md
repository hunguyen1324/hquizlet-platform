# Phase 3 Auth Security Matrix

Owner: Dev1  
Scope: P3-AUTH-01 through P3-AUTH-05

## Canonical internal verify contract

`GET /internal/auth/verify` accepts only `Authorization: Bearer <token>` and returns a
verified identity containing `authenticated`, `userId`, `email`, `name`, `role`, and
`expiresAt`. It never returns the raw token or token hash.

A token is authorized only when its session exists, has not expired, has not been
revoked, and its user is not disabled. Logout and logout-all soft-revoke sessions so
authorization is lost immediately while the audit trail is retained.

## Required cases

| Case | Expected result | Automated evidence / owner |
| --- | --- | --- |
| Missing bearer token | 401 canonical envelope | Auth service test |
| Invalid bearer token | 401 canonical envelope | Auth service test |
| Expired session | 401 canonical envelope | Auth service test |
| Revoked session after logout | 401 canonical envelope | Auth service test |
| All sessions after logout-all | 401 canonical envelope | Auth service test |
| Disabled user | 401 canonical envelope | Auth service test |
| Valid session | Canonical identity with expiry | Auth service test |
| Client sends `X-User-ID` | Gateway removes it before injecting verified ID | Dev5 gateway test/review |
| User A targets User B progress | 403/404; no read or write | Dev2 + Dev5 E2E |
| Auth error | `{code,message,requestId,details}` | Auth HTTP test |

## Review boundary

Dev1 must review progress ownership queries from Dev2 and identity-header stripping from
Dev5. Study must never treat a client-originated identity header as authentication.
