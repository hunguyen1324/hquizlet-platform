// TestMode — Dev 4
// FE-LEARN-04: Trắc nghiệm từ flashcards, check answer local (Sprint 1)

import React from "react";
import type { Flashcard, TestState, TestQuestion } from "./types";
import "./learning.css";

type Props = {
  cards: Flashcard[];
};

function shuffleArray<T>(arr: T[]): T[] {
  const a = [...arr];
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [a[i], a[j]] = [a[j], a[i]];
  }
  return a;
}

function buildQuestions(cards: Flashcard[]): TestQuestion[] {
  const allDefs = cards.map((c) => c.definition);
  return shuffleArray(cards).map((card) => {
    const distractors = shuffleArray(
      allDefs.filter((d) => d !== card.definition)
    ).slice(0, 3);
    const choices = shuffleArray([card.definition, ...distractors]);
    return {
      card,
      choices,
      userAnswer: null,
      correct: null,
    };
  });
}

export function TestMode({ cards }: Props) {
  const [state, setState] = React.useState<TestState>(() => ({
    questions: buildQuestions(cards),
    currentIndex: 0,
    submitted: false,
    score: 0,
  }));

  const { questions, currentIndex, submitted } = state;
  const q = questions[currentIndex];
  const total = questions.length;

  function handleChoose(choice: string) {
    if (q.userAnswer !== null) return; // already answered
    const isCorrect = choice === q.card.definition;
    setState((s) => {
      const updated = s.questions.map((item, i) =>
        i === currentIndex ? { ...item, userAnswer: choice, correct: isCorrect } : item
      );
      return { ...s, questions: updated };
    });
  }

  function handleNext() {
    const nextIndex = currentIndex + 1;
    if (nextIndex >= total) {
      const score = state.questions.filter((q) => q.correct).length;
      setState((s) => ({ ...s, submitted: true, score }));
    } else {
      setState((s) => ({ ...s, currentIndex: nextIndex }));
    }
  }

  function handleRestart() {
    setState({
      questions: buildQuestions(cards),
      currentIndex: 0,
      submitted: false,
      score: 0,
    });
  }

  if (total === 0) {
    return (
      <div className="learning-empty">
        <p>Cần ít nhất 1 thẻ để làm bài kiểm tra.</p>
      </div>
    );
  }

  if (submitted) {
    const pct = Math.round((state.score / total) * 100);
    return (
      <div className="learn-done">
        <h2>📝 Kết quả</h2>
        <p className="learn-score">
          <strong>{state.score}</strong> / {total} ({pct}%)
        </p>
        <div className="learn-review">
          {state.questions.map((q, i) => (
            <div key={i} className={`review-row ${q.correct ? "correct" : "wrong"}`}>
              <span className="review-term">{q.card.term}</span>
              <span className="review-answer">{q.userAnswer || "—"}</span>
              <span className="review-correct">{q.card.definition}</span>
            </div>
          ))}
        </div>
        <button className="primary-button" onClick={handleRestart}>Làm lại</button>
      </div>
    );
  }

  const answered = q.userAnswer !== null;

  return (
    <div className="test-mode">
      <div className="learn-header">
        <span className="flashcards-counter">{currentIndex + 1} / {total}</span>
      </div>

      <div className="test-question">
        <p className="test-question-label">Định nghĩa của từ nào sau đây là:</p>
        <p className="test-term">{q.card.term}</p>
      </div>

      <div className="test-choices">
        {q.choices.map((choice, i) => {
          let cls = "test-choice";
          if (answered) {
            if (choice === q.card.definition) cls += " choice-correct";
            else if (choice === q.userAnswer) cls += " choice-wrong";
          }
          return (
            <button
              key={i}
              className={cls}
              onClick={() => handleChoose(choice)}
              disabled={answered}
            >
              <span className="choice-letter">{String.fromCharCode(65 + i)}</span>
              {choice}
            </button>
          );
        })}
      </div>

      {answered && (
        <div className={`learn-feedback ${q.correct ? "feedback-correct" : "feedback-wrong"}`}>
          {q.correct ? "✅ Chính xác!" : `❌ Đáp án đúng: ${q.card.definition}`}
        </div>
      )}

      {answered && (
        <div className="learn-actions">
          <button className="primary-button" onClick={handleNext}>
            {currentIndex + 1 >= total ? "Xem kết quả" : "Câu tiếp →"}
          </button>
        </div>
      )}

      <div className="progress-bar-track">
        <div
          className="progress-bar-fill"
          style={{ width: `${((currentIndex + 1) / total) * 100}%` }}
        />
      </div>
    </div>
  );
}
