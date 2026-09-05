// FlashcardsMode — Quizlet-style UI (PM rewrite)
// Unified design system: ql-* classes, full Quizlet parity

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
  const [showBothSides, setShowBothSides] = React.useState(false);
  const [deck, setDeck] = React.useState<Flashcard[]>(cards);
  const [index, setIndex] = React.useState(0);
  const [flipped, setFlipped] = React.useState(false);
  const [seenCardIds, setSeenCardIds] = React.useState<Set<number>>(new Set());
  const completionTriggered = React.useRef(false);
  const touchStartX = React.useRef<number | null>(null);

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
    setTimeout(() => setIndex((i) => (i - 1 + total) % total), 60);
  }
  function handleNext() {
    setFlipped(false);
    setTimeout(() => setIndex((i) => (i + 1) % total), 60);
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
      <div className="ql-loading" role="status">
        <div className="ql-spinner" />
        <span>Đang tạo bộ flashcards…</span>
      </div>
    );
  }
  if (generation.state.state === "error") {
    return (
      <div className="ql-error" role="alert">
        <span>Không thể tải Flashcards: {generation.state.error.message}</span>
        <button className="ql-ghost-btn" onClick={generation.regenerate}>Thử lại</button>
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
    <div className="ql-root">
      {/* ── Progress bar ── */}
      <div className="ql-progress-wrap">
        <div
          className="ql-progress-bar"
          role="progressbar"
          aria-valuenow={index + 1}
          aria-valuemax={total}
          style={{ width: `${progressPct}%` }}
        />
      </div>

      {/* ── Counter + toolbar ── */}
      <div className="ql-topbar">
        <span className="ql-counter" aria-live="polite">
          <strong>{index + 1}</strong> / {total}
        </span>

        <div className="ql-actions">
          {/* Starred filter */}
          {starredCount > 0 && (
            <button
              className={`ql-icon-btn${starredOnly ? " ql-icon-btn--on" : ""}`}
              onClick={() => setStarredOnly((s) => !s)}
              title={starredOnly ? "Bỏ lọc sao" : "Chỉ thẻ đã đánh dấu sao"}
              aria-pressed={starredOnly}
            >
              <svg width="17" height="17" viewBox="0 0 24 24" fill={starredOnly ? "currentColor" : "none"} stroke="currentColor" strokeWidth="2">
                <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z"/>
              </svg>
            </button>
          )}

          {/* Show both sides toggle */}
          <button
            className={`ql-icon-btn${showBothSides ? " ql-icon-btn--on" : ""}`}
            onClick={() => setShowBothSides((v) => !v)}
            title="Hiện cả hai mặt"
            aria-pressed={showBothSides}
          >
            <svg width="17" height="17" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
              <rect x="2" y="4" width="9" height="16" rx="2"/>
              <rect x="13" y="4" width="9" height="16" rx="2"/>
            </svg>
          </button>

          {/* Shuffle */}
          <button
            className={`ql-icon-btn${shuffled ? " ql-icon-btn--on" : ""}`}
            onClick={() => { setShuffled(true); generation.regenerate(); }}
            title="Xáo trộn thẻ"
            aria-pressed={shuffled}
          >
            <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <polyline points="16 3 21 3 21 8"/><line x1="4" y1="20" x2="21" y2="3"/>
              <polyline points="21 16 21 21 16 21"/><line x1="15" y1="15" x2="21" y2="21"/>
              <line x1="4" y1="4" x2="9" y2="9"/>
            </svg>
          </button>

          {/* Restart */}
          <button className="ql-icon-btn" onClick={handleRestart} title="Bắt đầu lại">
            <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <polyline points="1 4 1 10 7 10"/><path d="M3.51 15a9 9 0 1 0 .49-4.15"/>
            </svg>
          </button>
        </div>
      </div>

      {/* ── Card ── */}
      {showBothSides ? (
        /* Both-sides view */
        <div className="ql-bothsides">
          <div className="ql-bothside-card ql-bothside-card--front">
            <span className="ql-face-label">Thuật ngữ</span>
            <p className="ql-card-text">{current.term}</p>
          </div>
          <div className="ql-bothside-card ql-bothside-card--back">
            <span className="ql-face-label">Định nghĩa</span>
            <p className="ql-card-text ql-card-text--def">{current.definition}</p>
          </div>
        </div>
      ) : (
        /* Flip card */
        <div
          className={`ql-card${flipped ? " ql-card--flipped" : ""}`}
          onClick={() => setFlipped((f) => !f)}
          onKeyDown={(e) => {
            if (e.key === " " || e.key === "Enter") { e.preventDefault(); setFlipped((f) => !f); }
          }}
          onTouchStart={(e) => { touchStartX.current = e.touches[0].clientX; }}
          onTouchEnd={(e) => {
            if (touchStartX.current === null) return;
            const dx = e.changedTouches[0].clientX - touchStartX.current;
            touchStartX.current = null;
            if (Math.abs(dx) < 40) { setFlipped((f) => !f); return; }
            if (dx < 0) handleNext(); else handlePrev();
          }}
          tabIndex={0}
          role="button"
          aria-label={flipped
            ? `Định nghĩa: ${current.definition}. Nhấn Space để lật lại.`
            : `Thuật ngữ: ${current.term}. Nhấn Space để xem định nghĩa.`}
        >
          <div className="ql-card-inner">
            {/* Front */}
            <div className="ql-face ql-face--front">
              <span className="ql-face-label">Thuật ngữ</span>
              {current.imageUrl && (
                <div className="ql-img-wrap">
                  <img src={current.imageUrl} alt={current.term} className="ql-img" />
                </div>
              )}
              <p className="ql-card-text">{current.term}</p>
              <span className="ql-flip-hint">
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true"><rect x="3" y="3" width="18" height="18" rx="2"/><path d="M7 12h10M12 7v10"/></svg>
                Nhấn để lật
              </span>
            </div>

            {/* Back */}
            <div className="ql-face ql-face--back">
              <span className="ql-face-label">Định nghĩa</span>
              <p className="ql-card-text ql-card-text--def">{current.definition}</p>
              {current.starred && (
                <span className="ql-starred">
                  <svg width="13" height="13" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z"/></svg>
                  Đã đánh dấu sao
                </span>
              )}
            </div>
          </div>
        </div>
      )}

      {/* ── Navigation ── */}
      <div className="ql-nav">
        <button className="ql-nav-btn" onClick={handlePrev} disabled={total <= 1} aria-label="Thẻ trước">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
            <path d="M15 18l-6-6 6-6"/>
          </svg>
        </button>

        <div className="ql-dots" aria-hidden="true">
          {total <= 20 ? deck.map((c, i) => (
            <button
              key={i}
              className={`ql-dot${i === index ? " ql-dot--active" : ""}${seenCardIds.has(c.id) ? " ql-dot--seen" : ""}`}
              onClick={(e) => { e.stopPropagation(); setFlipped(false); setIndex(i); }}
              tabIndex={-1}
            />
          )) : (
            <span className="ql-nav-count">{index + 1} / {total}</span>
          )}
        </div>

        <button className="ql-nav-btn" onClick={handleNext} disabled={total <= 1} aria-label="Thẻ tiếp theo">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
            <path d="M9 18l6-6-6-6"/>
          </svg>
        </button>
      </div>

      {/* ── Completion ── */}
      {allSeen && (
        <div className="ql-completion">
          <ProgressSaveStatus status={saveStatus} />
        </div>
      )}

      <p className="ql-kbd-hint" aria-hidden="true">← → điều hướng · Space lật thẻ · vuốt trên mobile</p>
    </div>
  );
}
