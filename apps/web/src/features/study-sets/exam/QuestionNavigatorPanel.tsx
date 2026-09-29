import { useMemo, type CSSProperties } from "react";

import type { ExamItem } from "./types";

function cellClass(
  i: number,
  answers: (string | null)[],
  flagged: boolean[],
  results: { isCorrect: boolean }[] | undefined,
  currentIndex: number,
): { className: string; style: CSSProperties } {
  let style: CSSProperties = {};
  let className =
    "w-8 h-8 rounded-md text-xs font-bold flex items-center justify-center cursor-pointer transition-all select-none border shrink-0";

  const r = results?.[i];
  if (r) {
    if (r.isCorrect) {
      style = { backgroundColor: "#2d8a4e", color: "#fff", borderColor: "#2d8a4e" };
    } else if (answers[i]) {
      style = { backgroundColor: "#c0392b", color: "#fff", borderColor: "#c0392b" };
    } else {
      style = { backgroundColor: "#e8e8e8", color: "#6b7280", borderColor: "#d8d8d8" };
    }
  } else if (flagged[i]) {
    style = { backgroundColor: "#ff8800", color: "#fff", borderColor: "#ff8800" };
  } else if (answers[i]) {
    style = { backgroundColor: "#1a2744", color: "#fff", borderColor: "#1a2744" };
  } else {
    style = { backgroundColor: "#e8e8e8", color: "#6b7280", borderColor: "#d8d8d8" };
  }

  if (i === currentIndex) {
    className += " ring-2 ring-offset-1 ring-blue-500";
  }

  return { className, style };
}

export function QuestionNavigatorPanel({
  items,
  currentIndex,
  answers,
  flagged,
  results,
  onNavigate,
  isOpen = false,
  onClose,
  embedded = false,
}: {
  items: ExamItem[];
  currentIndex: number;
  answers: (string | null)[];
  flagged: boolean[];
  results?: { isCorrect: boolean }[];
  onNavigate: (index: number) => void;
  isOpen?: boolean;
  onClose?: () => void;
  embedded?: boolean;
}) {
  const groups = useMemo(() => {
    const map = new Map<number, number[]>();
    items.forEach((it, i) => {
      const arr = map.get(it.part) ?? [];
      arr.push(i);
      map.set(it.part, arr);
    });
    return [...map.entries()].sort((a, b) => a[0] - b[0]);
  }, [items]);

  const answeredCount = answers.filter(Boolean).length;
  const flaggedCount = flagged.filter(Boolean).length;

  const legend = results
    ? [
        { label: "Đúng", color: "#2d8a4e" },
        { label: "Sai", color: "#c0392b" },
        { label: "Bỏ qua", color: "#e8e8e8" },
      ]
    : [
        { label: `Đã chọn (${answeredCount})`, color: "#1a2744" },
        { label: `Flagged (${flaggedCount})`, color: "#ff8800" },
        { label: "Chưa làm", color: "#e8e8e8" },
      ];

  const body = (
    <div className="flex h-full flex-col">
      <div className="flex shrink-0 items-center justify-between border-b border-zinc-200 bg-[#1a2744] px-4 py-3 text-white">
        <div className="flex items-center gap-2">
          <span className="text-[#ff9900]">▦</span>
          <span className="text-sm font-bold">Bảng điều khiển</span>
        </div>
        {onClose && (
          <button
            type="button"
            onClick={onClose}
            className="rounded p-1 hover:bg-white/10"
            aria-label="Đóng bảng điều khiển"
          >
            ✕
          </button>
        )}
      </div>
      <div className="flex shrink-0 flex-wrap items-center gap-x-4 gap-y-1.5 border-b border-zinc-100 px-4 py-2.5">
        {legend.map((item) => (
          <span key={item.label} className="flex items-center gap-1.5 text-[11px] font-medium text-zinc-600">
            <span
              className="size-3 rounded-full border"
              style={{
                backgroundColor: item.color,
                borderColor: item.color === "#e8e8e8" ? "#d8d8d8" : item.color,
              }}
            />
            {item.label}
          </span>
        ))}
      </div>
      <div className="flex-1 space-y-5 overflow-y-auto px-4 py-4 [&::-webkit-scrollbar]:w-1.5 [&::-webkit-scrollbar-thumb]:rounded-full [&::-webkit-scrollbar-thumb]:bg-zinc-300">
        {groups.map(([part, indexes]) => (
          <div key={part}>
            <div className="mb-2 flex items-center gap-2">
              <span className="text-xs font-extrabold uppercase tracking-wide text-[#1a56a0]">
                Part {part}
              </span>
              <span className="h-px flex-1 bg-zinc-200" />
            </div>
            <div className="grid grid-cols-6 gap-1.5">
              {indexes.map((i) => {
                const { className, style } = cellClass(i, answers, flagged, results, currentIndex);
                return (
                  <button
                    key={items[i]?.key ?? i}
                    type="button"
                    className={className}
                    style={style}
                    onClick={() => onNavigate(i)}
                    title={`Câu ${items[i]?.questionNumber ?? i + 1}`}
                  >
                    {items[i]?.questionNumber ?? i + 1}
                  </button>
                );
              })}
            </div>
          </div>
        ))}
      </div>
    </div>
  );

  if (embedded) {
    return (
      <div className="flex h-full flex-col overflow-hidden rounded-xl border border-zinc-200 bg-white shadow-sm">
        {body}
      </div>
    );
  }

  return (
    <>
      {isOpen && (
        <div className="fixed inset-0 z-[155] bg-black/40" onClick={onClose} aria-hidden />
      )}
      <aside
        className={`fixed top-0 right-0 z-[156] h-full w-[300px] max-w-[85vw] bg-white shadow-2xl transition-transform duration-300 ${
          isOpen ? "translate-x-0" : "translate-x-full"
        }`}
      >
        {body}
      </aside>
    </>
  );
}
