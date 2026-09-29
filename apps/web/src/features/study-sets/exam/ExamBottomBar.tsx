export function ExamBottomBar({
  isMarked,
  onToggleMark,
  onOpenNavigator,
  onPrev,
  onNext,
  canPrev,
  canNext,
}: {
  isMarked: boolean;
  onToggleMark: () => void;
  onOpenNavigator: () => void;
  onPrev: () => void;
  onNext: () => void;
  canPrev: boolean;
  canNext: boolean;
}) {
  return (
    <div className="flex h-14 shrink-0 items-center justify-between bg-[#1a2744] px-4">
      <label className="flex cursor-pointer select-none items-center gap-2 text-xs text-white">
        <input
          type="checkbox"
          checked={isMarked}
          onChange={onToggleMark}
          className="size-4 cursor-pointer accent-[#ff8c00]"
        />
        Mark item for review
      </label>
      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={onOpenNavigator}
          title="Bảng điều khiển (Navigator)"
          className="flex h-9 w-9 items-center justify-center rounded bg-[#2d3f6b] text-lg font-bold text-white transition-colors hover:bg-[#3a508a]"
        >
          ⊞
        </button>
        <button
          type="button"
          onClick={onPrev}
          disabled={!canPrev}
          title="Câu trước"
          className="flex h-9 w-9 items-center justify-center rounded bg-[#2d3f6b] text-lg font-bold text-white transition-colors hover:bg-[#3a508a] disabled:opacity-40 disabled:hover:bg-[#2d3f6b]"
        >
          ‹
        </button>
        <button
          type="button"
          onClick={onNext}
          disabled={!canNext}
          title="Câu sau"
          className="flex h-9 w-9 items-center justify-center rounded bg-[#2d3f6b] text-lg font-bold text-white transition-colors hover:bg-[#3a508a] disabled:opacity-40 disabled:hover:bg-[#2d3f6b]"
        >
          ›
        </button>
      </div>
    </div>
  );
}
