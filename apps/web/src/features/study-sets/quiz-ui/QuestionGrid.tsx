import { useEffect, useMemo, useState } from "react";

type Filter = "all" | "unanswered" | "flagged";

/** Question grid inside a bottom sheet. Every cell states its status with a
 * glyph as well as colour, and quick filters replace hunting through 200 cells. */
export function QuestionGrid({
  open,
  items,
  answers,
  flags,
  results,
  currentIndex,
  onNavigate,
  onClose,
}: {
  open: boolean;
  items: { number: number; part: number }[];
  answers: Record<number, string>;
  flags: boolean[];
  results?: Record<number, boolean>;
  currentIndex: number;
  onNavigate: (index: number) => void;
  onClose: () => void;
}) {
  const [filter, setFilter] = useState<Filter>("all");

  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  const indices = useMemo(() => {
    const all = items.map((_, i) => i);
    if (filter === "unanswered") return all.filter((i) => answers[i] === undefined);
    if (filter === "flagged") return all.filter((i) => flags[i]);
    return all;
  }, [items, answers, flags, filter]);

  const answeredCount = Object.keys(answers).length;
  const flaggedCount = flags.filter(Boolean).length;

  if (!open) return null;
  return (
    <div className="qz-sheet-backdrop qz-sheet-backdrop--grid" onClick={onClose}>
      <section
        className="qz-sheet qz-sheet--grid"
        role="dialog"
        aria-modal="true"
        aria-label="Bảng câu hỏi"
        onClick={(event) => event.stopPropagation()}
      >
        <header className="qz-sheet-head">
          <strong>
            Bảng câu · {answeredCount}/{items.length} đã làm
          </strong>
          <button type="button" className="qz-icon-btn" onClick={onClose} aria-label="Đóng bảng câu hỏi">
            ✕
          </button>
        </header>
        <div className="qz-filters" role="group" aria-label="Lọc câu hỏi">
          {(
            [
              ["all", `Tất cả (${items.length})`],
              ["unanswered", `Chưa làm (${items.length - answeredCount})`],
              ["flagged", `Đánh dấu (${flaggedCount})`],
            ] as const
          ).map(([value, label]) => (
            <button
              key={value}
              type="button"
              className={`qz-chip${filter === value ? " qz-chip--on" : ""}`}
              aria-pressed={filter === value}
              onClick={() => setFilter(value)}
            >
              {label}
            </button>
          ))}
        </div>
        <div className="qz-grid">
          {indices.map((i) => {
            const item = items[i]!;
            const result = results?.[i];
            const answered = answers[i] !== undefined;
            const flagged = flags[i] === true;
            const status = result !== undefined ? (result ? "correct" : answered ? "wrong" : "skipped") : answered ? "answered" : "blank";
            const glyph = flagged ? "⚑" : result !== undefined ? (result ? "✓" : "✗") : answered ? "•" : "";
            return (
              <button
                key={i}
                type="button"
                className={`qz-grid-cell qz-grid-cell--${status}${i === currentIndex ? " qz-grid-cell--current" : ""}${flagged ? " qz-grid-cell--flagged" : ""}`}
                aria-label={`Câu ${item.number}, Part ${item.part}${answered ? ", đã làm" : ", chưa làm"}${flagged ? ", đã đánh dấu" : ""}`}
                aria-current={i === currentIndex ? "true" : undefined}
                onClick={() => onNavigate(i)}
              >
                <span>{item.number}</span>
                {glyph ? (
                  <span className="qz-grid-glyph" aria-hidden="true">
                    {glyph}
                  </span>
                ) : null}
              </button>
            );
          })}
        </div>
        <p className="qz-legend">
          <span>• đã làm</span>
          <span>⚑ đánh dấu</span>
          <span>✓ đúng</span>
          <span>✗ sai</span>
        </p>
      </section>
    </div>
  );
}
