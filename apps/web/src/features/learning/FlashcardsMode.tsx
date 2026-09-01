// FlashcardsMode — Dev 4
// P2-LEARN-01: Flip card, next/prev, shuffle, starred filter
// P3-LEARN-01,02,03: Nối completion với saveProgress thật khi xem hết toàn bộ deck

import React from "react";
import type { Flashcard } from "./types";
import { LearningEmptyState } from "../../components/learning/LearningEmptyState";
import { useProgressSave } from "./useProgressSave";
import { ProgressSaveStatus } from "./ProgressSaveStatus";
import "./learning.css";

type Props = {
  cards: Flashcard[];
  studySetId: number;
};

function shuffleArray<T>(arr: T[]): T[] {
  const a = [...arr];
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [a[i], a[j]] = [a[j], a[i]];
  }
  return a;
}

export function FlashcardsMode({ cards, studySetId }: Props) {
  const [startedAt] = React.useState(() => new Date());
  const [shuffled, setShuffled] = React.useState(false);
  const [starredOnly, setStarredOnly] = React.useState(false);
  const [deck, setDeck] = React.useState<Flashcard[]>(cards);
  const [index, setIndex] = React.useState(0);
  const [flipped, setFlipped] = React.useState(false);
  // Track seen cards for completion detection.
  const [seenCardIds, setSeenCardIds] = React.useState<Set<number>>(new Set());

  const { status: saveStatus, onSessionComplete, reset: resetSave } = useProgressSave({
    studySetId,
    mode: "flashcards",
  });

  React.useEffect(() => {
    const base = starredOnly ? cards.filter((c) => c.starred) : cards;
    setDeck(shuffled ? shuffleArray(base) : [...base]);
    setIndex(0);
    setFlipped(false);
    setSeenCardIds(new Set());
    resetSave();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [cards, starredOnly, shuffled]);

  const current = deck[index];
  const total = deck.length;

  // Mark current card as seen and check for completion.
  React.useEffect(() => {
    if (!current) return;
    setSeenCardIds((prev) => {
      if (prev.has(current.id)) return prev;
      const next = new Set(prev);
      next.add(current.id);
      return next;
    });
  }, [current]);

  // Trigger save when all cards in current deck have been seen.
  const completionTriggered = React.useRef(false);
  React.useEffect(() => {
    if (completionTriggered.current || total === 0) return;
    if (seenCardIds.size >= total) {
      completionTriggered.current = true;
      // Flashcards: no right/wrong — score = total (all seen = session complete).
      onSessionComplete({
        score: total,
        total,
        cardResults: deck.map((c) => ({ cardId: c.id, correct: true, attempts: 1 })),
        startedAt,
      });
    }
  }, [seenCardIds, total]); // eslint-disable-line react-hooks/exhaustive-deps

  function handlePrev() {
    setFlipped(false);
    setIndex((i) => (i - 1 + total) % total);
  }

  function handleNext() {
    setFlipped(false);
    setIndex((i) => (i + 1) % total);
  }

  function handleRestart() {
    completionTriggered.current = false;
    resetSave();
    const base = starredOnly ? cards.filter((c) => c.starred) : cards;
    setDeck(shuffled ? shuffleArray(base) : [...base]);
    setIndex(0);
    setFlipped(false);
    setSeenCardIds(new Set());
  }

  React.useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.target instanceof HTMLButtonElement) return;
      if (e.key === " ") { e.preventDefault(); setFlipped((f) => !f); }
      if (e.key === "ArrowLeft") { e.preventDefault(); handlePrev(); }
      if (e.key === "ArrowRight") { e.preventDefault(); handleNext(); }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [total, index]);

  const starredCount = cards.filter((c) => c.starred).length;

  if (cards.length === 0) return <LearningEmptyState />;
  if (total === 0 && starredOnly) {
    return (
      <LearningEmptyState
        message="Chưa có thẻ nào được đánh dấu sao."
        hint="Quay về tab Overview để đánh dấu thẻ, rồi bật lọc lại."
      />
    );
  }

  const allSeen = seenCardIds.size >= total && total > 0;

  return (
    <div className="flashcards-mode">
      <div className="flashcards-toolbar">
        <span className="flashcards-counter">{index + 1} / {total}</span>
        <div className="toolbar-right">
          {starredCount > 0 && (
            <button
              className={`ghost-button${starredOnly ? " active" : ""}`}
              onClick={() => setStarredOnly((s) => !s)}
              title={starredOnly ? "Bỏ lọc sao" : "Chỉ xem thẻ đã đánh dấu sao"}
            >
              {starredOnly ? "★ Đang lọc sao" : "☆ Lọc sao"}
            </button>
          )}
          <button
            className={`ghost-button${shuffled ? " active" : ""}`}
            onClick={() => setShuffled((s) => !s)}
            title="Xáo trộn"
          >
            {shuffled ? "🔀 Đang xáo" : "🔀 Xáo trộn"}
          </button>
          <button className="ghost-button" onClick={handleRestart} title="Làm lại">↺ Làm lại</button>
        </div>
      </div>

      <div
        className={`flip-card${flipped ? " flipped" : ""}`}
        onClick={() => setFlipped((f) => !f)}
        tabIndex={0}
        onKeyDown={(e) => {
          if (e.key === " " || e.key === "Enter") { e.preventDefault(); setFlipped((f) => !f); }
        }}
        role="button"
        aria-label={flipped ? `Định nghĩa: ${current.definition}` : `Thuật ngữ: ${current.term}. Nhấn Space hoặc click để lật.`}
      >
        <div className="flip-card-inner">
          <div className="flip-card-front">
            <span className="card-side-label">Thuật ngữ</span>
            <p className="card-text">{current.term}</p>
            <span className="flip-hint">Space / click để lật</span>
          </div>
          <div className="flip-card-back">
            <span className="card-side-label">Định nghĩa</span>
            <p className="card-text">{current.definition}</p>
            {current.starred && <span className="starred-badge">★ Đã đánh dấu</span>}
          </div>
        </div>
      </div>

      <div className="flashcards-nav">
        <button className="nav-btn" onClick={handlePrev} disabled={total <= 1} aria-label="Thẻ trước (←)">← Trước</button>
        <button className="nav-btn" onClick={handleNext} disabled={total <= 1} aria-label="Thẻ tiếp (→)">Tiếp →</button>
      </div>

      <div className="progress-bar-track" role="progressbar" aria-valuenow={index + 1} aria-valuemax={total}>
        <div className="progress-bar-fill" style={{ width: `${((index + 1) / total) * 100}%` }} />
      </div>

      {/* Show save status when all cards seen */}
      {allSeen && (
        <div className="flashcards-completion">
          <ProgressSaveStatus status={saveStatus} />
        </div>
      )}

      <p className="keyboard-hint" aria-hidden="true">← → để điều hướng · Space để lật</p>
    </div>
  );
}
