import { useMemo, useState } from "react";

import { contentProtection } from "./contentProtection";
import { QuestionNavigatorPanel } from "./QuestionNavigatorPanel";
import { RichTextContent } from "./RichTextContent";
import { PART_LABELS, isAnswerCorrect } from "./toeic";
import type { ExamItem, ExamResult } from "./types";

function formatTime(sec: number) {
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  const s = sec % 60;
  if (h > 0) return `${h} giờ ${m} phút`;
  return `${m} phút ${s} giây`;
}

export function ExamFinishScreen({
  items,
  answers,
  results,
  timeUsed,
  onRetry,
  onExit,
}: {
  items: ExamItem[];
  answers: (string | null)[];
  results: ExamResult[];
  timeUsed?: number;
  onRetry: () => void;
  onExit: () => void;
}) {
  const [selectedIndex, setSelectedIndex] = useState(() => {
    const firstWrong = results.findIndex((r) => !r.isCorrect);
    return firstWrong >= 0 ? firstWrong : 0;
  });

  const score = useMemo(() => results.filter((r) => r.isCorrect).length, [results]);
  const total = items.length;
  const percent = total > 0 ? Math.round((score / total) * 100) : 0;

  const breakdown = useMemo(() => {
    const map = new Map<number, { total: number; correct: number }>();
    items.forEach((item, i) => {
      const entry = map.get(item.part) ?? { total: 0, correct: 0 };
      entry.total += 1;
      if (results[i]?.isCorrect) entry.correct += 1;
      map.set(item.part, entry);
    });
    return [...map.entries()].sort((a, b) => a[0] - b[0]);
  }, [items, results]);

  const flaggedAll = useMemo(() => items.map(() => false), [items.length]);

  const selectedItem = items[selectedIndex];
  const selectedResult = results[selectedIndex];
  const selectedAnswer = answers[selectedIndex] ?? null;

  const renderReviewOptions = (item: ExamItem) => {
    const q = item.question;
    const correct = q.correctAnswer ?? "";

    if (q.questionType === "true_false") {
      const options = ["True", "False"];
      return (
        <div className="space-y-2">
          {options.map((opt) => {
            const value = opt.toLowerCase();
            const isCorrectOpt = isAnswerCorrect(value, q);
            const isUserOpt = selectedAnswer === value;
            const style = isCorrectOpt
              ? "border-emerald-500 bg-emerald-50 text-emerald-800"
              : isUserOpt
                ? "border-red-500 bg-red-50 text-red-700"
                : "border-zinc-200 text-zinc-600";
            return (
              <div key={opt} className={`flex items-center gap-3 rounded-md border px-3 py-2.5 text-sm ${style}`}>
                <span className="font-bold">{opt}</span>
                {isCorrectOpt && <span className="text-emerald-600">✓</span>}
                {isUserOpt && !isCorrectOpt && <span className="text-red-600">✗</span>}
              </div>
            );
          })}
        </div>
      );
    }

    const reviewOptions = q.options ?? [];
    if (reviewOptions.length > 0) {
      return (
        <div className="space-y-2">
          {reviewOptions.map((opt, optIdx) => {
            const letter = String.fromCharCode(65 + optIdx);
            const isCorrectOpt = isAnswerCorrect(opt.text, q);
            const isUserOpt = selectedAnswer === opt.text;
            const style = isCorrectOpt
              ? "border-emerald-500 bg-emerald-50"
              : isUserOpt
                ? "border-red-500 bg-red-50"
                : "border-zinc-200";
            const textColor = isCorrectOpt
              ? "text-emerald-800"
              : isUserOpt
                ? "text-red-700"
                : "text-zinc-600";
            return (
              <div
                key={opt.id ?? optIdx}
                className={`flex items-center gap-3 rounded-md border px-3 py-2.5 text-sm ${style} ${textColor}`}
              >
                <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full border border-current text-xs font-bold">
                  {letter}
                </span>
                <span className="min-w-0 flex-1">{opt.text}</span>
                {isCorrectOpt && <span className="text-emerald-600">✓</span>}
                {isUserOpt && !isCorrectOpt && <span className="text-red-600">✗</span>}
              </div>
            );
          })}
        </div>
      );
    }

    return (
      <div className="space-y-2 text-sm">
        <div className="rounded-md border border-zinc-200 bg-zinc-50 px-3 py-2.5">
          <span className="font-semibold text-zinc-500">Đáp án của bạn: </span>
          <span className={selectedAnswer ? "text-zinc-800" : "italic text-zinc-400"}>
            {selectedAnswer ?? "(Bỏ trống)"}
          </span>
        </div>
        <div className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2.5">
          <span className="font-semibold text-emerald-700">Đáp án đúng: </span>
          <span className="text-emerald-800">{correct}</span>
        </div>
      </div>
    );
  };

  return (
    <div {...contentProtection} className="fixed inset-0 z-[140] overflow-y-auto bg-[#f4f5f7]">
      <div className="mx-auto w-full max-w-6xl px-4 py-8">
        <div className="rounded-2xl border border-zinc-200 bg-white p-8 shadow-sm">
          <div className="flex flex-col items-center gap-6 md:flex-row md:justify-between">
            <div className="text-center md:text-left">
              <h1 className="text-2xl font-extrabold text-zinc-900">Nộp bài thành công!</h1>
              <p className="mt-1 text-sm text-zinc-500">
                {timeUsed !== undefined && <>Thời gian làm bài: {formatTime(timeUsed)} · </>}
                Tổng cộng {total} câu hỏi
              </p>
            </div>
            <div className="flex items-center gap-6">
              <div className="text-center">
                <div className="text-4xl font-black text-[#1a2744]">
                  {score}
                  <span className="text-lg font-bold text-zinc-400">/{total}</span>
                </div>
                <div className="text-xs font-semibold uppercase tracking-wide text-zinc-400">Điểm</div>
              </div>
              <div className="h-12 w-px bg-zinc-200" />
              <div className="text-center">
                <div
                  className={`text-4xl font-black ${
                    percent >= 70 ? "text-emerald-600" : percent >= 50 ? "text-amber-500" : "text-red-500"
                  }`}
                >
                  {percent}%
                </div>
                <div className="text-xs font-semibold uppercase tracking-wide text-zinc-400">Chính xác</div>
              </div>
            </div>
          </div>
          <div className="mt-6 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {breakdown.map(([part, entry]) => (
              <div key={part} className="rounded-xl border border-zinc-200 bg-[#fafbfc] px-4 py-3">
                <div className="mb-1.5 flex items-center justify-between text-xs">
                  <span className="font-bold text-[#1a56a0]">{PART_LABELS[part]}</span>
                  <span className="font-semibold tabular-nums text-zinc-600">
                    {entry.correct}/{entry.total}
                  </span>
                </div>
                <div className="h-1.5 w-full overflow-hidden rounded-full bg-zinc-200">
                  <div
                    className="h-full rounded-full bg-[#1a2744]"
                    style={{ width: `${(entry.correct / entry.total) * 100}%` }}
                  />
                </div>
              </div>
            ))}
          </div>
          <div className="mt-6 flex flex-col justify-center gap-3 sm:flex-row">
            <button
              type="button"
              className="rounded-lg border border-zinc-300 px-5 py-2.5 text-sm font-bold text-zinc-700 hover:bg-zinc-50"
              onClick={onExit}
            >
              ← Về chọn chế độ
            </button>
            <button
              type="button"
              className="rounded-lg bg-[#1a2744] px-5 py-2.5 text-sm font-bold text-white hover:bg-[#24345e]"
              onClick={onRetry}
            >
              Làm lại
            </button>
          </div>
        </div>
        <div className="mt-6 grid gap-6 lg:grid-cols-[300px_1fr]">
          <div className="h-[600px]">
            <QuestionNavigatorPanel
              items={items}
              currentIndex={selectedIndex}
              answers={answers}
              flagged={flaggedAll}
              results={results}
              onNavigate={setSelectedIndex}
              embedded
            />
          </div>
          <div className="overflow-hidden rounded-2xl border border-zinc-200 bg-white shadow-sm">
            {selectedItem && (
              <div className="max-h-[600px] overflow-y-auto p-6 [&::-webkit-scrollbar]:w-1.5 [&::-webkit-scrollbar-thumb]:rounded-full [&::-webkit-scrollbar-thumb]:bg-zinc-300">
                <div className="mb-4 flex items-center justify-between">
                  <h3 className="text-lg font-bold text-zinc-900">Câu {selectedItem.questionNumber}</h3>
                  <span
                    className={`rounded-full px-3 py-1 text-xs font-bold ${
                      selectedResult?.isCorrect
                        ? "bg-emerald-100 text-emerald-700"
                        : selectedAnswer
                          ? "bg-red-100 text-red-700"
                          : "bg-zinc-100 text-zinc-500"
                    }`}
                  >
                    {selectedResult?.isCorrect ? "✓ Đúng" : selectedAnswer ? "✗ Sai" : "— Bỏ qua"}
                  </span>
                </div>
                <div className="mb-4 rounded-lg border border-zinc-100 bg-[#fafbfc] p-4">
                  <RichTextContent
                    value={selectedItem.question.questionText}
                    className="text-[15px] font-semibold text-zinc-900"
                  />
                </div>
                {renderReviewOptions(selectedItem)}
                {selectedItem.question.answerExplanation && (
                  <div className="mt-5 border-t border-zinc-100 pt-4">
                    <h4 className="mb-2 text-xs font-bold uppercase tracking-wide text-zinc-400">Giải thích</h4>
                    <RichTextContent
                      value={selectedItem.question.answerExplanation}
                      className="text-sm leading-relaxed text-zinc-700"
                    />
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
