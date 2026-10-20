import type { ReactNode } from "react";

import { StatusPill, type SaveState } from "./StatusPill";

/** The only top bar in the quiz flow. Both layouts use it, so TOEIC no longer
 * ships its own dark "Lưu và thoát" header. */
export function QuizTopBar({
  title,
  position,
  total,
  answered,
  timer,
  marked,
  onToggleMark,
  canMark = true,
  canSubmit = true,
  saveState = "idle",
  onRetrySave,
  onExit,
  onSubmit,
}: {
  title: string;
  position: number;
  total: number;
  answered: number;
  timer?: ReactNode;
  marked: boolean;
  onToggleMark: () => void;
  canMark?: boolean;
  canSubmit?: boolean;
  saveState?: SaveState;
  onRetrySave?: () => void;
  onExit: () => void;
  onSubmit: () => void;
}) {
  const percent = total > 0 ? Math.min(100, Math.round((answered / total) * 100)) : 0;
  return (
    <header className="qz-top">
      <div className="qz-top-row">
        <button type="button" className="qz-icon-btn" onClick={onExit} aria-label="Lưu và thoát" title="Lưu và thoát">
          ✕
        </button>
        <div className="qz-top-title">
          <strong>{title}</strong>
          <span>
            Câu {position}/{total} · {answered} đã làm
          </span>
        </div>
        <div className="qz-top-actions">
          <StatusPill state={saveState} onRetry={onRetrySave} />
          {timer}
          {canMark ? (
            <button
              type="button"
              className={`qz-icon-btn${marked ? " qz-icon-btn--on" : ""}`}
              aria-pressed={marked}
              aria-label={marked ? "Bỏ đánh dấu câu này" : "Đánh dấu câu này"}
              title="Đánh dấu để xem lại"
              onClick={onToggleMark}
            >
              ⚑
            </button>
          ) : null}
          <button type="button" className="qz-btn qz-btn--primary qz-btn--sm" disabled={!canSubmit} onClick={onSubmit}>
            Nộp bài
          </button>
        </div>
      </div>
      <div
        className="qz-progress"
        role="progressbar"
        aria-label="Tiến độ làm bài"
        aria-valuemin={0}
        aria-valuemax={total}
        aria-valuenow={answered}
      >
        <span style={{ width: `${percent}%` }} />
      </div>
    </header>
  );
}
