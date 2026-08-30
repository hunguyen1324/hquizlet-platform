// FlashcardsMode — Dev 4
// FE-LEARN-02: Flip card, next/prev, shuffle UI
// Runs on mock data in Sprint 1; connects to study set data via props

import React from "react";
import type { Flashcard } from "./types";
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

export function FlashcardsMode({ cards }: Props) {
  const [deck, setDeck] = React.useState<Flashcard[]>(cards);
  const [index, setIndex] = React.useState(0);
  const [flipped, setFlipped] = React.useState(false);
  const [shuffled, setShuffled] = React.useState(false);

  const current = deck[index];
  const total = deck.length;

  function handlePrev() {
    setFlipped(false);
    setIndex((i) => (i - 1 + total) % total);
  }

  function handleNext() {
    setFlipped(false);
    setIndex((i) => (i + 1) % total);
  }

  function handleShuffle() {
    if (shuffled) {
      setDeck(cards);
      setShuffled(false);
    } else {
      setDeck(shuffleArray(cards));
      setShuffled(true);
    }
    setIndex(0);
    setFlipped(false);
  }

  function handleRestart() {
    setIndex(0);
    setFlipped(false);
    if (shuffled) setDeck(shuffleArray(cards));
    else setDeck(cards);
  }

  if (total === 0) {
    return (
      <div className="learning-empty">
        <p>Chưa có thẻ nào trong học phần này.</p>
      </div>
    );
  }

  return (
    <div className="flashcards-mode">
      <div className="flashcards-toolbar">
        <span className="flashcards-counter">
          {index + 1} / {total}
        </span>
        <button
          className={`ghost-button${shuffled ? " active" : ""}`}
          onClick={handleShuffle}
          title="Xáo trộn"
        >
          {shuffled ? "🔀 Đang xáo" : "🔀 Xáo trộn"}
        </button>
        <button className="ghost-button" onClick={handleRestart} title="Làm lại">
          ↺ Làm lại
        </button>
      </div>

      {/* Flip card */}
      <div
        className={`flip-card${flipped ? " flipped" : ""}`}
        onClick={() => setFlipped((f) => !f)}
        tabIndex={0}
        onKeyDown={(e) => e.key === " " && setFlipped((f) => !f)}
        aria-label={flipped ? `Định nghĩa: ${current.definition}` : `Thuật ngữ: ${current.term}`}
      >
        <div className="flip-card-inner">
          <div className="flip-card-front">
            <span className="card-side-label">Thuật ngữ</span>
            <p className="card-text">{current.term}</p>
            <span className="flip-hint">Nhấp để lật</span>
          </div>
          <div className="flip-card-back">
            <span className="card-side-label">Định nghĩa</span>
            <p className="card-text">{current.definition}</p>
            {current.starred && <span className="starred-badge">★ Đã đánh dấu</span>}
          </div>
        </div>
      </div>

      <div className="flashcards-nav">
        <button className="nav-btn" onClick={handlePrev} disabled={total <= 1}>
          ← Trước
        </button>
        <button className="nav-btn" onClick={handleNext} disabled={total <= 1}>
          Tiếp →
        </button>
      </div>

      {/* Progress bar */}
      <div className="progress-bar-track">
        <div
          className="progress-bar-fill"
          style={{ width: `${((index + 1) / total) * 100}%` }}
        />
      </div>
    </div>
  );
}
