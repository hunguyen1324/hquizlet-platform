import { useCallback, useEffect, useRef, useState } from "react";

import { RichTextContent } from "./RichTextContent";
import { PART_DIRECTIONS, isAnswerCorrect } from "./toeic";
import type { ExamItem, QuizQuestionLike } from "./types";

function parseSrt(content: string) {
  const blocks = content.trim().split(/\n\n+/);
  return blocks.flatMap((block) => {
    const lines = block.split(/\r?\n/).filter(Boolean);
    if (lines.length < 2) return [];
    const timeLine = lines[1] ?? "";
    const m = /(\d+):(\d+):(\d+)[.,](\d+)\s*-->\s*(\d+):(\d+):(\d+)[.,](\d+)/.exec(timeLine);
    if (!m) return [];
    const toSec = (h: string, min: string, s: string, ms: string) =>
      +h * 3600 + +min * 60 + +s + +ms / 1000;
    const g = (i: number) => m[i] ?? "";
    return [
      {
        start: toSec(g(1), g(2), g(3), g(4)),
        end: toSec(g(5), g(6), g(7), g(8)),
        text: lines
          .slice(2)
          .join(" ")
          .replace(/<[^>]+>/g, "")
          .trim(),
      },
    ];
  });
}

function formatTime(sec: number) {
  return `${Math.floor(sec / 60)}:${String(Math.floor(sec % 60)).padStart(2, "0")}`;
}

export function CustomAudioPlayer({
  src,
  srtUrl,
  muted,
}: {
  src: string;
  srtUrl?: string | null;
  muted: boolean;
}) {
  const audioRef = useRef<HTMLAudioElement>(null);
  const [error, setError] = useState("");
  const [isPlaying, setIsPlaying] = useState(false);
  const [current, setCurrent] = useState(0);
  const [duration, setDuration] = useState(0);
  const [isSeeking, setIsSeeking] = useState(false);
  const [cues, setCues] = useState<{ start: number; end: number; text: string }[]>([]);
  const [activeCue, setActiveCue] = useState(-1);

  useEffect(() => {
    if (audioRef.current) audioRef.current.muted = muted;
  }, [muted]);

  useEffect(() => {
    setError("");
    setIsPlaying(false);
    setCurrent(0);
    setDuration(0);
    setCues([]);
    setActiveCue(-1);
    if (!srtUrl) return;
    fetch(srtUrl)
      .then((r) => r.text())
      .then((t) => setCues(parseSrt(t)))
      .catch(() => setCues([]));
  }, [src, srtUrl]);

  useEffect(() => {
    if (!cues.length) return;
    const idx = cues.findIndex((c) => current >= c.start && current < c.end);
    setActiveCue(idx);
  }, [current, cues]);

  const togglePlay = useCallback(() => {
    const el = audioRef.current;
    if (!el) return;
    if (isPlaying) el.pause();
    else { setError(""); void el.play().catch(() => setError("Không phát được âm thanh. Vui lòng thử lại hoặc kiểm tra đường dẫn file nghe.")); }
  }, [isPlaying]);

  const srtMax = cues.length > 0 ? Math.max(...cues.map((c) => c.end || 0)) : 0;
  const effectiveDuration = duration > 0 ? duration : srtMax;

  return (
    <div className="rounded-lg border border-zinc-200 bg-zinc-50 px-4 py-3">
      <audio
        ref={audioRef}
        controlsList="nodownload"
        onError={() => setError("Không tải được file nghe. Vui lòng kiểm tra đường dẫn hoặc quyền truy cập.")}
        src={src}
        preload="metadata"
        className="hidden"
        onTimeUpdate={() => {
          const el = audioRef.current;
          if (!el || isSeeking) return;
          setCurrent(el.currentTime);
        }}
        onLoadedMetadata={() => {
          const el = audioRef.current;
          if (el && !isNaN(el.duration) && isFinite(el.duration) && el.duration > 0) {
            setDuration(el.duration);
          }
        }}
        onPlay={() => setIsPlaying(true)}
        onPause={() => setIsPlaying(false)}
        onEnded={() => setIsPlaying(false)}
      />
      {error && <p role="alert" className="text-sm text-red-600">{error}</p>}
      <div className="flex items-center gap-3">
        <button
          type="button"
          onClick={togglePlay}
          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-[#4a90d9] text-xs text-white transition-colors hover:bg-[#3d7fc4]"
          title={isPlaying ? "Tạm dừng" : "Phát"}
        >
          {isPlaying ? "❚❚" : "▶"}
        </button>
        <input
          type="range"
          aria-label="Vị trí phát âm thanh"
          min={0}
          max={effectiveDuration > 0 ? effectiveDuration : 1}
          step={0.1}
          value={effectiveDuration > 0 ? Math.min(current, effectiveDuration) : 0}
          onMouseDown={() => {
            setIsSeeking(true);
            audioRef.current?.pause();
          }}
          onTouchStart={() => {
            setIsSeeking(true);
            audioRef.current?.pause();
          }}
          onMouseUp={(e) => {
            const next = Number((e.target as HTMLInputElement).value);
            if (audioRef.current) {
              audioRef.current.currentTime = next;
              void audioRef.current.play().catch(() => null);
            }
            setCurrent(next);
            setIsSeeking(false);
          }}
          onTouchEnd={(e) => {
            const next = Number((e.target as HTMLInputElement).value);
            if (audioRef.current) {
              audioRef.current.currentTime = next;
              void audioRef.current.play().catch(() => null);
            }
            setCurrent(next);
            setIsSeeking(false);
          }}
          onChange={(e) => setCurrent(Number(e.target.value))}
          className="flex-1 accent-[#4a90d9]"
        />
        <span className="w-24 shrink-0 text-right text-xs tabular-nums text-zinc-600">
          {formatTime(current)} / {formatTime(effectiveDuration)}
        </span>
      </div>
      {cues.length > 0 && (
        <div className="mt-2 max-h-36 space-y-1 overflow-y-auto rounded-lg border border-zinc-200 bg-white p-2 text-sm">
          {cues.map((cue, idx) => (
            <div
              key={idx}
              onClick={() => {
                if (audioRef.current) {
                  audioRef.current.currentTime = cue.start;
                  void audioRef.current.play().catch(() => null);
                }
                setCurrent(cue.start);
              }}
              className={`cursor-pointer rounded px-2 py-1 transition-colors ${
                idx === activeCue
                  ? "bg-[#4a90d9] font-semibold text-white"
                  : "text-zinc-600 hover:bg-zinc-100"
              }`}
            >
              <span className="mr-2 text-xs opacity-60">{formatTime(cue.start)}</span>
              {cue.text}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function renderOptions(
  question: QuizQuestionLike,
  flatIdx: number,
  selected: string | null,
  onAnswer: (flatIdx: number, answer: string) => void,
) {
  const questionType = question.questionType;

  if (questionType === "true_false") {
    const options = ["True", "False"];
    return (
      <div className="space-y-2">
        {options.map((opt) => {
          const value = opt.toLowerCase();
          const checked = selected === value;
          return (
            <label
              key={opt}
              className={`flex cursor-pointer items-center gap-3 rounded-md border px-3 py-2.5 transition-all hover:bg-blue-50/60 ${
                checked
                  ? "border-[#4a90d9] bg-blue-50/40 ring-1 ring-[#4a90d9]"
                  : "border-zinc-200 bg-white hover:border-[#4a90d9]/60"
              }`}
            >
              <input
                type="radio"
                name={`exam-q-${flatIdx}`}
                checked={checked}
                onChange={() => onAnswer(flatIdx, value)}
                className="sr-only"
              />
              <span
                className={`flex h-6 w-6 shrink-0 items-center justify-center rounded-full border text-xs font-bold ${
                  checked
                    ? "border-[#4a90d9] bg-[#4a90d9] text-white"
                    : "border-zinc-300 text-zinc-500"
                }`}
              >
                {opt[0]}
              </span>
              <span className="text-sm font-medium text-zinc-800">{opt}</span>
            </label>
          );
        })}
      </div>
    );
  }

  const options = question.options ?? [];
  if (options.length > 0) {
    return (
      <div className="space-y-2">
        {options.map((opt, optIdx) => {
          const letter = String.fromCharCode(65 + optIdx);
          const checked = selected === opt.text;
          return (
            <label
              key={opt.id ?? optIdx}
              className={`flex cursor-pointer items-start gap-3 rounded-md border px-3 py-2.5 transition-all hover:bg-blue-50/60 ${
                checked
                  ? "border-[#4a90d9] bg-blue-50/40 ring-1 ring-[#4a90d9]"
                  : "border-zinc-200 bg-white hover:border-[#4a90d9]/60"
              }`}
            >
              <input
                type="radio"
                name={`exam-q-${flatIdx}`}
                checked={checked}
                onChange={() => onAnswer(flatIdx, opt.text)}
                className="sr-only"
              />
              <span
                className={`mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full border text-xs font-bold ${
                  checked
                    ? "border-[#4a90d9] bg-[#4a90d9] text-white"
                    : "border-zinc-300 text-zinc-500"
                }`}
              >
                {letter}
              </span>
              <span className="min-w-0 flex-1 text-sm font-medium leading-relaxed text-zinc-800">
                ({letter}) {opt.text}
              </span>
            </label>
          );
        })}
      </div>
    );
  }

  if (questionType === "written" || questionType === "multiple_choice") {
    return (
      <textarea
        value={selected ?? ""}
        onChange={(e) => onAnswer(flatIdx, e.target.value)}
        placeholder="Type your answer here..."
        className="min-h-28 w-full resize-none rounded-md border border-zinc-300 p-3 text-sm focus:border-[#4a90d9] focus:outline-none focus:ring-1 focus:ring-[#4a90d9]"
      />
    );
  }

  return (
    <p className="text-sm italic text-zinc-500">
      Loại câu hỏi này chưa hỗ trợ trong chế độ thi.
    </p>
  );
}

export function ExamQuestionLayout({
  group,
  groupStart,
  answers,
  onAnswer,
  muted = false,
  lockAudio = false,
  instantFeedback = false,
  readOnly = false,
  resultByIndex,
}: {
  group: ExamItem[];
  groupStart: number;
  answers: (string | null)[];
  onAnswer: (flatIndex: number, answer: string) => void;
  muted?: boolean;
  lockAudio?: boolean;
  instantFeedback?: boolean;
  readOnly?: boolean;
  resultByIndex?: Record<number, boolean>;
}) {
  const part = group[0]?.part ?? 1;
  const parent = group[0]?.parent;
  const firstQuestion = group[0]?.question;

  const paragraphText = parent?.paragraphText;
  const mediaQuestionText = parent?.questionText;
  const imageUrl = parent?.imageUrl ?? firstQuestion?.imageUrl;
  const audioUrl = parent?.audioUrl ?? firstQuestion?.audioUrl;
  const srtUrl = parent?.srtUrl ?? firstQuestion?.srtUrl;
  const groupKey = group[0]?.groupKey;

  const [audioBlocked, setAudioBlocked] = useState(false);
  const audioRef = useRef<HTMLAudioElement>(null);
  useEffect(() => {
    if (!lockAudio || !audioUrl) return;
    const el = audioRef.current;
    if (!el) return;
    setAudioBlocked(false);
    el.currentTime = 0;
    void el.play().catch(() => setAudioBlocked(true));
  }, [lockAudio, audioUrl, groupKey]);

  return (
    <div className="flex h-full w-full">
      <div className="w-1/2 shrink-0 space-y-4 overflow-y-auto border-r border-zinc-200 bg-[#fafbfc] p-6 [&::-webkit-scrollbar]:w-1.5 [&::-webkit-scrollbar-thumb]:rounded-full [&::-webkit-scrollbar-thumb]:bg-zinc-300">
        <p className="text-[13px] font-bold leading-relaxed text-[#1a56a0]">
          {PART_DIRECTIONS[part]}
        </p>
        {mediaQuestionText && (
          <div className="text-sm font-semibold text-zinc-800">
            <RichTextContent value={mediaQuestionText} />
          </div>
        )}
        {imageUrl && (
          <img
            src={imageUrl}
            alt="Hình minh họa"
            className="max-h-72 w-full rounded-lg border border-zinc-200 bg-white object-contain"
          />
        )}
        {paragraphText && (
          <div className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm">
            <RichTextContent value={paragraphText} className="text-sm leading-relaxed text-zinc-800" />
          </div>
        )}
        {audioUrl &&
          (lockAudio ? (
            <>
            {audioBlocked && <button type="button" className="rounded bg-blue-600 p-3 text-white" onClick={() => { void audioRef.current?.play().then(() => setAudioBlocked(false)).catch(() => setAudioBlocked(true)); }}>Phát / thử lại âm thanh</button>}
            <audio
              onError={() => setAudioBlocked(true)}
              key={groupKey ?? "audio"}
              ref={audioRef}
              src={audioUrl}
              preload="auto"
              className="hidden"
            />
            </>
          ) : (
            <CustomAudioPlayer src={audioUrl} srtUrl={srtUrl} muted={muted} />
          ))}
      </div>
      <div className="w-1/2 min-w-0 overflow-y-auto p-6 [&::-webkit-scrollbar]:w-1.5 [&::-webkit-scrollbar-thumb]:rounded-full [&::-webkit-scrollbar-thumb]:bg-zinc-300">
        <h2 className="mb-5 border-b border-zinc-200 pb-2 text-lg font-bold text-zinc-900">
          Question
        </h2>
        {group.map((item, gi) => {
          const flatIdx = groupStart + gi;
          const q = item.question;
          const selected = answers[flatIdx] ?? null;
          const freeform = q.questionType === "written" || (q.questionType === "multiple_choice" && !q.options?.length);
          return (
            <div
              key={item.key}
              className="mb-6 border-b border-zinc-100 pb-6 last:border-b-0 last:pb-0"
            >
              <div className="mb-3 flex items-baseline gap-2">
                <span className="shrink-0 text-sm font-bold text-[#1a56a0]">
                  {item.questionNumber}.
                </span>
                <RichTextContent
                  value={q.questionText}
                  className="text-[15px] font-semibold leading-relaxed text-zinc-900"
                />
              </div>
              <fieldset disabled={readOnly || (instantFeedback && selected != null)}>
                {!(freeform && instantFeedback && selected == null) && renderOptions(q, flatIdx, selected, onAnswer)}
              </fieldset>
              {!readOnly && freeform && instantFeedback && selected == null && <form onSubmit={(event) => { event.preventDefault(); const value = new FormData(event.currentTarget).get("answer"); if (typeof value === "string" && value.trim()) onAnswer(flatIdx, value); }}><input name="answer" aria-label="Câu trả lời" required className="rounded border p-2" /><button type="submit" className="rounded bg-blue-600 p-2 text-white">Kiểm tra</button></form>}
              {instantFeedback && selected != null && <div className="mt-3 rounded border bg-blue-50 p-3 text-sm">
                <strong>{(resultByIndex?.[flatIdx] ?? isAnswerCorrect(selected, q)) ? "✓ Chính xác" : "✕ Chưa đúng"}</strong>
                <p>Đáp án: {q.options?.find((option) => isAnswerCorrect(option.text, q))?.text ?? q.correctAnswer}</p>
                {q.answerExplanation && <RichTextContent value={q.answerExplanation} />}
              </div>}
            </div>
          );
        })}
        {group.length === 0 && <p className="text-sm text-zinc-500">Không có câu hỏi.</p>}
      </div>
    </div>
  );
}
