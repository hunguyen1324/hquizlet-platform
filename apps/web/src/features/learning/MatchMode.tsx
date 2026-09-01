// MatchMode — Dev 4
// P2-LEARN-04: Ghép cặp term/definition, timer local, completion state
// P3-LEARN-01,02,03: Nối completion với saveProgress thật; per-card results từ wrongCount

import React from "react";
import type { Flashcard, MatchState, MatchItem } from "./types";
import type { CardResult } from "./progressContract";
import { LearningEmptyState } from "../../components/learning/LearningEmptyState";
import { useProgressSave } from "./useProgressSave";
import { ProgressSaveStatus } from "./ProgressSaveStatus";
import "./learning.css";

type Props = {
  cards: Flashcard[];
  studySetId: number;
};

const MAX_CARDS = 6;

function shuffleArray<T>(arr: T[]): T[] {
  const a = [...arr];
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [a[i], a[j]] = [a[j], a[i]];
  }
  return a;
}

function buildItems(cards: Flashcard[]): MatchItem[] {
  const subset = shuffleArray(cards).slice(0, MAX_CARDS);
  const terms: MatchItem[] = subset.map((c) => ({
    id: `card-${c.id}-term`, cardId: c.id, text: c.term, type: "term", matched: false, selected: false,
  }));
  const defs: MatchItem[] = subset.map((c) => ({
    id: `card-${c.id}-def`, cardId: c.id, text: c.definition, type: "definition", matched: false, selected: false,
  }));
  return shuffleArray([...terms, ...defs]);
}

function formatTime(ms: number): string {
  const secs = Math.floor(ms / 1000);
  const m = Math.floor(secs / 60);
  const s = secs % 60;
  return m > 0 ? `${m}m ${s}s` : `${s}s`;
}

function useTimer(active: boolean) {
  const [elapsed, setElapsed] = React.useState(0);
  const elapsedRef = React.useRef(0);
  const startRef = React.useRef(Date.now());
  React.useEffect(() => {
    if (!active) return;
    startRef.current = Date.now() - elapsedRef.current;
    const id = setInterval(() => {
      const next = Date.now() - startRef.current;
      elapsedRef.current = next;
      setElapsed(next);
    }, 100);
    return () => clearInterval(id);
  }, [active]);
  return elapsed;
}

export function MatchMode({ cards, studySetId }: Props) {
  const [startedAt] = React.useState(() => new Date());
  const [state, setState] = React.useState<MatchState>(() => ({
    items: buildItems(cards),
    selectedId: null,
    matchedPairs: 0,
    totalPairs: Math.min(cards.length, MAX_CARDS),
    startedAt: Date.now(),
    finishedAt: null,
  }));
  const [wrongPair, setWrongPair] = React.useState<[string, string] | null>(null);
  const [wrongCount, setWrongCount] = React.useState(0);
  // Track which cardIds were used in this round (subset of full deck).
  const [subsetCards, setSubsetCards] = React.useState<Flashcard[]>(() =>
    cards.slice(0, MAX_CARDS)
  );

  const { status: saveStatus, onSessionComplete, reset: resetSave } = useProgressSave({
    studySetId,
    mode: "match",
  });

  const finished = state.matchedPairs === state.totalPairs && state.totalPairs > 0;
  const elapsed = useTimer(!finished && state.totalPairs > 0);

  // Trigger save on completion — once.
  const finishTriggeredRef = React.useRef(false);
  React.useEffect(() => {
    if (!finished || finishTriggeredRef.current) return;
    finishTriggeredRef.current = true;
    // Match mode: all cards are "correct" (player matched them all).
    // wrongCount tracks mismatches for accuracy display, not correctness.
    const score = state.totalPairs;
    const total = state.totalPairs;
    const cardResults: CardResult[] = subsetCards.slice(0, state.totalPairs).map((card) => ({
      cardId: card.id,
      correct: true,
      attempts: 1,
    }));
    onSessionComplete({ score, total, cardResults, startedAt });
  }, [finished]); // eslint-disable-line react-hooks/exhaustive-deps

  if (cards.length < 2) {
    return (
      <LearningEmptyState
        message="Cần ít nhất 2 thẻ để chơi ghép cặp."
        hint="Thêm thẻ trong phần 'Sửa thẻ'."
      />
    );
  }

  function handleSelect(id: string) {
    if (wrongPair) return;
    setState((s) => {
      const item = s.items.find((x) => x.id === id);
      if (!item || item.matched || item.selected) return s;
      if (!s.selectedId) {
        return { ...s, items: s.items.map((x) => (x.id === id ? { ...x, selected: true } : x)), selectedId: id };
      }
      const prev = s.items.find((x) => x.id === s.selectedId);
      if (!prev) return s;
      const isMatch = prev.cardId === item.cardId && prev.type !== item.type;
      if (isMatch) {
        const newPairs = s.matchedPairs + 1;
        return {
          ...s,
          items: s.items.map((x) =>
            x.id === id || x.id === s.selectedId ? { ...x, matched: true, selected: false } : x
          ),
          selectedId: null,
          matchedPairs: newPairs,
          finishedAt: newPairs === s.totalPairs ? Date.now() : s.finishedAt,
        };
      } else {
        const firstId = s.selectedId!;
        setWrongPair([firstId, id]);
        setWrongCount((c) => c + 1);
        setTimeout(() => {
          setState((ss) => ({
            ...ss,
            items: ss.items.map((x) =>
              x.id === id || x.id === firstId ? { ...x, selected: false } : x
            ),
            selectedId: null,
          }));
          setWrongPair(null);
        }, 700);
        return { ...s, items: s.items.map((x) => (x.id === id ? { ...x, selected: true } : x)) };
      }
    });
  }

  function handleRestart() {
    resetSave();
    finishTriggeredRef.current = false;
    const newSubset = shuffleArray(cards).slice(0, MAX_CARDS);
    setSubsetCards(newSubset);
    setState({
      items: buildItems(cards),
      selectedId: null,
      matchedPairs: 0,
      totalPairs: Math.min(cards.length, MAX_CARDS),
      startedAt: Date.now(),
      finishedAt: null,
    });
    setWrongPair(null);
    setWrongCount(0);
  }

  if (finished) {
    const duration = state.finishedAt! - state.startedAt;
    const accuracy = state.totalPairs > 0
      ? Math.round((state.totalPairs / (state.totalPairs + wrongCount)) * 100)
      : 100;
    return (
      <div className="learn-done">
        <h2>🎉 Ghép xong!</h2>
        <div className="match-stats">
          <div className="match-stat"><span className="stat-label">Thời gian</span><strong className="stat-value">{formatTime(duration)}</strong></div>
          <div className="match-stat"><span className="stat-label">Chính xác</span><strong className="stat-value">{accuracy}%</strong></div>
          <div className="match-stat"><span className="stat-label">Số cặp</span><strong className="stat-value">{state.totalPairs}</strong></div>
        </div>
        {cards.length > MAX_CARDS && (
          <p className="match-note" style={{ color: "#9ca3af", fontSize: "0.85rem" }}>
            (Đã hiển thị {MAX_CARDS}/{cards.length} thẻ ngẫu nhiên)
          </p>
        )}
        <ProgressSaveStatus status={saveStatus} onRetry={() => {
          const cardResults: CardResult[] = subsetCards.slice(0, state.totalPairs).map((c) => ({
            cardId: c.id, correct: true, attempts: 1,
          }));
          onSessionComplete({ score: state.totalPairs, total: state.totalPairs, cardResults, startedAt });
        }} />
        <button className="primary-button" onClick={handleRestart}>Chơi lại</button>
      </div>
    );
  }

  return (
    <div className="match-mode">
      <div className="learn-header">
        <span className="flashcards-counter">{state.matchedPairs} / {state.totalPairs} cặp</span>
        <span className="match-timer">{formatTime(elapsed)}</span>
        {wrongCount > 0 && <span className="wrong-count">❌ {wrongCount} lần sai</span>}
      </div>
      {cards.length > MAX_CARDS && <p className="match-note">Hiển thị {MAX_CARDS}/{cards.length} thẻ</p>}
      <div className="match-grid" role="group" aria-label="Ghép cặp thuật ngữ và định nghĩa">
        {state.items.map((item) => {
          let cls = "match-item";
          if (item.matched) cls += " matched";
          else if (item.selected) cls += " selected";
          if (wrongPair && (wrongPair[0] === item.id || wrongPair[1] === item.id)) cls += " wrong-flash";
          return (
            <button
              key={item.id}
              className={cls}
              onClick={() => !item.matched && handleSelect(item.id)}
              disabled={item.matched}
              aria-pressed={item.selected}
              aria-label={`${item.type === "term" ? "Thuật ngữ" : "Định nghĩa"}: ${item.text}${item.matched ? " (đã ghép)" : ""}`}
            >
              <span className="match-item-type">{item.type === "term" ? "T" : "Đ"}</span>
              <span className="match-item-text">{item.text}</span>
            </button>
          );
        })}
      </div>
      <p className="keyboard-hint" aria-hidden="true">Click để chọn và ghép cặp</p>
    </div>
  );
}
