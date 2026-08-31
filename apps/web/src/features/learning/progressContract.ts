// progressContract.ts — Dev 4
// P2-LEARN-05: Progress API contract cho Phase 3
//
// STATUS: FINALIZED with Dev 5 (follow-up review 2026-08-31)
// - LearningMode is canonical in lib/api/client.ts — imported from there
// - LearningProgress (backend record shape) is canonical in lib/api/client.ts
// - saveProgress / fetchProgress remain no-op stubs until Phase 3 backend is built
// - Endpoint: POST /v1/study-sets/{studySetId}/progress (resource-based, agreed)
// - userId is NOT sent in body — derived from auth token by gateway

import type { LearningMode, LearningProgress } from "../../lib/api/client";

export type { LearningMode, LearningProgress };

// ── Request/response types specific to progress submission ──────────────────

/**
 * Payload sent to POST /v1/study-sets/{studySetId}/progress
 * Fields align with LearningProgress backend record (client.ts).
 */
export type ProgressSaveRequest = {
  studySetId: number;
  mode: LearningMode;
  /** Số thẻ đúng trong session */
  score: number;
  /** Tổng số thẻ trong session */
  total: number;
};

/**
 * Per-card result — optional, sent when available for adaptive learning (Phase 4+).
 * Truncate to 100 entries max to keep payload size bounded.
 */
export type CardResult = {
  cardId: number;
  correct: boolean;
  /** Số lần thử (Learn mode retry) */
  attempts: number;
  /** Thời gian phản hồi ms — nếu đo được */
  responseTimeMs?: number;
};

// ── Placeholder stubs — Phase 3 will replace with real apiFetch calls ───────

/**
 * Phase 3: POST /v1/study-sets/{studySetId}/progress
 * Currently a no-op; UI must handle absence of persisted progress gracefully.
 */
export async function saveProgress(
  _token: string,
  data: ProgressSaveRequest
): Promise<void> {
  if (import.meta.env.DEV) {
    console.debug("[Dev4 Progress Stub] Would save:", data);
  }
}

/**
 * Phase 3: GET /v1/study-sets/{studySetId}/progress
 * Returns empty array until backend endpoint exists.
 */
export async function fetchProgress(
  _token: string,
  _studySetId: number
): Promise<LearningProgress[]> {
  return [];
}
