import { useCallback, useEffect, useMemo, useState } from "react";

import { contentProtection } from "./contentProtection";
import { ExamBottomBar } from "./ExamBottomBar";
import { ExamFinishScreen } from "./ExamFinishScreen";
import { ExamQuestionLayout } from "./ExamQuestionLayout";
import { ExamTopBar } from "./ExamTopBar";
import { QuestionNavigatorPanel } from "./QuestionNavigatorPanel";
import { buildExamItems, isAnswerCorrect, isListeningPart } from "./toeic";
import type { ExamResult, QuizQuestionLike } from "./types";

const EXAM_DURATION_SECONDS = 120 * 60;

export function ExamGame({
  questions,
  timerEnabled = true,
  durationSeconds = EXAM_DURATION_SECONDS,
  selectedPart = "all",
  instantFeedback = false,
  lockAudio = false,
  onExit,
}: {
  questions: QuizQuestionLike[];
  timerEnabled?: boolean;
  durationSeconds?: number;
  selectedPart?: number | "all";
  instantFeedback?: boolean;
  lockAudio?: boolean;
  onExit: () => void;
}) {
  const items = useMemo(() => buildExamItems(questions).filter((item) => selectedPart === "all" || item.part === selectedPart), [questions, selectedPart]);

  const [answers, setAnswers] = useState<(string | null)[]>(() => items.map(() => null));
  const [flagged, setFlagged] = useState<boolean[]>(() => items.map(() => false));
  const [currentIndex, setCurrentIndex] = useState(0);
  const [timeLeft, setTimeLeft] = useState(timerEnabled ? durationSeconds : 0);
  const [deadline, setDeadline] = useState(() => Date.now() + durationSeconds * 1000);
  const [isSubmitted, setIsSubmitted] = useState(false);
  const [isNavigatorOpen, setIsNavigatorOpen] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);
  const [isMuted, setIsMuted] = useState(false);

  useEffect(() => {
    setAnswers(items.map(() => null));
    setFlagged(items.map(() => false));
    setCurrentIndex(0);
    setIsSubmitted(false);
  }, [items]);

  const answeredCount = useMemo(
    () => answers.filter((a) => a != null && a.trim() !== "").length,
    [answers],
  );

  useEffect(() => {
    if (!timerEnabled || isSubmitted) return;
    const tick = () => {
      const remaining = Math.max(0, Math.ceil((deadline - Date.now()) / 1000));
      setTimeLeft(remaining);
      if (remaining === 0) setIsSubmitted(true);
    };
    tick();
    const timer = window.setInterval(tick, 250);
    return () => window.clearInterval(timer);
  }, [timerEnabled, isSubmitted, deadline]);

  const currentItem = items[currentIndex];

  const { group, groupStart } = useMemo(() => {
    if (!currentItem) return { group: [] as typeof items, groupStart: 0 };
    const start = items.findIndex((it) => it.groupKey === currentItem.groupKey);
    const nextDiff = items.findIndex(
      (it, i) => i > start && it.groupKey !== currentItem.groupKey,
    );
    return {
      group: items.slice(start, nextDiff === -1 ? undefined : nextDiff),
      groupStart: start,
    };
  }, [items, currentItem]);

  const handleAnswer = useCallback((flatIndex: number, answer: string) => {
    setAnswers((prev) => {
      const next = [...prev];
      if (instantFeedback && prev[flatIndex] != null) return prev;
      next[flatIndex] = answer;
      return next;
    });
  }, [instantFeedback]);

  const toggleFlag = useCallback(() => {
    setFlagged((prev) => {
      const next = [...prev];
      next[currentIndex] = !next[currentIndex];
      return next;
    });
  }, [currentIndex]);

  const goPrev = useCallback(() => setCurrentIndex((i) => Math.max(0, i - 1)), []);
  const goNext = useCallback(() => setCurrentIndex((i) => Math.min(items.length - 1, i + 1)), [items.length]);
  const navigate = useCallback((index: number) => {
    setCurrentIndex(index);
    setIsNavigatorOpen(false);
  }, []);

  const submit = useCallback(() => {
    setShowConfirm(false);
    setIsSubmitted(true);
    setIsNavigatorOpen(false);
  }, []);

  const retry = useCallback(() => {
    setAnswers(items.map(() => null));
    setFlagged(items.map(() => false));
    setCurrentIndex(0);
    setDeadline(Date.now() + durationSeconds * 1000);
    setTimeLeft(timerEnabled ? durationSeconds : 0);
    setIsSubmitted(false);
    setIsNavigatorOpen(false);
    window.scrollTo({ top: 0 });
  }, [items, timerEnabled, durationSeconds]);

  const currentPartLabel = currentItem
    ? isListeningPart(currentItem.part)
      ? "Listening"
      : "Reading"
    : "Listening";

  const questionRange = useMemo(() => {
    if (!currentItem) return "1";
    const nums = items.filter((it) => it.part === currentItem.part).map((it) => it.questionNumber);
    if (nums.length === 0) return String(currentItem.questionNumber);
    const min = Math.min(...nums);
    const max = Math.max(...nums);
    return min === max ? String(min) : `${min} - ${max}`;
  }, [items, currentItem]);

  const results: ExamResult[] | undefined = useMemo(() => {
    if (!isSubmitted) return undefined;
    return items.map((it, i) => ({
      questionIndex: i,
      selectedAnswer: answers[i] ?? null,
      correctAnswer: it.question.correctAnswer ?? "",
      isCorrect: isAnswerCorrect(answers[i] ?? null, it.question),
    }));
  }, [isSubmitted, items, answers]);

  if (isSubmitted && results) {
    return (
      <ExamFinishScreen
        items={items}
        answers={answers}
        results={results}
        timeUsed={timerEnabled ? durationSeconds - timeLeft : undefined}
        onRetry={retry}
        onExit={onExit}
      />
    );
  }

  const unansweredCount = items.length - answeredCount;

  return (
    <div {...contentProtection} className="fixed inset-0 z-[130] flex flex-col bg-white">
      <button type="button" className="bg-[#1a2744] px-4 py-1 text-left text-sm text-white" onClick={onExit}>← Về chọn chế độ</button>
      <ExamTopBar
        currentPart={currentPartLabel}
        questionRange={questionRange}
        totalQuestions={items.length}
        answeredCount={answeredCount}
        timeLeft={timeLeft}
        showTimer={timerEnabled}
        muted={lockAudio ? false : isMuted}
        onVolumeChange={lockAudio ? undefined : () => setIsMuted((m) => !m)}
        onSubmit={() => setShowConfirm(true)}
      />
      <div className="min-h-0 flex-1 overflow-hidden">
        <ExamQuestionLayout
          group={group}
          groupStart={groupStart}
          answers={answers}
          onAnswer={handleAnswer}
          muted={isMuted}
          lockAudio={lockAudio}
          instantFeedback={instantFeedback}
        />
      </div>
      <ExamBottomBar
        isMarked={flagged[currentIndex] ?? false}
        onToggleMark={toggleFlag}
        onOpenNavigator={() => setIsNavigatorOpen(true)}
        onPrev={goPrev}
        onNext={goNext}
        canPrev={currentIndex > 0}
        canNext={currentIndex < items.length - 1}
      />
      <QuestionNavigatorPanel
        items={items}
        currentIndex={currentIndex}
        answers={answers}
        flagged={flagged}
        onNavigate={navigate}
        isOpen={isNavigatorOpen}
        onClose={() => setIsNavigatorOpen(false)}
      />
      {showConfirm && (
        <div
          className="fixed inset-0 z-[170] flex items-center justify-center bg-black/50 p-4"
          onClick={() => setShowConfirm(false)}
        >
          <div
            className="w-full max-w-md rounded-2xl bg-white p-6 shadow-2xl"
            onClick={(e) => e.stopPropagation()}
          >
            <h3 className="text-xl font-bold text-zinc-900">Nộp bài?</h3>
            <p className="mt-2 text-sm leading-relaxed text-zinc-600">
              Bạn đã trả lời{" "}
              <strong className="text-[#1a2744]">
                {answeredCount}/{items.length}
              </strong>{" "}
              câu
              {unansweredCount > 0 && (
                <>
                  , còn <strong className="text-amber-600">{unansweredCount} câu</strong> chưa trả lời
                </>
              )}
              . Sau khi nộp bài, bạn không thể thay đổi đáp án.
            </p>
            <div className="mt-6 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setShowConfirm(false)}
                className="rounded-lg border border-zinc-300 px-4 py-2 text-sm font-semibold text-zinc-700 transition-colors hover:bg-zinc-50"
              >
                Tiếp tục làm bài
              </button>
              <button
                type="button"
                onClick={submit}
                className="rounded-lg bg-[#ff8c00] px-5 py-2 text-sm font-bold text-white transition-colors hover:bg-[#e67e00]"
              >
                Nộp bài
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
