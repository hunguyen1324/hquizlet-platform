// MatchMode — Dev 4
// FE-LEARN-05: Ghép cặp term/definition bằng click, tính thời gian

import React from "react";
import type { Flashcard, MatchState, MatchItem } from "./types";
import "./learning.css";

type Props = {
  cards: Flashcard[];
};

const MAX_CARDS = 6; // Limit to 6 pairs for usability

function shuffleArray<T>(arr: T[]): T[] {
  const a = [...arr];
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [a[i], a[j]] = [a[j], a[i]];
  }
  return a;
}

function buildItems(cards: Flashcard[]): MatchItem[] {
  const subset = cards.slice(0, MAX_CARDS);
  const terms: MatchItem[] = subset.map((c) => ({
    id: `card-${c.id}-term`,
    cardId: c.id,
    text: c.term,
    type: "term",
    matched: false,
    selected: false,
  }));
  const defs: MatchItem[] = subset.map((c) => ({
    id: `card-${c.id}-def`,
    cardId: c.id,
    text: c.definition,
    type: "definition",
    matched: false,
    selected: false,
  }));
  return shuffleArray([...terms, ...defs]);
}

function formatTime(ms: number): string {
  const secs = Math.floor(ms / 1000);
  const m = Math.floor(secs / 60);
  const s = secs % 60;
  return m > 0 ? `${m}m ${s}s` : `${s}s`;
}

export function MatchMode({ cards }: Props) {
  const [state, setState] = React.useState<MatchState>(() => ({
    items: buildItems(cards),
    selectedId: null,
    matchedPairs: 0,
    totalPairs: Math.min(cards.length, MAX_CARDS),
    startedAt: Date.now(),
    finishedAt: null,
  }));
  const [wrongPair, setWrongPair] = React.useState<[string, string] | null>(null);

  const finished = state.matchedPairs === state.totalPairs && state.totalPairs > 0;

  function handleSelect(id: string) {
    if (wrongPair) return; // wait for error flash to clear
    setState((s) => {
      const item = s.items.find((x) => x.id === id);
      if (!item || item.matched || item.selected) return s;

      // First selection
      if (!s.selectedId) {
        return {
          ...s,
          items: s.items.map((x) => (x.id === id ? { ...x, selected: true } : x)),
          selectedId: id,
        };
      }

      // Second selection — check match
      const prev = s.items.find((x) => x.id === s.selectedId);
      if (!prev) return s;

      const isMatch = prev.cardId === item.cardId && prev.type !== item.type;
      if (isMatch) {
        const newPairs = s.matchedPairs + 1;
        return {
          ...s,
          items: s.items.map((x) =>
            x.id === id || x.id === s.selectedId
              ? { ...x, matched: true, selected: false }
              : x
          ),
          selectedId: null,
          matchedPairs: newPairs,
          finishedAt: newPairs === s.totalPairs ? Date.now() : null,
        };
      } else {
        // Wrong: flash error then deselect
        setWrongPair([s.selectedId!, id]);
        setTimeout(() => {
          setState((ss) => ({
            ...ss,
            items: ss.items.map((x) =>
              x.id === id || x.id === ss.selectedId
                ? { ...x, selected: false }
                : x
            ),
            selectedId: null,
          }));
          setWrongPair(null);
        }, 600);
        return {
          ...s,
          items: s.items.map((x) =>
            x.id === id ? { ...x, selected: true } : x
          ),
        };
      }
    });
  }

  function handleRestart() {
    setState({
      items: buildItems(cards),
      selectedId: null,
      matchedPairs: 0,
      totalPairs: Math.min(cards.length, MAX_CARDS),
      startedAt: Date.now(),
      finishedAt: null,
    });
    setWrongPair(null);
  }

  if (cards.length < 2) {
    return (
      <div className="learning-empty">
        <p>Cần ít nhất 2 thẻ để chơi ghép cặp.</p>
      </div>
    );
  }

  if (finished) {
    const elapsed = state.finishedAt! - state.startedAt;
    return (
      <div className="learn-done">
        <h2>🎉 Ghép xong!</h2>
        <p className="learn-score">
          Thời gian: <strong>{formatTime(elapsed)}</strong>
        </p>
        <p style={{ color: "#888", fontSize: "0.9rem" }}>
          {state.totalPairs} cặp — {state.totalPairs < cards.length ? `(hiển thị ${MAX_CARDS}/${cards.length} thẻ)` : ""}
        </p>
        <button className="primary-button" onClick={handleRestart}>Chơi lại</button>
      </div>
    );
  }

  return (
    <div className="match-mode">
      <div className="learn-header">
        <span className="flashcards-counter">
          {state.matchedPairs} / {state.totalPairs} cặp
        </span>
        {cards.length > MAX_CARDS && (
          <span className="match-note">Hiển thị {MAX_CARDS}/{cards.length} thẻ</span>
        )}
      </div>

      <div className="match-grid">
        {state.items.map((item) => {
          let cls = "match-item";
          if (item.matched) cls += " matched";
          else if (item.selected) cls += " selected";
          if (wrongPair && (wrongPair[0] === item.id || wrongPair[1] === item.id))
            cls += " wrong-flash";

          return (
            <button
              key={item.id}
              className={cls}
              onClick={() => !item.matched && handleSelect(item.id)}
              disabled={item.matched}
            >
              {item.text}
            </button>
          );
        })}
      </div>
    </div>
  );
}
