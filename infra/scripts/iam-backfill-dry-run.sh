#!/bin/bash
# infra/scripts/iam-backfill-dry-run.sh
#
# Reports (and optionally applies) the legacy users.role -> iam_assignments backfill
# described in docs/dashboard-iam-plan.md §3.8 / IAM-04.
#
#   users.role = 'admin' -> PlatformAdmin (platform scope)
#   legacy user with no platform assignment -> Student (preserve Teacher)
#
# Default is a dry run: it only counts what is missing. Pass --apply to insert.
# The auth service migration also performs this backfill idempotently; use this script
# to inspect the planned delta before a deployment or to repair a drifted database.
#
# Connection: set DATABASE_URL to use a local psql client, otherwise the script falls back
# to `docker compose exec postgres`.
#
# Usage:
#   infra/scripts/iam-backfill-dry-run.sh            # dry run
#   infra/scripts/iam-backfill-dry-run.sh --apply    # apply the missing assignments
set -euo pipefail

APPLY="false"
if [ "${1:-}" == "--apply" ]; then
  APPLY="true"
fi

if [ -n "${DATABASE_URL:-}" ] && command -v psql > /dev/null 2>&1; then
  psql_run() { psql "$DATABASE_URL" -tA -v ON_ERROR_STOP=1 "$@"; }
else
  COMPOSE_FILE="${COMPOSE_FILE:-infra/docker/docker-compose.yml}"
  PSQL_USER="${POSTGRES_USER:-hquizlet}"
  PSQL_DB="${POSTGRES_DB:-hquizlet}"
  PROJECT_ARGS=()
  if [ -n "${COMPOSE_PROJECT_NAME:-}" ]; then PROJECT_ARGS=(--project-name "$COMPOSE_PROJECT_NAME"); fi
  psql_run() { docker compose "${PROJECT_ARGS[@]}" -f "$COMPOSE_FILE" exec -T postgres psql -U "$PSQL_USER" -d "$PSQL_DB" -tA -v ON_ERROR_STOP=1 "$@"; }
fi

echo "=== IAM backfill ($([ "$APPLY" == "true" ] && echo apply || echo dry-run)) ==="

echo ""
echo "-- Legacy role distribution --"
psql_run -c "
SELECT COALESCE(role, '(null)') AS legacy_role, COUNT(*) FROM users GROUP BY 1 ORDER BY 2 DESC;"
echo "-- Expected: 'admin' -> PlatformAdmin, users without a platform assignment -> Student (preserve Teacher and managed roles)"

echo ""
echo "-- Existing platform assignments by role --"
psql_run -c "
SELECT role_key, COUNT(*) FROM iam_assignments WHERE scope_type = 'platform' GROUP BY 1 ORDER BY 2 DESC;"

echo ""
echo "-- Assignments a backfill would create (missing rows) --"
MISSING=$(psql_run -c "
SELECT COUNT(*) FROM (
  SELECT id, 'PlatformAdmin' FROM users WHERE role = 'admin'
  UNION ALL
  SELECT id, 'Student' FROM users u WHERE role <> 'admin' AND NOT EXISTS (SELECT 1 FROM iam_assignments a WHERE a.principal_type='user' AND a.principal_id=u.id AND a.scope_type='platform')
) planned(user_id, role_key)
WHERE NOT EXISTS (
  SELECT 1 FROM iam_assignments a
  WHERE a.principal_type = 'user' AND a.principal_id = planned.user_id
    AND a.role_key = planned.role_key AND a.scope_type = 'platform'
);")
echo "  missing: $MISSING"

psql_run -c "
SELECT planned.role_key, COUNT(*) FROM (
  SELECT id, 'PlatformAdmin' AS role_key FROM users WHERE role = 'admin'
  UNION ALL
  SELECT id, 'Student' FROM users u WHERE role <> 'admin' AND NOT EXISTS (SELECT 1 FROM iam_assignments a WHERE a.principal_type='user' AND a.principal_id=u.id AND a.scope_type='platform')
) planned
WHERE NOT EXISTS (
  SELECT 1 FROM iam_assignments a
  WHERE a.principal_type = 'user' AND a.principal_id = planned.id
    AND a.role_key = planned.role_key AND a.scope_type = 'platform'
)
GROUP BY 1 ORDER BY 1;"

if [ "$MISSING" == "0" ]; then
  echo ""
  echo "Nothing to do: every user already holds its expected platform role."
  exit 0
fi

if [ "$APPLY" != "true" ]; then
  echo ""
  echo "Dry run: no rows written. Re-run with --apply to insert the $MISSING missing assignment(s)."
  exit 0
fi

echo ""
echo "-- Applying backfill --"
psql_run -c "
INSERT INTO iam_assignments (principal_type, principal_id, role_key, scope_type, scope_id, created_by)
SELECT 'user', id, 'PlatformAdmin', 'platform', NULL, id FROM users WHERE role = 'admin'
ON CONFLICT (principal_type, principal_id, role_key, scope_type, scope_id) DO NOTHING;"
psql_run -c "
INSERT INTO iam_assignments (principal_type, principal_id, role_key, scope_type, scope_id, created_by)
SELECT 'user', id, 'Student', 'platform', NULL, id FROM users u WHERE role <> 'admin' AND NOT EXISTS (SELECT 1 FROM iam_assignments a WHERE a.principal_type='user' AND a.principal_id=u.id AND a.scope_type='platform')
ON CONFLICT (principal_type, principal_id, role_key, scope_type, scope_id) DO NOTHING;"

REMAINING=$(psql_run -c "
SELECT COUNT(*) FROM (
  SELECT id, 'PlatformAdmin' FROM users WHERE role = 'admin'
  UNION ALL
  SELECT id, 'Student' FROM users u WHERE role <> 'admin' AND NOT EXISTS (SELECT 1 FROM iam_assignments a WHERE a.principal_type='user' AND a.principal_id=u.id AND a.scope_type='platform')
) planned(user_id, role_key)
WHERE NOT EXISTS (
  SELECT 1 FROM iam_assignments a
  WHERE a.principal_type = 'user' AND a.principal_id = planned.user_id
    AND a.role_key = planned.role_key AND a.scope_type = 'platform'
);")
echo "  remaining after apply: $REMAINING"
[ "$REMAINING" == "0" ] || { echo "FAIL: backfill incomplete"; exit 1; }
echo "Backfill complete."
