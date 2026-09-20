// MatchEndScreen — hiện sau khi ghép xong
import React from "react";
import { ProgressSaveStatus } from "./ProgressSaveStatus";
import type { SaveStatus } from "./useProgressSave"; // eslint-disable-line @typescript-eslint/no-unused-vars
import "./learning.css";

type Props = {
  elapsedMs: number;
  wrongCount: number;
  score: number;
  total: number;
  saveStatus: SaveStatus;
  onRetry: () => void;
  onRestart: () => void;
  onNewRound: () => void;
  onBack?: () => void;
};

function formatTime(ms: number) {
  const secs = Math.floor(ms / 1000);
  const m = Math.floor(secs / 60).toString().padStart(2, "0");
  const s = (secs % 60).toString().padStart(2, "0");
  return `${m}:${s}`;
}

export function MatchEndScreen({
  elapsedMs,
  wrongCount,
  score,
  total,
  saveStatus,
  onRetry,
  onRestart,
  onNewRound,
}: Props) {
  const pct = total > 0 ? Math.round((score / total) * 100) : 100;
  const accuracy = wrongCount === 0 ? "Hoàn hảo! 🎉" : wrongCount <= 2 ? "Rất tốt! 👍" : "Cố lên! 💪";

  return (
    <div className="ql-match-end-screen">
      <div className="ql-match-end-emoji" aria-hidden="true">🎉</div>
      <h2 className="ql-match-end-title">Ghép xong!</h2>
      <p className="ql-match-end-sub">{accuracy}</p>

      <div className="ql-match-end-stats">
        <div className="ql-match-stat">
          <span className="ql-match-stat-val">{formatTime(elapsedMs)}</span>
          <span className="ql-match-stat-label">Thời gian</span>
        </div>
        <div className="ql-match-stat">
          <span className="ql-match-stat-val">{wrongCount}</span>
          <span className="ql-match-stat-label">Lần sai</span>
        </div>
        <div className="ql-match-stat">
          <span className="ql-match-stat-val">{pct}%</span>
          <span className="ql-match-stat-label">Chính xác</span>
        </div>
      </div>

      <ProgressSaveStatus status={saveStatus} onRetry={onRetry} />

      <div className="ql-match-end-actions">
        <button type="button" className="secondary-button" onClick={onRestart}>
          Chơi lại (cùng thẻ)
        </button>
        <button type="button" className="primary-button" onClick={onNewRound}>
          Lượt mới
        </button>
      </div>
    </div>
  );
}
