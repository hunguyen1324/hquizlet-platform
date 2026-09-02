// StudyDetail - Dev 3 + Dev 4 integration
// Dev 4 (FE-LEARN-06): LearningContainer gắn thật vào các tab learning mode

import React from "react";
import type { StudySet, Flashcard } from "../../types";
import type { LearningMode } from "../learning/types";
import { LearningContainer } from "../learning";
import { fetchProgressSummary, type ProgressListResponse } from "../learning/progressContract";
import { useAuth } from "../auth/AuthContext";
import { ProgressPanel, type ProgressPanelStatus } from "../../components/progress";

type StudyMode = "dashboard" | LearningMode;

type Props = {
  set: StudySet;
  onEdit: () => void;
  onDelete: () => void;
  onBack: () => void;
  // Learning mode handlers - will be filled by Dev 4 integration
  onToggleStar?: (card: Flashcard) => void;
};

export function StudyDetail({ set, onEdit, onDelete, onBack, onToggleStar }: Props) {
  const { token } = useAuth();
  const [studyMode, setStudyMode] = React.useState<StudyMode>("dashboard");
  const [progressStatus, setProgressStatus] = React.useState<ProgressPanelStatus>("idle");
  const [progress, setProgress] = React.useState<ProgressListResponse | null>(null);
  const [progressError, setProgressError] = React.useState("");
  const cards = set.flashcards ?? [];
  const progressHistory = progress?.history ?? [];
  const latestProgress = progressHistory[0] ?? null;

  const loadProgress = React.useCallback(async () => {
    setProgressStatus("loading");
    setProgressError("");
    try {
      const result = await fetchProgressSummary(token, set.id);
      setProgress(result);
      setProgressStatus(result.totalSessions === 0 ? "empty" : "success");
    } catch (error) {
      setProgressError(error instanceof Error ? error.message : "Không tải được tiến độ học.");
      setProgressStatus("error");
    }
  }, [token, set.id]);

  React.useEffect(() => {
    if (studyMode === "dashboard") void loadProgress();
  }, [studyMode, loadProgress]);

  return (
    <>
      <section className="page-heading">
        <div>
          <button className="ghost-button back-btn" onClick={onBack}>← Quay lại</button>
          <p className="eyebrow">Học phần</p>
          <h1>{set.title}</h1>
          <p>{set.description || "Chưa có mô tả"}</p>
        </div>
        <div className="header-actions">
          <button className="secondary-button" onClick={onEdit}>Sửa thẻ</button>
          <button className="danger" onClick={onDelete}>Xóa set</button>
        </div>
      </section>

      <section className="panel study-panel">
        <div className="mode-tabs">
          {(["dashboard", "flashcards", "learn", "test", "match"] as StudyMode[]).map((mode) => (
            <button
              className={studyMode === mode ? "tab active" : "tab"}
              key={mode}
              onClick={() => setStudyMode(mode)}
            >
              {mode}
            </button>
          ))}
        </div>

        {/* Overview tab - card list */}
        {studyMode === "dashboard" && (
          <><div className="card-list">
            {cards.length === 0 && <p className="empty">Chưa có flashcard nào.</p>}
            {cards.map((card) => (
              <article className="mini-card" key={card.id}>
                <div>
                  <strong>{card.term}</strong>
                  <span>{card.definition}</span>
                </div>
                {onToggleStar && (
                  <button onClick={() => onToggleStar(card)}>
                    {card.starred ? "★ Starred" : "☆ Star"}
                  </button>
                )}
              </article>
            ))}
          </div>
          <ProgressPanel
            status={progressStatus}
            errorMessage={progressError}
            onRetry={loadProgress}
            summary={progress ? {
              latestScore: latestProgress?.score ?? null,
              latestTotal: latestProgress?.total ?? null,
              attemptCount: progress.totalSessions,
              latestMode: progress.lastMode,
            } : null}
            history={progressHistory.map((item) => ({
              id: item.id, mode: item.mode, score: item.score,
              total: item.total, completedAt: item.completedAt,
            }))}
          /></>
        )}

        {/* Dev 4 (FE-LEARN-06): LearningContainer nhận set thật từ Dev 3.
            studyMode đã là LearningMode khi !== "dashboard" nhờ type alias trên. */}
        {studyMode !== "dashboard" && (
          <LearningContainer set={set} mode={studyMode} />
        )}
      </section>
    </>
  );
}
