// FlashcardsMode — Dev 4 (UI Refresh)
// P2-LEARN-01: Flip card, next/prev, shuffle, starred filter
// P3-LEARN-01,02,03: saveProgress khi xem hết toàn bộ deck

import React from "react";
import type { Flashcard } from "./types";
import { LearningEmptyState } from "../../components/learning/LearningEmptyState";
import { useProgressSave } from "./useProgressSave";
import { ProgressSaveStatus } from "./ProgressSaveStatus";
import { useQuizGeneration } from "./useQuizGeneration";
import "./learning.css";

type Props = {
  cards: Flashcard[];
  studySetId: number;
};

export function FlashcardsMode({ cards, studySetId }: Props) {
  const generation = useQuizGeneration(studySetId, "flashcards", Math.min(cards.length, 100));
  const [startedAt, setStartedAt] = React.useState(() => new Date());
  const [shuffled, setShuffled] = React.useState(false);
  const [starredOnly, setStarredOnly] = React.useState(false);
  const [deck, setDeck] = React.useState<Flashcard[]>(cards);
  const [index, setIndex] = React.useState(0);
  const [flipped, setFlipped] = React.useState(false);
  const [darkCard, setDarkCard] = React.useState(false);
  const [seenCardIds, setSeenCardIds] = React.useState<Set<number>>(new Set());
  const completionTriggered = React.useRef(false);

  const { status: saveStatus, onSessionComplete, reset: resetSave } = useProgressSave({
    studySetId,
    mode: "flashcards",
  });

  React.useEffect(() => {
    if (generation.state.state !== "ready") return;
    const generated = generation.state.data.items.map((item) => ({
      id: item.flashcardId,
      studySetId,
      term: item.term ?? "",
      definition: item.definition ?? "",
      starred: item.starred ?? false,
    }));
    const base = starredOnly ? generated.filter((c) => c.starred) : generated;
    setDeck(base);
    setIndex(0);
    setFlipped(false);
    setSeenCardIds(new Set());
    completionTriggered.current = false;
    resetSave();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [generation.state, starredOnly, studySetId, resetSave]);

  const current = deck[index];
  const total = deck.length;

  React.useEffect(() => {
    if (!current) return;
    setSeenCardIds((prev) => {
      if (prev.has(current.id)) return prev;
      const next = new Set(prev);
      next.add(current.id);
      return next;
    });
  }, [current]);

  React.useEffect(() => {
    if (completionTriggered.current || total === 0) return;
    if (seenCardIds.size >= total) {
      completionTriggered.current = true;
      onSessionComplete({
        score: total,
        total,
        cardResults: deck.map((c) => ({ flashcardId: c.id, correct: true, attempts: 1 })),
        startedAt,
      });
    }
  }, [seenCardIds, total]); // eslint-disable-line react-hooks/exhaustive-deps

  function handlePrev() {
    setFlipped(false);
    setTimeout(() => setIndex((i) => (i - 1 + total) % total), 50);
  }

  function handleNext() {
    setFlipped(false);
    setTimeout(() => setIndex((i) => (i + 1) % total), 50);
  }

  function handleRestart() {
    completionTriggered.current = false;
    resetSave();
    setStartedAt(new Date());
    setIndex(0);
    setFlipped(false);
    setSeenCardIds(new Set());
    generation.regenerate();
  }

  React.useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.target instanceof HTMLButtonElement || e.target instanceof HTMLInputElement) return;
      if (e.key === " ") { e.preventDefault(); setFlipped((f) => !f); }
      if (e.key === "ArrowLeft") { e.preventDefault(); handlePrev(); }
      if (e.key === "ArrowRight") { e.preventDefault(); handleNext(); }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [total, index]);

  const starredCount = cards.filter((c) => c.starred).length;
  const allSeen = seenCardIds.size >= total && total > 0;
  const progressPct = total > 0 ? ((index + 1) / total) * 100 : 0;

  if (cards.length === 0) return <LearningEmptyState />;
  if (generation.state.state === "loading") {
    return (
      <div className="fc-loading" role="status">
        <div className="fc-loading-spinner" />
        <span>Đang tạo bộ flashcards…</span>
      </div>
    );
  }
  if (generation.state.state === "error") {
    return (
      <div className="fc-error" role="alert">
        <span>Không thể tạo Flashcards: {generation.state.error.message}</span>
        <button className="fc-btn fc-btn--ghost" onClick={generation.regenerate}>Thử lại</button>
      </div>
    );
  }
  if (total === 0 && starredOnly) {
    return (
      <LearningEmptyState
        message="Chưa có thẻ nào được đánh dấu sao."
        hint="Quay về tab Overview để đánh dấu thẻ, rồi bật lọc lại."
      />
    );
  }

  return (
    <div className={`fc-root${darkCard ? " fc-root--dark" : ""}`}>
      {/* Toolbar */}
      <div className="fc-toolbar">
        <span className="fc-counter">
          <strong>{index + 1}</strong>
          <span className="fc-counter-sep">/</span>
          <span>{total}</span>
        </span>

        <div className="fc-toolbar-actions">
          {starredCount > 0 && (
            <button
              className={`fc-btn fc-btn--ghost${starredOnly ? " fc-btn--active" : ""}`}
              onClick={() => setStarredOnly((s) => !s)}
              title={starredOnly ? "Bỏ lọc sao" : "Chỉ xem thẻ đã đánh dấu sao"}
            >
              {starredOnly ? "★" : "☆"}
            </button>
          )}
          <button
            className={`fc-btn fc-btn--ghost${shuffled ? " fc-btn--active" : ""}`}
            onClick={() => { setShuffled(true); generation.regenerate(); }}
            title="Xáo trộn"
          >
            🔀
          </button>
          <button
            className={`fc-btn fc-btn--ghost${darkCard ? " fc-btn--active" : ""}`}
            onClick={() => setDarkCard((d) => !d)}
            title="Đổi màu thẻ"
          >
            {darkCard ? "☀️" : "🌙"}
          </button>
          <button className="fc-btn fc-btn--ghost" onClick={handleRestart} title="Làm lại">↺</button>
        </div>
      </div>

      {/* Progress bar */}
      <div className="fc-progress-track" role="progressbar" aria-valuenow={index + 1} aria-valuemax={total}>
        <div className="fc-progress-fill" style={{ width: `${progressPct}%` }} />
      </div>

      {/* Card */}
      <div
        className={`fc-card${flipped ? " fc-card--flipped" : ""}`}
        onClick={() => setFlipped((f) => !f)}
        tabIndex={0}
        onKeyDown={(e) => {
          if (e.key === " " || e.key === "Enter") { e.preventDefault(); setFlipped((f) => !f); }
        }}
        role="button"
        aria-label={flipped
          ? `Định nghĩa: ${current.definition}. Nhấn Space để lật lại.`
          : `Thuật ngữ: ${current.term}. Nhấn Space để xem định nghĩa.`}
      >
        <div className="fc-card-inner">
          {/* Front */}
          <div className="fc-face fc-face--front">
            <span className="fc-face-label">Thuật ngữ</span>
            {current.imageUrl && (
              <div className="fc-card-img-wrap">
                <img src={current.imageUrl} alt={current.term} className="fc-card-img" />
              </div>
            )}
            <p className="fc-card-text">{current.term}</p>
            <span className="fc-flip-hint">Nhấn Space hoặc click để lật →</span>
          </div>
          {/* Back */}
          <div className="fc-face fc-face--back">
            <span className="fc-face-label">Định nghĩa</span>
            <p className="fc-card-text fc-card-text--def">{current.definition}</p>
            {current.starred && <span className="fc-starred-badge">★ Đã đánh dấu</span>}
          </div>
        </div>
      </div>

      {/* Navigation */}
      <div className="fc-nav">
        <button
          className="fc-nav-btn fc-nav-btn--prev"
          onClick={handlePrev}
          disabled={total <= 1}
          aria-label="Thẻ trước"
        >
          <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
            <path d="M12 4l-6 6 6 6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
          </svg>
          <span>Trước</span>
        </button>

        {/* Dot indicators — show up to 9 dots */}
        {total <= 9 ? (
          <div className="fc-dots" aria-hidden="true">
            {deck.map((_, i) => (
              <button
                key={i}
                className={`fc-dot${i === index ? " fc-dot--active" : ""}${seenCardIds.has(deck[i].id) ? " fc-dot--seen" : ""}`}
                onClick={(e) => { e.stopPropagation(); setFlipped(false); setIndex(i); }}
                aria-label={`Đến thẻ ${i + 1}`}
              />
            ))}
          </div>
        ) : (
          <span className="fc-nav-label" aria-hidden="true">{index + 1} / {total}</span>
        )}

        <button
          className="fc-nav-btn fc-nav-btn--next"
          onClick={handleNext}
          disabled={total <= 1}
          aria-label="Thẻ tiếp"
        >
          <span>Tiếp</span>
          <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
            <path d="M8 4l6 6-6 6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
          </svg>
        </button>
      </div>

      {/* Completion */}
      {allSeen && (
        <div className="fc-completion">
          <ProgressSaveStatus status={saveStatus} />
        </div>
      )}

      <p className="fc-kbd-hint" aria-hidden="true">← → điều hướng · Space lật thẻ</p>
    </div>
  );
}
