// LearnMode — Dev 4
// P2-LEARN-02: Hỏi đáp, scoring local, retry wrong answers
// P3-LEARN-01,02,03: Nối completion với saveProgress thật; per-card results; chống double-submit
//
// Phase 3 changes:
//   - useProgressSave hook injected via props (studySetId required)
//   - onSessionComplete gọi khi phase="done", truyền per-card CardResult[]
//   - ProgressSaveStatus hiển thị ở màn done
//   - Restart reset cả save status

import React from "react";
import type { Flashcard, LearnState } from "./types";
import type { CardResult } from "./progressContract";
import { LearningEmptyState } from "../../components/learning/LearningEmptyState";
import { useProgressSave } from "./useProgressSave";
import { ProgressSaveStatus } from "./ProgressSaveStatus";
import "././learning.css";

type Props = {
  cards: Flashcard[];
  studySetId: number;
};

type Phase = "quiz" | "retry" | "done";

function buildQueue(cards: Flashcard[]): Flashcard[] { return [...cards]; }

export function LearnMode({ cards, studySetId }: Props) {
  const [startedAt, setStartedAt] = React.useState(() => new Date());
  const [state, setState] = React.useState<LearnState>(() => ({
    queue: buildQueue(cards), currentIndex: 0, answers: {}, done: false,
  }));
  const [input, setInput] = React.useState("");
  const [submitted, setSubmitted] = React.useState(false);
  const [correct, setCorrect] = React.useState<boolean | null>(null);
  const [phase, setPhase] = React.useState<Phase>("quiz");
  const [retryQueue, setRetryQueue] = React.useState<Flashcard[]>([]);
  const [retryIndex, setRetryIndex] = React.useState(0);
  const inputRef = React.useRef<HTMLInputElement>(null);

  const { status: saveStatus, onSessionComplete, reset: resetSave } = useProgressSave({
    studySetId,
    mode: "learn",
  });

  const current = phase === "retry" ? retryQueue[retryIndex] : state.queue[state.currentIndex];
  const total = phase === "retry" ? retryQueue.length : state.queue.length;
  const currentIdx = phase === "retry" ? retryIndex : state.currentIndex;

  function normalize(s: string) { return s.trim().toLowerCase().replace(/\s+/g, " "); }

  function handleSubmit() {
    if (!current || !input.trim() || submitted) return;
    const isCorrect = normalize(input) === normalize(current.definition);
    setCorrect(isCorrect);
    setSubmitted(true);
    setState((s) => ({
      ...s,
      answers: {
        ...s.answers,
        [current.id]: { card: current, userAnswer: input, submitted: true, correct: isCorrect },
      },
    }));
  }

  function handleNext() {
    const nextIdx = currentIdx + 1;
    if (phase === "quiz") {
      if (nextIdx >= total) {
        const finalAnswers = {
          ...state.answers,
          [current.id]: { card: current, userAnswer: input, submitted: true, correct: correct ?? false },
        };
        const wrongs = state.queue.filter((card) => !finalAnswers[card.id]?.correct);
        if (wrongs.length > 0) {
          setRetryQueue(wrongs); setRetryIndex(0); setPhase("retry");
          setState((s) => ({ ...s, answers: finalAnswers }));
        } else {
          setState((s) => ({ ...s, answers: finalAnswers, done: true }));
          setPhase("done");
          // Build CardResult list for all cards.
          const cardResults: CardResult[] = state.queue.map((card) => {
            const ans = finalAnswers[card.id];
            return {
              flashcardId: card.id,
              correct: ans?.correct ?? false,
              attempts: 1,
            };
          });
          const correctCount = cardResults.filter((r) => r.correct).length;
          onSessionComplete({
            score: correctCount,
            total: state.queue.length,
            cardResults,
            startedAt,
          });
        }
      } else {
        setState((s) => ({ ...s, currentIndex: nextIdx }));
      }
    } else if (nextIdx >= total) {
      const finalAnswers = {
        ...state.answers,
        [current.id]: { card: current, userAnswer: input, submitted: true, correct: correct ?? false },
      };
      setPhase("done");
      setState((s) => ({ ...s, answers: finalAnswers, done: true }));
      // Build per-card results including retry-corrected cards.
      const cardResults: CardResult[] = state.queue.map((card) => {
        const ans = finalAnswers[card.id];
        return {
          flashcardId: card.id,
          correct: ans?.correct ?? false,
          attempts: retryQueue.some((r) => r.id === card.id) ? 2 : 1,
        };
      });
      const correctCount = cardResults.filter((r) => r.correct).length;
      onSessionComplete({
        score: correctCount,
        total: state.queue.length,
        cardResults,
        startedAt,
      });
    } else {
      setRetryIndex(nextIdx);
    }
    setInput(""); setSubmitted(false); setCorrect(null);
    setTimeout(() => inputRef.current?.focus(), 50);
  }

  function handleRestart() {
    resetSave();
    setStartedAt(new Date());
    setState({ queue: buildQueue(cards), currentIndex: 0, answers: {}, done: false });
    setRetryQueue([]); setRetryIndex(0); setPhase("quiz"); setInput(""); setSubmitted(false); setCorrect(null);
    setTimeout(() => inputRef.current?.focus(), 50);
  }

  if (cards.length < 2) {
    return <LearningEmptyState message="Cần ít nhất 2 thẻ để học." hint="Thêm thẻ trong phần 'Sửa thẻ' trước khi học." />;
  }

  if (phase === "done") {
    const correctCount = Object.values(state.answers).filter((a) => a.correct).length;
    return (
      <div className="learn-done">
        <h2>🎉 Hoàn thành!</h2>
        <p className="learn-score">
          Đúng <strong>{correctCount}</strong> / {state.queue.length} câu
          {retryQueue.length > 0 && <span className="retry-note"> (đã ôn {retryQueue.length} câu sai)</span>}
        </p>
        <ProgressSaveStatus status={saveStatus} onRetry={() => {
          const cardResults: CardResult[] = state.queue.map((card) => ({
            flashcardId: card.id,
            correct: state.answers[card.id]?.correct ?? false,
            attempts: 1,
          }));
          onSessionComplete({ score: correctCount, total: state.queue.length, cardResults, startedAt });
        }} />
        <div className="learn-review">
          <div className="review-header"><span>Thuật ngữ</span><span>Câu trả lời</span><span>Đáp án đúng</span></div>
          {state.queue.map((card) => {
            const ans = state.answers[card.id];
            return (
              <div key={card.id} className={`review-row ${ans?.correct ? "correct" : "wrong"}`}>
                <span className="review-term">{card.term}</span>
                <span className="review-answer">{ans?.userAnswer || "—"}</span>
                <span className="review-correct">{card.definition}</span>
              </div>
            );
          })}
        </div>
        <button className="primary-button" onClick={handleRestart}>Học lại</button>
      </div>
    );
  }

  const isRetry = phase === "retry";
  return (
    <div className="learn-mode">
      <div className="learn-header">
        <span className="flashcards-counter">
          {currentIdx + 1} / {total}
          {isRetry && <span className="retry-badge"> Ôn lại</span>}
        </span>
        {isRetry && <span className="retry-label">Đang ôn lại {retryQueue.length} câu sai</span>}
      </div>
      <div className="learn-card">
        <p className="learn-prompt-label">Thuật ngữ</p>
        <p className="learn-prompt">{current.term}</p>
      </div>
      <div className="learn-input-area">
        <label className="learn-input-label" htmlFor="learn-answer">Nhập định nghĩa</label>
        <input
          id="learn-answer"
          ref={inputRef}
          className={`learn-input${submitted ? (correct ? " input-correct" : " input-wrong") : ""}`}
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => { if (e.key === "Enter") submitted ? handleNext() : handleSubmit(); }}
          placeholder="Nhập câu trả lời..."
          disabled={submitted}
          autoFocus
          autoComplete="off"
        />
        {submitted && (
          <div className={`learn-feedback ${correct ? "feedback-correct" : "feedback-wrong"}`}>
            {correct ? <span>✅ Chính xác!</span> : <span>❌ Đáp án đúng: <strong>{current.definition}</strong></span>}
          </div>
        )}
      </div>
      <div className="learn-actions">
        {!submitted
          ? <button className="primary-button" onClick={handleSubmit} disabled={!input.trim()}>Kiểm tra</button>
          : <button className="primary-button" onClick={handleNext}>Tiếp theo →</button>
        }
      </div>
      <div className="progress-bar-track" role="progressbar" aria-valuenow={currentIdx + 1} aria-valuemax={total}>
        <div className="progress-bar-fill" style={{ width: `${((currentIdx + 1) / total) * 100}%` }} />
      </div>
      <p className="keyboard-hint" aria-hidden="true">Enter để kiểm tra / chuyển câu</p>
    </div>
  );
}
