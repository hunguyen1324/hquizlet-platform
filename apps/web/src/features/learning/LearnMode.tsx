import React from "react";
import { useAuth } from "../auth/AuthContext";
import { quizApi, type QuizGeneratedItem } from "../../lib/api/client";
import type { Flashcard } from "./types";
import type { CardResult } from "./progressContract";
import { LearningEmptyState } from "../../components/learning/LearningEmptyState";
import { useProgressSave } from "./useProgressSave";
import { ProgressSaveStatus } from "./ProgressSaveStatus";
import { useQuizGeneration } from "./useQuizGeneration";
import "./learning.css";

type Props = { cards: Flashcard[]; studySetId: number };
type LearnItem = Pick<QuizGeneratedItem, "flashcardId" | "term" | "definition">;
type AnswerState = { submitted: string; correct: boolean; attempts: number; responseTimeMs: number };

export function LearnMode({ cards, studySetId }: Props) {
  const { token } = useAuth();
  const generation = useQuizGeneration(studySetId, "learn", Math.min(cards.length, 100));
  const { status: saveStatus, onSessionComplete, reset: resetSave } = useProgressSave({ studySetId, mode: "learn" });
  const [queue, setQueue] = React.useState<number[]>([]);
  const [answers, setAnswers] = React.useState<Record<number, AnswerState>>({});
  const [input, setInput] = React.useState("");
  const [feedback, setFeedback] = React.useState<boolean | null>(null);
  const [submitting, setSubmitting] = React.useState(false);
  const [done, setDone] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [startedAt, setStartedAt] = React.useState(() => new Date());
  const [questionStartedAt, setQuestionStartedAt] = React.useState(() => Date.now());
  const inputRef = React.useRef<HTMLInputElement>(null);
  const data = generation.state.state === "ready" ? generation.state.data : null;
  const items = React.useMemo(() => (data?.items ?? []).filter((x) => x.term !== undefined) as LearnItem[], [data]);

  React.useEffect(() => { if (items.length && queue.length === 0 && !done) setQueue(items.map((_, i) => i)); }, [items, queue.length, done]);
  const currentIndex = queue[0];
  const current = currentIndex === undefined ? undefined : items[currentIndex];

  async function submit() {
    if (!current || !input.trim() || submitting || feedback !== null || !data) return;
    setSubmitting(true); setError(null);
    const previous = answers[current.flashcardId];
    const attempts = (previous?.attempts ?? 0) + 1;
    try {
      const result = await quizApi.evaluate(token, studySetId, { mode: "learn", seed: data.seed, limit: items.length, answers: [{ flashcardId: current.flashcardId, submitted: input, attempts, responseTimeMs: Date.now() - questionStartedAt }] });
      const cardResult = result.cardResults[0];
      if (!cardResult) throw new Error("Backend không trả kết quả cho thẻ này");
      setAnswers((old) => ({ ...old, [current.flashcardId]: { submitted: input, correct: cardResult.correct, attempts, responseTimeMs: cardResult.responseTimeMs ?? 0 } }));
      setFeedback(cardResult.correct);
    } catch (e: unknown) { setError(e instanceof Error ? e.message : "Không thể chấm câu trả lời"); }
    finally { setSubmitting(false); }
  }

  function next() {
    if (!current || feedback === null) return;
    const nextQueue = feedback ? queue.slice(1) : [...queue.slice(1), currentIndex];
    setQueue(nextQueue); setInput(""); setFeedback(null); setQuestionStartedAt(Date.now());
    if (nextQueue.length === 0) finish(); else window.setTimeout(() => inputRef.current?.focus(), 0);
  }

  function finish() {
    const cardResults: CardResult[] = items.map((item) => { const answer = answers[item.flashcardId]; return { flashcardId: item.flashcardId, correct: answer?.correct ?? true, attempts: answer?.attempts ?? 1, responseTimeMs: answer?.responseTimeMs }; });
    setDone(true); onSessionComplete({ score: cardResults.filter((x) => x.correct).length, total: cardResults.length, cardResults, startedAt });
  }

  function restart() { resetSave(); setQueue([]); setAnswers({}); setInput(""); setFeedback(null); setDone(false); setError(null); setStartedAt(new Date()); setQuestionStartedAt(Date.now()); generation.regenerate(); }

  if (cards.length < 2) return <LearningEmptyState message="Cần ít nhất 2 thẻ để học." hint="Thêm thẻ trong phần 'Sửa thẻ'." />;
  if (generation.state.state === "loading") return <div className="learn-loading" role="status">Đang tạo bài học…</div>;
  if (generation.state.state === "error") return <div className="learn-error" role="alert">Không thể tạo Learn: {generation.state.error.message}<button className="secondary-button" onClick={generation.regenerate}>Thử lại</button></div>;
  if (!items.length) return <LearningEmptyState message="Backend không trả về câu hỏi hợp lệ." />;
  if (done) {
    const cardResults: CardResult[] = items.map((item) => ({ flashcardId: item.flashcardId, correct: answers[item.flashcardId]?.correct ?? true, attempts: answers[item.flashcardId]?.attempts ?? 1, responseTimeMs: answers[item.flashcardId]?.responseTimeMs }));
    return <div className="learn-done"><h2>🎉 Hoàn thành!</h2><p className="learn-score">Đúng <strong>{cardResults.filter((x) => x.correct).length}</strong> / {items.length} câu</p><ProgressSaveStatus status={saveStatus} onRetry={() => onSessionComplete({ score: cardResults.filter((x) => x.correct).length, total: items.length, cardResults, startedAt })} /><button className="primary-button" onClick={restart}>Học lại</button></div>;
  }
  if (!current) return <div className="learn-loading" role="status">Đang chuẩn bị câu hỏi…</div>;

  return <div className="learn-mode">
    <div className="learn-header"><span className="flashcards-counter">Còn {queue.length} thẻ</span><span>Seed: {data?.seed}</span></div>
    <div className="learn-card"><p className="learn-prompt-label">Thuật ngữ</p><p className="learn-prompt">{current.term}</p></div>
    <div className="learn-input-area"><label className="learn-input-label" htmlFor="learn-answer">Nhập định nghĩa</label><input id="learn-answer" ref={inputRef} className={`learn-input${feedback === null ? "" : feedback ? " input-correct" : " input-wrong"}`} value={input} onChange={(e) => setInput(e.target.value)} onKeyDown={(e) => { if (e.key === "Enter") feedback === null ? void submit() : next(); }} disabled={submitting || feedback !== null} autoFocus autoComplete="off" />
      {feedback !== null && <div className={`learn-feedback ${feedback ? "feedback-correct" : "feedback-wrong"}`}>{feedback ? "✅ Chính xác!" : <>❌ Chưa đúng. Đáp án: <strong>{current.definition}</strong></>}</div>}
      {error && <div className="learn-error" role="alert">{error}</div>}
    </div>
    <div className="learn-actions">{feedback === null ? <button className="primary-button" onClick={() => void submit()} disabled={!input.trim() || submitting}>{submitting ? "Đang chấm…" : "Kiểm tra"}</button> : <button className="primary-button" onClick={next}>Tiếp theo →</button>}</div>
  </div>;
}
