// progressContract.ts — Dev 4
// P2-LEARN-05: Đề xuất API lưu progress cho Phase 3
// Đây là contract DRAFT - chưa gọi backend, chờ Dev 5 review và chốt endpoint

// ── Types ──────────────────────────────────────────────────────────────────

export type LearningMode = "flashcards" | "learn" | "test" | "match";

/**
 * Gửi lên backend sau khi user hoàn thành 1 session học.
 * Endpoint đề xuất: POST /v1/study-sets/{studySetId}/progress
 */
export type ProgressSaveRequest = {
  studySetId: number;
  mode: LearningMode;
  /** Tổng số thẻ trong session */
  totalCards: number;
  /** Số thẻ trả lời đúng */
  correctCards: number;
  /** Thời gian hoàn thành tính bằng ms */
  durationMs: number;
  /** Timestamp bắt đầu (ms epoch) */
  startedAt: number;
  /** Timestamp kết thúc (ms epoch) */
  finishedAt: number;
  /** Chi tiết từng thẻ (tuỳ chọn, dùng cho adaptive learning sau) */
  cardResults?: CardResult[];
};

export type CardResult = {
  cardId: number;
  /** Đúng hay sai */
  correct: boolean;
  /** Số lần thử (dùng trong learn mode retry) */
  attempts: number;
  /** Thời gian phản hồi ms (nếu có đo) */
  responseTimeMs?: number;
};

/**
 * Backend trả về sau khi lưu progress.
 * Endpoint đề xuất: GET /v1/study-sets/{studySetId}/progress
 */
export type ProgressSummary = {
  studySetId: number;
  /** Số session đã học */
  sessionCount: number;
  /** Điểm tốt nhất theo mode */
  bestScores: Record<LearningMode, number | null>; // null = chưa chơi
  /** Thời gian match tốt nhất (ms) */
  bestMatchTimeMs: number | null;
  /** Ngày học lần cuối */
  lastStudiedAt: string; // ISO8601
};

// ── Placeholder hooks - Phase 3 sẽ implement thật ──────────────────────────

/**
 * TODO Phase 3: Gọi POST /v1/study-sets/{studySetId}/progress
 * Hiện tại: no-op, chỉ log để debug
 */
export async function saveProgress(
  _token: string,
  data: ProgressSaveRequest
): Promise<void> {
  // Phase 3: replace with real API call
  // await apiFetch(`/v1/study-sets/${data.studySetId}/progress`, token, {
  //   method: "POST",
  //   body: JSON.stringify(data),
  // });
  if (process.env.NODE_ENV === "development") {
    console.debug("[Dev4 Progress Draft] Would save:", data);
  }
}

/**
 * TODO Phase 3: Gọi GET /v1/study-sets/{studySetId}/progress
 * Hiện tại: trả về null, UI phải handle gracefully
 */
export async function fetchProgress(
  _token: string,
  _studySetId: number
): Promise<ProgressSummary | null> {
  // Phase 3: replace with real API call
  return null;
}

// ── Dev 5 Review Notes ──────────────────────────────────────────────────────
/**
 * REVIEW REQUEST (Dev 4 → Dev 5):
 *
 * 1. Endpoint convention:
 *    - Dùng /v1/study-sets/{id}/progress (resource-based) hay /v1/progress?studySetId={id}?
 *    - Nên có sub-resource per mode: /v1/study-sets/{id}/progress/{mode}?
 *
 * 2. Storage strategy:
 *    - Lưu mỗi session riêng (history table) hay chỉ lưu aggregate (best/last)?
 *    - Phase 3 nên có history để vẽ graph progress sau.
 *
 * 3. Auth:
 *    - Progress gắn với userId từ token, không cần gửi userId trong body.
 *
 * 4. Versioning:
 *    - Nếu schema thay đổi giữa Phase 3 và 4, cần migration strategy.
 *
 * 5. cardResults field:
 *    - Optional để tránh payload quá lớn với set nhiều thẻ.
 *    - Có thể truncate nếu > 100 cards.
 */
