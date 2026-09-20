// MatchMode — full parity: start screen + shake animation + MM:SS timer + end screen
import React from "react";
import { useAuth } from "../auth/AuthContext";
import { quizApi, type QuizAnswer, type QuizGeneratedItem } from "../../lib/api/client";
import type { Flashcard } from "./types";
import type { CardResult } from "./progressContract";
import { LearningEmptyState } from "../../components/learning/LearningEmptyState";
import { useProgressSave } from "./useProgressSave";
import { useQuizGeneration } from "./useQuizGeneration";
import { MatchStartScreen } from "./MatchStartScreen";
import { MatchEndScreen } from "./MatchEndScreen";
import "./learning.css";

type Props = { cards: Flashcard[]; studySetId: number };
type MatchTile = { id: string; cardId: number; text: string; type: "term" | "definition"; pairId: string };
type Phase = "start" | "playing" | "done";

function formatTime(ms: number) {
  const secs = Math.floor(ms / 1000);
  const m = Math.floor(secs / 60).toString().padStart(2, "0");
  const s = (secs % 60).toString().padStart(2, "0");
  return `${m}:${s}`;
}

export function MatchMode({ cards, studySetId }: Props) {
  const { token } = useAuth();
  const generation = useQuizGeneration(studySetId, "match", 6);
  const { status: saveStatus, onSessionComplete, reset: resetSave } = useProgressSave({ studySetId, mode: "match" });

  const [phase, setPhase] = React.useState<Phase>("start");
  const [selectedId, setSelectedId] = React.useState<string | null>(null);
  const [matched, setMatched] = React.useState<Set<number>>(new Set());
  const [wrongTileIds, setWrongTileIds] = React.useState<Set<string>>(new Set());
  const [wrongCount, setWrongCount] = React.useState(0);
  const [answers, setAnswers] = React.useState<QuizAnswer[]>([]);
  const [attemptsByCard, setAttemptsByCard] = React.useState<Record<number, number>>({});
  const [result, setResult] = React.useState<{ score: number; total: number; cardResults: CardResult[] } | null>(null);
  const [startedAt, setStartedAt] = React.useState(() => new Date());
  const [elapsed, setElapsed] = React.useState(0);
  const [error, setError] = React.useState<string | null>(null);

  const data = generation.state.state === "ready" ? generation.state.data : null;
  const tiles = React.useMemo(() => toTiles(data?.items ?? []), [data]);
  const totalPairs = React.useMemo(() => new Set(tiles.map((x) => x.pairId)).size, [tiles]);
  const finished = phase === "playing" && totalPairs > 0 && matched.size === totalPairs;

  // Timer — only runs when playing
  React.useEffect(() => {
    if (phase !== "playing" || finished) return;
    const timer = window.setInterval(() => setElapsed(Date.now() - startedAt.getTime()), 100);
    return () => window.clearInterval(timer);
  }, [phase, finished, startedAt]);

  // Auto-evaluate when all matched
  React.useEffect(() => {
    if (!finished || !data || result) return;
    const payload = answers.map((a) => ({ ...a, attempts: Math.max(1, a.attempts) }));
    if (payload.length !== totalPairs) return;
    let cancelled = false;
    quizApi
      .evaluate(token, studySetId, { mode: "match", seed: data.seed, limit: totalPairs, answers: payload })
      .then((res) => {
        if (cancelled) return;
        const cardResults = res.cardResults as CardResult[];
        setResult({ score: res.score, total: res.total, cardResults });
        onSessionComplete({ score: res.score, total: res.total, cardResults, startedAt });
        setPhase("done");
      })
      .catch((e: unknown) => {
        if (!cancelled) {
          setError(e instanceof Error ? e.message : "Không thể chấm kết quả");
          // Still show done even if evaluation fails
          setPhase("done");
        }
      });
    return () => { cancelled = true; };
  }, [finished, data, answers, result, token, studySetId, totalPairs, startedAt, onSessionComplete]);

  /* ── Guard states ── */
  if (cards.length < 2) return <LearningEmptyState message="Cần ít nhất 2 thẻ để chơi ghép cặp." hint="Thêm thẻ trong phần 'Sửa thẻ'." />;
  if (generation.state.state === "loading") return <div className="learn-loading" role="status">Đang tạo bộ ghép cặp…</div>;
  if (generation.state.state === "error") return (
    <div className="learn-error" role="alert">
      Không thể tạo Match: {generation.state.error.message}
      <button className="secondary-button" onClick={generation.regenerate}>Thử lại</button>
    </div>
  );
  if (tiles.length === 0) return <LearningEmptyState message="Backend không trả về cặp ghép hợp lệ." hint="Thử lại để tạo một bộ mới." />;

  /* ── Start screen ── */
  if (phase === "start") {
    return (
      <MatchStartScreen
        totalPairs={totalPairs}
        onStart={() => {
          setStartedAt(new Date());
          setElapsed(0);
          setPhase("playing");
        }}
      />
    );
  }

  /* ── Done screen ── */
  if (phase === "done") {
    return (
      <MatchEndScreen
        elapsedMs={elapsed}
        wrongCount={wrongCount}
        score={result?.score ?? matched.size}
        total={result?.total ?? totalPairs}
        saveStatus={saveStatus}
        onRetry={() => {
          if (result) onSessionComplete({ score: result.score, total: result.total, cardResults: result.cardResults, startedAt });
        }}
        onRestart={restart}
      />
    );
  }

  /* ── Select handler ── */
  function select(tile: MatchTile) {
    if (matched.has(tile.cardId) || wrongTileIds.has(tile.id)) return;
    if (!selectedId) { setSelectedId(tile.id); return; }
    const first = tiles.find((x) => x.id === selectedId);
    if (!first || first.id === tile.id) { setSelectedId(tile.id); return; }
    const isCorrect = first.cardId === tile.cardId && first.type !== tile.type;
    const correctAttempts = (attemptsByCard[tile.cardId] ?? 0) + 1;
    setAttemptsByCard((cur) => {
      const next = { ...cur };
      for (const id of [first.cardId, tile.cardId]) next[id] = (next[id] ?? 0) + 1;
      return next;
    });
    if (isCorrect) {
      setAnswers((cur) => [...cur.filter((x) => x.flashcardId !== tile.cardId), {
        flashcardId: tile.cardId,
        pairId: tile.pairId,
        matchedFlashcardId: tile.cardId,
        attempts: correctAttempts,
      }]);
      setMatched((cur) => new Set(cur).add(tile.cardId));
      setSelectedId(null);
    } else {
      // Wrong match → shake both tiles
      const wrongIds = new Set([first.id, tile.id]);
      setWrongTileIds(wrongIds);
      setWrongCount((n) => n + 1);
      setSelectedId(null);
      window.setTimeout(() => {
        setWrongTileIds((cur) => {
          const next = new Set(cur);
          wrongIds.forEach((id) => next.delete(id));
          return next;
        });
      }, 400);
    }
  }

  function restart() {
    resetSave();
    setStartedAt(new Date());
    setElapsed(0);
    setSelectedId(null);
    setMatched(new Set());
    setWrongTileIds(new Set());
    setWrongCount(0);
    setAnswers([]);
    setAttemptsByCard({});
    setResult(null);
    setError(null);
    setPhase("start");
    generation.regenerate();
  }

  /* ── Playing ── */
  return (
    <div className="match-mode">
      <div className="learn-header">
        <span className="flashcards-counter">{matched.size} / {totalPairs} cặp</span>
        <span className="match-timer-display">{formatTime(elapsed)}</span>
        {wrongCount > 0 && <span className="wrong-count">❌ {wrongCount} lần sai</span>}
      </div>
      <div className="match-grid" role="group" aria-label="Ghép cặp thuật ngữ và định nghĩa">
        {tiles.map((tile) => {
          const isMatched = matched.has(tile.cardId);
          const isSelected = selectedId === tile.id;
          const isWrong = wrongTileIds.has(tile.id);
          return (
            <button
              key={tile.id}
              className={[
                "match-item",
                isMatched ? "matched" : "",
                isSelected ? "selected" : "",
                isWrong ? "ql-match-tile--wrong" : "",
              ].filter(Boolean).join(" ")}
              onClick={() => select(tile)}
              disabled={isMatched}
              aria-pressed={isSelected}
            >
              <span className="match-item-type">{tile.type === "term" ? "T" : "Đ"}</span>
              <span className="match-item-text">{tile.text}</span>
            </button>
          );
        })}
      </div>
      {error && <p className="learn-error">{error}</p>}
      <p className="keyboard-hint" aria-hidden="true">Click để chọn và ghép cặp · {formatTime(elapsed)}</p>
    </div>
  );
}

function toTiles(items: QuizGeneratedItem[]): MatchTile[] {
  const result: MatchTile[] = [];
  for (const item of items) {
    if (item.kind === "term" || item.kind === "definition") {
      result.push({ id: item.id, cardId: item.flashcardId, text: item.text ?? "", type: item.kind, pairId: item.pairId ?? String(item.flashcardId) });
    } else if (item.kind === "pair" && item.term !== undefined && item.definition !== undefined) {
      result.push({ id: `${item.id}-term`, cardId: item.flashcardId, text: item.term, type: "term", pairId: item.pairId ?? String(item.flashcardId) });
      result.push({ id: `${item.id}-definition`, cardId: item.flashcardId, text: item.definition, type: "definition", pairId: item.pairId ?? String(item.flashcardId) });
    }
  }
  return result;
}
