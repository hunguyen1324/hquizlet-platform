import type { ReactNode } from "react";

/** Fixed action bar. Primary action sits in the thumb zone on a phone, and a
 * disabled action always explains itself through `hint`. */
export function QuizBottomBar({
  canPrev,
  canNext,
  onPrev,
  onNext,
  onOpenGrid,
  nextLabel = "Tiếp",
  hint,
  children,
}: {
  canPrev: boolean;
  canNext: boolean;
  onPrev: () => void;
  onNext: () => void;
  onOpenGrid: () => void;
  nextLabel?: string;
  hint?: string;
  children?: ReactNode;
}) {
  return (
    <nav className="qz-bottom" aria-label="Điều hướng câu hỏi">
      {hint ? (
        <p className="qz-hint" role="status">
          {hint}
        </p>
      ) : null}
      <div className="qz-bottom-row">
        <button type="button" className="qz-btn qz-btn--ghost" onClick={onOpenGrid} aria-label="Mở bảng câu hỏi">
          ☰ <span>Bảng câu</span>
        </button>
        <div className="qz-bottom-nav">
          <button type="button" className="qz-btn qz-btn--ghost" disabled={!canPrev} onClick={onPrev}>
            ‹ Trước
          </button>
          <button type="button" className="qz-btn qz-btn--primary" disabled={!canNext} onClick={onNext}>
            {nextLabel} ›
          </button>
        </div>
      </div>
      {children}
    </nav>
  );
}
