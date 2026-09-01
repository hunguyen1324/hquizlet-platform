// progressContract.ts — Dev 4
// P3-LEARN-01..03: Progress API contract — Phase 3 real implementation
//
// REPLACES: Phase 2 no-op stubs
// STATUS: Phase 3 — real apiFetch calls to POST/GET /v1/study-sets/{id}/progress
//
// Contract:
//   - userId is NOT sent in body — derived from auth token by gateway
//   - studySetId comes from path, NOT body
//   - cardResults truncated to max 100 items
//   - idempotencyKey prevents duplicate sessions on retry

import { apiFetch } from "../../lib/api/client";
import type { LearningMode, LearningProgress } from "../../lib/api/client";

export type { LearningMode, LearningProgress };

// ── Request/response types ──────────────────────────────────────────────────

/**
 * Per-card result — sent when available (Learn, Test, Match modes).
 * Max 100 items per contract. Truncation is done in saveProgress.
 */
export type CardResult = {
  cardId: number;
  correct: boolean;
  /** Số lần thử — Learn mode retry. Clamped 1..100. */
  attempts: number;
  /** Response time ms — nếu đo được. */
  responseTimeMs?: number;
};

/**
 * Payload sent to POST /v1/study-sets/{studySetId}/progress
 * Do NOT include studySetId or userId in body — sent via path/token.
 */
export type ProgressSaveRequest = {
  mode: LearningMode;
  score: number;
  total: number;
  startedAt: string;       // ISO 8601
  completedAt: string;     // ISO 8601
  cardResults: CardResult[];
  idempotencyKey: string;
};

/**
 * Response shape from POST /v1/study-sets/{studySetId}/progress
 * Backend returns the created session record.
 */
export type ProgressSaveResponse = {
  id: number;
  mode: LearningMode;
  score: number;
  total: number;
  completedAt: string | null;
  createdAt: string;
};

/**
 * Response shape from GET /v1/study-sets/{studySetId}/progress
 */
export type ProgressListResponse = {
  items: LearningProgress[];
  total: number;
  page: number;
  pageSize: number;
};

/**
 * Response shape from GET /v1/study-sets/{studySetId}/progress/latest
 */
export type ProgressLatestResponse = {
  byMode: Partial<Record<LearningMode, LearningProgress>>;
};

// ── Error types ─────────────────────────────────────────────────────────────

export type ProgressSaveError =
  | { kind: "network" }
  | { kind: "unauthorized" }
  | { kind: "forbidden" }
  | { kind: "conflict"; message: string }   // 409 idempotency — safe to ignore
  | { kind: "validation"; message: string }
  | { kind: "server"; message: string };

// ── API functions ────────────────────────────────────────────────────────────

/**
 * POST /v1/study-sets/{studySetId}/progress
 *
 * Returns the saved session on success, or a typed error.
 * 409 (idempotency conflict) is returned as { kind: "conflict" } — callers
 * SHOULD treat this as success (session was already saved).
 */
export async function saveProgress(
  token: string,
  studySetId: number,
  data: ProgressSaveRequest
): Promise<{ ok: true; session: ProgressSaveResponse } | { ok: false; error: ProgressSaveError }> {
  // Truncate cardResults to max 100 per contract.
  const payload: ProgressSaveRequest = {
    ...data,
    cardResults: data.cardResults.slice(0, 100),
  };

  try {
    const session = await apiFetch<ProgressSaveResponse>(
      `/v1/study-sets/${studySetId}/progress`,
      token,
      { method: "POST", body: JSON.stringify(payload) }
    );
    return { ok: true, session };
  } catch (err: unknown) {
    return { ok: false, error: classifyError(err) };
  }
}

/**
 * GET /v1/study-sets/{studySetId}/progress?page=1&pageSize=20
 *
 * Returns paginated history. Returns empty items on auth/fetch error
 * (progress history is non-critical UI — callers handle gracefully).
 */
export async function fetchProgress(
  token: string,
  studySetId: number,
  page = 1,
  pageSize = 20
): Promise<LearningProgress[]> {
  try {
    const result = await apiFetch<ProgressListResponse>(
      `/v1/study-sets/${studySetId}/progress`,
      token,
      {},
      { page, pageSize }
    );
    return result.items ?? [];
  } catch {
    return [];
  }
}

/**
 * GET /v1/study-sets/{studySetId}/progress/latest
 *
 * Returns latest session per mode. Returns {} on error.
 */
export async function fetchLatestProgress(
  token: string,
  studySetId: number
): Promise<Partial<Record<LearningMode, LearningProgress>>> {
  try {
    const result = await apiFetch<ProgressLatestResponse>(
      `/v1/study-sets/${studySetId}/progress/latest`,
      token
    );
    return result.byMode ?? {};
  } catch {
    return {};
  }
}

// ── Idempotency key generation ───────────────────────────────────────────────

/**
 * Generate a stable idempotency key for a session.
 * Format: {studySetId}:{mode}:{startedAt ISO}
 * Allows safe retry without duplicate sessions.
 */
export function makeIdempotencyKey(
  studySetId: number,
  mode: LearningMode,
  startedAt: string
): string {
  return `${studySetId}:${mode}:${startedAt}`;
}

// ── Internal ─────────────────────────────────────────────────────────────────

function classifyError(err: unknown): ProgressSaveError {
  if (!(err instanceof Error)) return { kind: "network" };
  const msg = err.message ?? "";
  if (msg.includes("401")) return { kind: "unauthorized" };
  if (msg.includes("403")) return { kind: "forbidden" };
  if (msg.includes("409")) return { kind: "conflict", message: msg };
  if (msg.includes("422") || msg.includes("400"))
    return { kind: "validation", message: msg };
  if (msg.includes("5")) return { kind: "server", message: msg };
  return { kind: "network" };
}
