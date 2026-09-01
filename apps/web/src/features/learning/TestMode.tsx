// TestMode — Dev 5
// Phase 4: questions, distractors and scoring come from quiz service.
// The client never compares the selected answer with the correct answer.
import React from "react";
import { useAuth } from "../auth/AuthContext";
import { quizApi, type QuizAnswer, type QuizGeneratedItem } from "../../lib/api/client";
import type { Flashcard } from "./types";
import type { CardResult } from "./progressContract";
import { LearningEmptyState } from "../../components/learning/LearningEmptyState";
import { useProgressSave } from "./useProgressSave";
import { ProgressSaveStatus } from "./ProgressSaveStatus";
import { useQuizGeneration } from "./useQuizGeneration";
import "./learning.css";

type Props = { cards: Flashcard[]; studySetId: number };
type AnswerState = { answer: string; attempts: number; responseTimeMs: number };

export function TestMode({ cards, studySetId }: Props) {
  const { token } = useAuth();
  const generation = useQuizGeneration(studySetId, "test", cards.length);
  const { status: saveStatus, onSessionComplete, reset: resetSave } = useProgressSave({ studySetId, mode: "test" });
  const [index, setIndex] = React.useState(0);
  const [answers, setAnswers] = React.useState<Record<number, AnswerState>>({});
  const [submitted, setSubmitted] = React.useState(false);
  const [result, setResult] = React.useState<{ score: number; total: number; cardResults: CardResult[] } | null>(null);
  const [startedAt, setStartedAt] = React.useState(() => new Date());
  const [questionStartedAt, setQuestionStartedAt] = React.useState(() => Date.now());
  const [error, setError] = React.useState<string | null>(null);
  const data = generation.state.state === "ready" ? generation.state.data : null;
  const questions = React.useMemo(() => toQuestions(data?.items ?? []), [data]);
  const q = questions[index];

  if (cards.length < 2) return <LearningEmptyState message="Cần ít nhất 2 thẻ để làm bài kiểm tra." hint="Thêm thẻ trong phần 'Sửa thẻ'." />;
  if (generation.state.state === "loading") return <div className="learn-loading" role="status">Đang tạo bài kiểm tra…</div>;
  if (generation.state.state === "error") return <div className="learn-error" role="alert">Không thể tạo Test: {generation.state.error.message}<button className="secondary-button" onClick={generation.regenerate}>Thử lại</button></div>;
  if (!q || !data) return <LearningEmptyState message="Backend không trả về câu hỏi hợp lệ." hint="Thử lại để tạo một bài kiểm tra mới." />;
  const seed = data.seed;
  async function submitCurrent() {
    const current = answers[q.flashcardId];
    if (!current || submitted) return;
    const nextAnswers: Record<number, AnswerState> = { ...answers, [q.flashcardId]: current };
    setAnswers(nextAnswers);
    if (index + 1 < questions.length) {
      setIndex((i) => i + 1);
      setQuestionStartedAt(Date.now());
      return;
    }
    setSubmitted(true);
    setError(null);
    const payload: QuizAnswer[] = questions.map((item) => ({ flashcardId: item.flashcardId, answer: nextAnswers[item.flashcardId]?.answer ?? "", attempts: nextAnswers[item.flashcardId]?.attempts ?? 1, responseTimeMs: nextAnswers[item.flashcardId]?.responseTimeMs ?? 0 }));
    try {
      const evaluated = await quizApi.evaluate(token, studySetId, {
        mode: "test",
        seed,
        answers: payload,
      });
      const cardResults = evaluated.cardResults as CardResult[];
      setResult({ score: evaluated.score, total: evaluated.total, cardResults });
      onSessionComplete({ score: evaluated.score, total: evaluated.total, cardResults, startedAt });
    } catch (e: unknown) {
      setSubmitted(false);
      setError(e instanceof Error ? e.message : "Không thể chấm bài");
    }
  }

  function choose(choice: string) {
    if (answers[q.flashcardId] || submitted) return;
    setAnswers((current) => ({ ...current, [q.flashcardId]: { answer: choice, attempts: 1, responseTimeMs: Date.now() - questionStartedAt } }));
  }

  function restart() {
    resetSave(); setIndex(0); setAnswers({}); setSubmitted(false); setResult(null); setStartedAt(new Date()); setQuestionStartedAt(Date.now()); setError(null); generation.regenerate();
  }

  if (result) {
    const pct = result.total ? Math.round((result.score / result.total) * 100) : 0;
    return <div className="learn-done"><h2>📝 Kết quả bài kiểm tra</h2><p className="learn-score"><strong>{result.score}</strong> / {result.total} ({pct}%)</p>{error && <p className="learn-error">{error}</p>}<ProgressSaveStatus status={saveStatus} onRetry={() => onSessionComplete({ score: result.score, total: result.total, cardResults: result.cardResults, startedAt })} /><div className="learn-review">{questions.map((item) => { const r = result.cardResults.find((x) => x.flashcardId === item.flashcardId); return <div key={item.flashcardId} className={`review-row ${r?.correct ? "correct" : "wrong"}`}><span className="review-term">{item.term}</span><span className="review-answer">{answers[item.flashcardId]?.answer || "—"}</span><span className="review-correct">{r?.correct ? "✓ Correct" : "✗ Incorrect"}</span></div>; })}</div><button className="primary-button" onClick={restart}>Làm lại</button></div>;
  }

  const selected = answers[q.flashcardId];
  return <div className="test-mode">
    <div className="learn-header"><span className="flashcards-counter">{index + 1} / {questions.length}</span><span>Seed: {seed}</span></div>
    <div className="test-question"><p className="test-question-label">Chọn định nghĩa đúng:</p><p className="test-term">{q.term}</p></div>
    <div className="test-choices">
      {q.choices.map((choice, i) => <button key={`${choice}-${i}`} className={`test-choice${selected?.answer === choice ? " selected" : ""}`} onClick={() => choose(choice)} disabled={Boolean(selected) || submitted} aria-label={`Đáp án ${String.fromCharCode(65 + i)}: ${choice}`}><span className="choice-letter">{String.fromCharCode(65 + i)}</span>{choice}</button>)}
    </div>
    {selected && <div className="learn-actions"><button className="primary-button" onClick={submitCurrent} disabled={submitted}>{index + 1 >= questions.length ? "Nộp bài" : "Câu tiếp →"}</button></div>}
    {error && <div className="learn-error" role="alert">{error}</div>}
    <div className="progress-bar-track" role="progressbar" aria-valuenow={index + 1} aria-valuemax={questions.length}><div className="progress-bar-fill" style={{ width: `${((index + 1) / questions.length) * 100}%` }} /></div>
    <p className="keyboard-hint" aria-hidden="true">Nhấn 1–{Math.min(q.choices.length, 4)} để chọn đáp án</p>
  </div>;
}

function toQuestions(items: QuizGeneratedItem[]) {
  return items.filter((item) => item.kind === "question" || item.choices).map((item) => ({ flashcardId: item.flashcardId, term: item.term ?? item.text ?? "", choices: item.choices ?? [], id: item.id }));
}
