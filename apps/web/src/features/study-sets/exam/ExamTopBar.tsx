import { ExamTimer } from "./ExamTimer";

export function ExamTopBar({
  currentPart,
  questionRange,
  totalQuestions,
  answeredCount,
  timeLeft,
  showTimer = true,
  muted = false,
  onSubmit,
  onVolumeChange,
}: {
  currentPart: string;
  questionRange: string;
  totalQuestions: number;
  answeredCount: number;
  timeLeft: number;
  showTimer?: boolean;
  muted?: boolean;
  onSubmit: () => void;
  onVolumeChange?: () => void;
}) {
  return (
    <div className="flex h-14 shrink-0 items-center gap-4 bg-[#1a2744] px-4 shadow-md">
      <div className="flex shrink-0 items-center justify-center rounded bg-white px-3 py-1">
        <span className="text-lg font-black tracking-wide text-[#1a2744]">IIG</span>
      </div>
      <div className="min-w-0 truncate text-[15px] font-medium text-white">
        {currentPart}: Questions {questionRange} of {totalQuestions}
      </div>
      <div className="ml-auto flex shrink-0 items-center gap-2">
        {onVolumeChange && (
          <button
            type="button"
            onClick={onVolumeChange}
            title={muted ? "Bật âm thanh" : "Tắt âm thanh"}
            className="flex h-8 w-8 items-center justify-center rounded bg-[#4a90d9] text-sm text-white transition-colors hover:bg-[#3d7fc4]"
          >
            {muted ? "🔇" : "🔊"}
          </button>
        )}
        <span className="rounded bg-[#2d3f6b] px-3 py-1.5 text-xs font-bold tabular-nums text-white">
          {answeredCount}/{totalQuestions}
        </span>
        {showTimer && (
          <span className="rounded bg-[#2d3f6b] px-3 py-1.5">
            <ExamTimer secondsLeft={timeLeft} />
          </span>
        )}
        <button
          type="button"
          onClick={onSubmit}
          className="rounded bg-[#ff8c00] px-5 py-1.5 text-sm font-bold text-white transition-colors hover:bg-[#e67e00]"
        >
          Submit
        </button>
      </div>
    </div>
  );
}
