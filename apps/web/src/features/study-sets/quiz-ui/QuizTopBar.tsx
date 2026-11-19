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
  onOpenGrid,
  onToggleFullscreen,
  fullscreen = false,
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
  onOpenGrid: () => void;
  onToggleFullscreen: () => void;
  fullscreen?: boolean;
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
          <button type="button" className="qz-btn qz-btn--ghost qz-btn--sm" onClick={onOpenGrid} aria-haspopup="dialog">
            <span aria-hidden="true">☰</span> Bảng câu
          </button>
          <button type="button" className="qz-icon-btn" onClick={onToggleFullscreen}
            aria-label={fullscreen ? "Thoát toàn màn hình" : "Toàn màn hình"}
            title={fullscreen ? "Thoát toàn màn hình" : "Toàn màn hình"}>
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
              <path d={fullscreen ? "M9 3v6H3m12-6v6h6M3 15h6v6m12-6h-6v6" : "M9 4H4v5m11-5h5v5M4 15v5h5m11-5v5h-5"} />
            </svg>
          </button>
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
