// MatchMode — Dev 5
// Phase 4: Match renders only the subset/pairs returned by quiz/generate and
// sends the selected card identities to quiz/evaluate. No client-side shuffle.
import React from "react";
import { useAuth } from "../auth/AuthContext";
import { quizApi, type QuizAnswer, type QuizGeneratedItem } from "../../lib/api/client";
import type { Flashcard } from "./types";
import type { CardResult } from "./progressContract";
import { LearningEmptyState } from "../../components/learning/LearningEmptyState";
import { useProgressSave } from "./useProgressSave";
import { ProgressSaveStatus } from "./ProgressSaveStatus";
import { useQuizGeneration } from "./useQuizGeneration";
import "./learning.css";

type Props = { cards: Flashcard[]; studySetId: number };
type MatchTile = { id: string; cardId: number; text: string; type: "term" | "definition"; pairId: string };

export function MatchMode({ cards, studySetId }: Props) {
  const { token } = useAuth();
  const generation = useQuizGeneration(studySetId, "match", 6);
  const { status: saveStatus, onSessionComplete, reset: resetSave } = useProgressSave({ studySetId, mode: "match" });
  const [selectedId, setSelectedId] = React.useState<string | null>(null);
  const [matched, setMatched] = React.useState<Set<number>>(new Set());
  const [wrongCount, setWrongCount] = React.useState(0);
  const [answers, setAnswers] = React.useState<QuizAnswer[]>([]);
  const [evaluated, setEvaluated] = React.useState<Set<number>>(new Set());
  const [startedAt, setStartedAt] = React.useState(() => new Date());
  const [elapsed, setElapsed] = React.useState(0);
  const [error, setError] = React.useState<string | null>(null);

  const data = generation.state.state === "ready" ? generation.state.data : null;
  const tiles = React.useMemo(() => toTiles(data?.items ?? []), [data]);
  const totalPairs = React.useMemo(() => new Set(tiles.map((x) => x.pairId)).size, [tiles]);
  const finished = totalPairs > 0 && matched.size === totalPairs;

  React.useEffect(() => {
    if (finished) return;
    const timer = window.setInterval(() => setElapsed(Date.now() - startedAt.getTime()), 100);
    return () => window.clearInterval(timer);
  }, [finished, startedAt]);

  React.useEffect(() => {
    if (!finished || !data || evaluated.size > 0) return;
    const payload = answers.map((a) => ({ ...a, attempts: Math.max(1, a.attempts) }));
    if (payload.length !== totalPairs) return;
    let cancelled = false;
    quizApi.evaluate(token, studySetId, { mode: "match", seed: data.seed, answers: payload })
      .then((result) => {
        if (cancelled) return;
        const next = new Set(result.cardResults.filter((r) => r.correct).map((r) => r.flashcardId));
        setEvaluated(next);
        onSessionComplete({ score: result.score, total: result.total, cardResults: result.cardResults as CardResult[], startedAt });
      })
      .catch((e: unknown) => { if (!cancelled) setError(e instanceof Error ? e.message : "Không thể chấm kết quả"); });
    return () => { cancelled = true; };
  }, [finished, data, answers, evaluated.size, token, studySetId, totalPairs, startedAt, onSessionComplete]);

  if (cards.length < 2) return <LearningEmptyState message="Cần ít nhất 2 thẻ để chơi ghép cặp." hint="Thêm thẻ trong phần 'Sửa thẻ'." />;
  if (generation.state.state === "loading") return <div className="learn-loading" role="status">Đang tạo bộ ghép cặp…</div>;
  if (generation.state.state === "error") return <div className="learn-error" role="alert">Không thể tạo Match: {generation.state.error.message}<button className="secondary-button" onClick={generation.regenerate}>Thử lại</button></div>;
  if (tiles.length === 0) return <LearningEmptyState message="Backend không trả về cặp ghép hợp lệ." hint="Thử lại để tạo một bộ mới." />;

  function select(tile: MatchTile) {
    if (matched.has(tile.cardId)) return;
    if (!selectedId) { setSelectedId(tile.id); return; }
    const first = tiles.find((x) => x.id === selectedId);
    if (!first || first.id === tile.id) return;
    const isCorrect = first.cardId === tile.cardId && first.type !== tile.type;
    const attempts = (answers.find((x) => x.flashcardId === tile.cardId)?.attempts ?? 0) + 1;
    setAnswers((current) => [...current.filter((x) => x.flashcardId !== tile.cardId), {
      flashcardId: tile.cardId,
      pairId: tile.pairId,
      matchedFlashcardId: tile.cardId,
      attempts,
    }]);
    if (isCorrect) setMatched((current) => new Set(current).add(tile.cardId));
    else setWrongCount((n) => n + 1);
    setSelectedId(null);
  }

  function restart() {
    resetSave(); setStartedAt(new Date()); setElapsed(0); setSelectedId(null); setMatched(new Set());
    setWrongCount(0); setAnswers([]); setEvaluated(new Set()); setError(null); generation.regenerate();
  }

  if (finished && evaluated.size > 0) {
    const pct = totalPairs ? Math.round((evaluated.size / totalPairs) * 100) : 0;
    return <div className="learn-done"><h2>🎉 Ghép xong!</h2><p className="learn-score"><strong>{evaluated.size}</strong> / {totalPairs} ({pct}%)</p><p>Thời gian: {Math.floor(elapsed / 1000)}s · Lần sai: {wrongCount}</p>{error && <p className="learn-error">{error}</p>}<ProgressSaveStatus status={saveStatus} onRetry={() => undefined} /><button className="primary-button" onClick={restart}>Chơi lại</button></div>;
  }

  return <div className="match-mode">
    <div className="learn-header"><span className="flashcards-counter">{matched.size} / {totalPairs} cặp</span><span className="match-timer">{Math.floor(elapsed / 1000)}s</span>{wrongCount > 0 && <span className="wrong-count">❌ {wrongCount} lần sai</span>}</div>
    <div className="match-grid" role="group" aria-label="Ghép cặp thuật ngữ và định nghĩa">
      {tiles.map((tile) => <button key={tile.id} className={`match-item${matched.has(tile.cardId) ? " matched" : ""}${selectedId === tile.id ? " selected" : ""}`} onClick={() => select(tile)} disabled={matched.has(tile.cardId)} aria-pressed={selectedId === tile.id}><span className="match-item-type">{tile.type === "term" ? "T" : "Đ"}</span><span className="match-item-text">{tile.text}</span></button>)}
    </div>
    <p className="keyboard-hint" aria-hidden="true">Click để chọn và ghép cặp · Seed: {data?.seed ?? "-"}</p>
  </div>;
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
