// LearnMode — Dev 4
// P2-LEARN-02: Hỏi đáp, scoring local, retry wrong answers
// Phase 2: retry sai, score hiển thị, empty state < 2 cards, keyboard nav

import React from "react";
import type { Flashcard, LearnState } from "./types";
import { LearningEmptyState } from "../../components/learning/LearningEmptyState";
import "./learning.css";

type Props = {
  cards: Flashcard[];
};

function buildQueue(cards: Flashcard[]): Flashcard[] {
  return [...cards];
}

type Phase = "quiz" | "retry" | "done";

export function LearnMode({ cards }: Props) {
  const [state, setState] = React.useState<LearnState>(() => ({
    queue: buildQueue(cards),
    currentIndex: 0,
    answers: {},
    done: false,
  }));
  const [input, setInput] = React.useState("");
  const [submitted, setSubmitted] = React.useState(false);
  const [correct, setCorrect] = React.useState<boolean | null>(null);
  const [phase, setPhase] = React.useState<Phase>("quiz");
  const [retryQueue, setRetryQueue] = React.useState<Flashcard[]>([]);
  const [retryIndex, setRetryIndex] = React.useState(0);
  const inputRef = React.useRef<HTMLInputElement>(null);

  if (cards.length < 2) {
    return (
      <LearningEmptyState
        message="Cần ít nhất 2 thẻ để học."
        hint="Thêm thẻ trong phần 'Sửa thẻ' trước khi học."
      />
    );
  }

  // ── Quiz phase ──
  const current = phase === "retry" ? retryQueue[retryIndex] : state.queue[state.currentIndex];
  const total = phase === "retry" ? retryQueue.length : state.queue.length;
  const currentIdx = phase === "retry" ? retryIndex : state.currentIndex;

  function normalize(s: string) {
    return s.trim().toLowerCase().replace(/\s+/g, " ");
  }

  function handleSubmit() {
    if (!input.trim() || submitted) return;
    const isCorrect = normalize(input) === normalize(current.definition);
    setCorrect(isCorrect);
    setSubmitted(true);

    if (phase === "quiz") {
      setState((s) => ({
        ...s,
        answers: {
          ...s.answers,
          [current.id]: {
            card: current,
            userAnswer: input,
            submitted: true,
            correct: isCorrect,
          },
        },
      }));
    }
  }

  function handleNext() {
    const nextIdx = currentIdx + 1;

    if (phase === "quiz") {
      if (nextIdx >= total) {
        // Build retry queue from wrong answers
        const wrongs = state.queue.filter((c) => {
          const ans = state.answers[c.id];
          // include unanswered too (shouldn't happen but safe)
          return !ans || !ans.correct;
        }).filter((c) => {
          const ans = { ...state.answers };
          // Re-check with current answer just submitted
          if (c.id === current.id) return !correct;
          return ans[c.id] && !ans[c.id].correct;
        });

        if (wrongs.length > 0) {
          setRetryQueue(wrongs);
          setRetryIndex(0);
          setPhase("retry");
        } else {
          setState((s) => ({ ...s, done: true }));
          setPhase("done");
        }
      } else {
        setState((s) => ({ ...s, currentIndex: nextIdx }));
      }
    } else {
      // retry phase
      if (nextIdx >= total) {
        setPhase("done");
        setState((s) => ({ ...s, done: true }));
      } else {
        setRetryIndex(nextIdx);
      }
    }

    setInput("");
    setSubmitted(false);
    setCorrect(null);
    setTimeout(() => inputRef.current?.focus(), 50);
  }

  function handleRestart() {
    setState({ queue: buildQueue(cards), currentIndex: 0, answers: {}, done: false });
    setRetryQueue([]);
    setRetryIndex(0);
    setPhase("quiz");
    setInput("");
    setSubmitted(false);
    setCorrect(null);
    setTimeout(() => inputRef.current?.focus(), 50);
  }

  // Done screen
  if (phase === "done") {
    const allAnswers = Object.values(state.answers);
    const correctCount = allAnswers.filter((a) => a.correct).length;
    const total2 = state.queue.length;

    return (
      <div className="learn-done">
        <h2>🎉 Hoàn thành!</h2>
        <p className="learn-score">
          Đúng <strong>{correctCount}</strong> / {total2} câu
          {retryQueue.length > 0 && <span className="retry-note"> (đã ôn {retryQueue.length} câu sai)</span>}
        </p>
        <div className="learn-review">
          <div className="review-header">
            <span>Thuật ngữ</span>
            <span>Câu trả lời</span>
            <span>Đáp án đúng</span>
          </div>
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
        <button className="primary-button" onClick={handleRestart}>
          Học lại
        </button>
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
        {isRetry && (
          <span className="retry-label">Đang ôn lại {retryQueue.length} câu sai</span>
        )}
      </div>

      <div className="learn-card">
        <p className="learn-prompt-label">Thuật ngữ</p>
        <p className="learn-prompt">{current.term}</p>
      </div>

      <div className="learn-input-area">
        <label className="learn-input-label" htmlFor="learn-answer">
          Nhập định nghĩa
        </label>
        <input
          id="learn-answer"
          ref={inputRef}
          className={`learn-input${submitted ? (correct ? " input-correct" : " input-wrong") : ""}`}
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") submitted ? handleNext() : handleSubmit();
          }}
          placeholder="Nhập câu trả lời..."
          disabled={submitted}
          autoFocus
          autoComplete="off"
        />

        {submitted && (
          <div className={`learn-feedback ${correct ? "feedback-correct" : "feedback-wrong"}`}>
            {correct ? (
              <span>✅ Chính xác!</span>
            ) : (
              <span>
                ❌ Đáp án đúng: <strong>{current.definition}</strong>
              </span>
            )}
          </div>
        )}
      </div>

      <div className="learn-actions">
        {!submitted ? (
          <button className="primary-button" onClick={handleSubmit} disabled={!input.trim()}>
            Kiểm tra
          </button>
        ) : (
          <button className="primary-button" onClick={handleNext}>
            {currentIdx + 1 >= total ? (isRetry ? "Xem kết quả" : "Xem kết quả") : "Tiếp theo →"}
          </button>
        )}
      </div>

      {/* Progress */}
      <div
        className="progress-bar-track"
        role="progressbar"
        aria-valuenow={currentIdx + 1}
        aria-valuemax={total}
      >
        <div
          className="progress-bar-fill"
          style={{ width: `${((currentIdx + 1) / total) * 100}%` }}
        />
      </div>

      <p className="keyboard-hint" aria-hidden="true">
        Enter để kiểm tra / chuyển câu
      </p>
    </div>
  );
}
