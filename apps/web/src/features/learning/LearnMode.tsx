// LearnMode — Dev 4
// FE-LEARN-03: Hỏi đáp, check answer local (no backend needed in Sprint 1)

import React from "react";
import type { Flashcard, LearnState } from "./types";
import "./learning.css";

type Props = {
  cards: Flashcard[];
};

function buildQueue(cards: Flashcard[]): Flashcard[] {
  return [...cards];
}

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

  const current = state.queue[state.currentIndex];
  const total = state.queue.length;

  function handleSubmit() {
    if (!input.trim() || submitted) return;
    const isCorrect =
      input.trim().toLowerCase() === current.definition.trim().toLowerCase();
    setCorrect(isCorrect);
    setSubmitted(true);
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

  function handleNext() {
    const nextIndex = state.currentIndex + 1;
    if (nextIndex >= total) {
      setState((s) => ({ ...s, done: true }));
    } else {
      setState((s) => ({ ...s, currentIndex: nextIndex }));
    }
    setInput("");
    setSubmitted(false);
    setCorrect(null);
  }

  function handleRestart() {
    setState({ queue: buildQueue(cards), currentIndex: 0, answers: {}, done: false });
    setInput("");
    setSubmitted(false);
    setCorrect(null);
  }

  if (total === 0) {
    return <div className="learning-empty"><p>Chưa có thẻ nào.</p></div>;
  }

  if (state.done) {
    const correctCount = Object.values(state.answers).filter((a) => a.correct).length;
    return (
      <div className="learn-done">
        <h2>🎉 Hoàn thành!</h2>
        <p className="learn-score">
          Đúng <strong>{correctCount}</strong> / {total} câu
        </p>
        <div className="learn-review">
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

  return (
    <div className="learn-mode">
      <div className="learn-header">
        <span className="flashcards-counter">{state.currentIndex + 1} / {total}</span>
      </div>

      <div className="learn-card">
        <p className="learn-prompt-label">Thuật ngữ</p>
        <p className="learn-prompt">{current.term}</p>
      </div>

      <div className="learn-input-area">
        <label className="learn-input-label">Nhập định nghĩa</label>
        <input
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
            {state.currentIndex + 1 >= total ? "Xem kết quả" : "Tiếp theo →"}
          </button>
        )}
      </div>

      {/* Mini progress */}
      <div className="progress-bar-track">
        <div
          className="progress-bar-fill"
          style={{ width: `${((state.currentIndex + 1) / total) * 100}%` }}
        />
      </div>
    </div>
  );
}
