import { useEffect, useRef } from "react";

import { RichTextContent } from "../exam/RichTextContent";

/** Result of the question the learner just answered, shown as a bottom sheet so
 * the answer options stay visible on a phone and the primary action (next
 * question) stays within thumb reach. */
export function FeedbackSheet({
  open,
  correct,
  correctAnswer,
  explanation,
  canNext,
  onNext,
  onClose,
}: {
  open: boolean;
  correct: boolean;
  correctAnswer?: string;
  explanation?: string;
  canNext: boolean;
  onNext: () => void;
  onClose: () => void;
}) {
  const nextRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (open) nextRef.current?.focus();
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;
  return (
    <div className="qz-sheet-backdrop" onClick={onClose}>
      <section
        className={`qz-sheet ${correct ? "qz-sheet--ok" : "qz-sheet--bad"}`}
        role="dialog"
        aria-modal="false"
        aria-label={correct ? "Chính xác" : "Chưa đúng"}
        onClick={(event) => event.stopPropagation()}
      >
        {/* Live region announces the result without moving focus off the sheet. */}
        <p className="qz-sheet-title" aria-live="polite">
          <span aria-hidden="true">{correct ? "✓" : "✗"}</span> {correct ? "Chính xác" : "Chưa đúng"}
        </p>
        {correctAnswer ? (
          <p className="qz-sheet-answer">
            <strong>Đáp án:</strong> {correctAnswer}
          </p>
        ) : null}
        {explanation ? <RichTextContent value={explanation} className="qz-sheet-explanation" /> : null}
        <div className="qz-sheet-actions">
          <button type="button" className="qz-btn qz-btn--ghost" onClick={onClose}>
            Đóng
          </button>
          <button type="button" ref={nextRef} className="qz-btn qz-btn--primary" disabled={!canNext} onClick={onNext}>
            Câu tiếp theo
          </button>
        </div>
      </section>
    </div>
  );
}
