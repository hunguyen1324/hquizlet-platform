// StudyDetail - Dev 3 + Dev 4 integration
// Dev 4 (FE-LEARN-06): LearningContainer gắn thật vào các tab learning mode

import React from "react";
import type { StudySet, Flashcard } from "../../types";
import type { LearningMode } from "../learning/types";
import { LearningContainer } from "../learning";
import { fetchProgressSummary, type ProgressListResponse } from "../learning/progressContract";
import { useAuth } from "../auth/AuthContext";
import { ProgressPanel, type ProgressPanelStatus } from "../../components/progress";
import "./StudyDetail.css";

type StudyMode = "dashboard" | LearningMode;

type Props = {
  set: StudySet;
  onEdit: () => void;
  onDelete: () => void;
  onBack: () => void;
  onToggleStar?: (card: Flashcard) => void;
};

const MODE_CONFIG: { mode: StudyMode; icon: string; label: string }[] = [
  { mode: "flashcards", icon: "▤", label: "Thẻ ghi nhớ" },
  { mode: "learn",      icon: "✦", label: "Học" },
  { mode: "test",       icon: "✓", label: "Kiểm tra" },
  { mode: "match",      icon: "⌘", label: "Ghép thẻ" },
];

export function StudyDetail({ set, onEdit, onDelete, onBack, onToggleStar }: Props) {
  const { token } = useAuth();
  const [studyMode, setStudyMode] = React.useState<StudyMode>("dashboard");
  const [progressStatus, setProgressStatus] = React.useState<ProgressPanelStatus>("idle");
  const [progress, setProgress] = React.useState<ProgressListResponse | null>(null);
  const [progressError, setProgressError] = React.useState("");
  const [sortOrder, setSortOrder] = React.useState<"original" | "alphabetical">("original");
  const [menuOpen, setMenuOpen] = React.useState(false);
  const menuRef = React.useRef<HTMLDivElement>(null);

  // Close dropdown on outside click
  React.useEffect(() => {
    if (!menuOpen) return;
    const handler = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [menuOpen]);

  const cards = set.flashcards ?? [];
  const progressHistory = progress?.history ?? [];
  const latestProgress = progressHistory[0] ?? null;

  const sortedCards = React.useMemo(() => {
    if (sortOrder === "alphabetical") {
      return [...cards].sort((a, b) => a.term.localeCompare(b.term));
    }
    return cards;
  }, [cards, sortOrder]);

  const totalItems =
    (set.contentType === "quiz" ? (set.quizQuestions?.length ?? 0) : cards.length);

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
    <div className="sd-page">
      {/* ── Page header ── */}
      <div className="sd-header">
        <button className="sd-back" onClick={onBack}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"><path d="M19 12H5"/><path d="M12 19l-7-7 7-7"/></svg>
          Quay lại
        </button>

        <div className="sd-title-row">
          <div className="sd-title-copy">
            <div className="sd-eyebrow">
              {set.visibility === "private" && <span className="sd-private-badge">🔒 Riêng tư</span>}
              <span>
                {set.contentType === "quiz" ? "Quiz" : set.contentType === "grammar" ? "Ngữ pháp" : "Học phần"}
              </span>
              <span className="sd-dot">·</span>
              <span>{totalItems} thuật ngữ</span>
            </div>
            <h1 className="sd-title">{set.title}</h1>
            {set.description && <p className="sd-description">{set.description}</p>}
          </div>

          {/* ── More (···) menu ── */}
          <div className="sd-more-wrap" ref={menuRef}>
            <button
              className="sd-more-btn"
              onClick={() => setMenuOpen((v) => !v)}
              aria-label="Xem thêm"
              title="Xem thêm"
            >
              <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor">
                <circle cx="5" cy="12" r="2"/><circle cx="12" cy="12" r="2"/><circle cx="19" cy="12" r="2"/>
              </svg>
            </button>

            {menuOpen && (
              <div className="sd-dropdown">
                <button
                  className="sd-dropdown-item"
                  onClick={() => { setMenuOpen(false); onEdit(); }}
                >
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
                  Sửa
                </button>
                <div className="sd-dropdown-divider" />
                <button
                  className="sd-dropdown-item sd-dropdown-item--danger"
                  onClick={() => { setMenuOpen(false); onDelete(); }}
                >
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/><path d="M10 11v6"/><path d="M14 11v6"/><path d="M9 6V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/></svg>
                  Xóa
                </button>
              </div>
            )}
          </div>
        </div>

        {/* ── Mode buttons (Quizlet style) ── */}
        {studyMode === "dashboard" && (
          <div className="sd-mode-grid">
            {MODE_CONFIG.map(({ mode, icon, label }) => (
              <button
                key={mode}
                className="sd-mode-btn"
                onClick={() => setStudyMode(mode)}
              >
                <span className="sd-mode-icon">{icon}</span>
                <span className="sd-mode-label">{label}</span>
              </button>
            ))}
          </div>
        )}

        {/* ── Sub-nav tabs (visible in learning modes) ── */}
        {studyMode !== "dashboard" && (
          <div className="sd-subnav">
            <button
              className="sd-subnav-tab"
              onClick={() => setStudyMode("dashboard")}
            >
              Tổng quan
            </button>
            {MODE_CONFIG.map(({ mode, label }) => (
              <button
                key={mode}
                className={`sd-subnav-tab ${studyMode === mode ? "active" : ""}`}
                onClick={() => setStudyMode(mode)}
              >
                {label}
              </button>
            ))}
          </div>
        )}
      </div>

      {/* ── Learning modes ── */}
      {studyMode !== "dashboard" && (
        <div className="sd-learning-wrapper">
          <LearningContainer set={set} mode={studyMode} />
        </div>
      )}

      {/* ── Overview / Dashboard ── */}
      {studyMode === "dashboard" && (
        <div className="sd-overview">

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
          />

          {/* Flashcard term list */}
          {(set.contentType === "flashcard" || !set.contentType) && (
            <div className="sd-termlist">
              <div className="sd-termlist-header">
                <h2 className="sd-termlist-title">
                  Thuật ngữ trong học phần này
                  <span className="sd-termlist-count">({cards.length})</span>
                </h2>
                <div className="sd-termlist-controls">
                  <select
                    className="sd-sort-select"
                    value={sortOrder}
                    onChange={(e) => setSortOrder(e.target.value as "original" | "alphabetical")}
                  >
                    <option value="original">Thứ tự gốc</option>
                    <option value="alphabetical">A → Z</option>
                  </select>
                </div>
              </div>

              {cards.length === 0 && (
                <div className="sd-empty">
                  <span>📭</span>
                  <p>Chưa có thẻ học nào.</p>
                </div>
              )}

              <div className="sd-cards">
                {sortedCards.map((card) => (
                  <article className="sd-card" key={card.id}>
                    {card.imageUrl && (
                      <div className="sd-card-image">
                        <img src={card.imageUrl} alt={card.term} />
                      </div>
                    )}
                    <div className="sd-card-body">
                      <div className="sd-card-term">{card.term}</div>
                      <div className="sd-card-divider" />
                      <div className="sd-card-definition">{card.definition}</div>
                      {card.exampleSentence && (
                        <div className="sd-card-example">{card.exampleSentence}</div>
                      )}
                    </div>
                    <div className="sd-card-actions">
                      {onToggleStar && (
                        <button
                          className={`sd-star-btn ${card.starred ? "starred" : ""}`}
                          onClick={() => onToggleStar(card)}
                          title={card.starred ? "Bỏ đánh dấu" : "Đánh dấu"}
                        >
                          {card.starred ? "★" : "☆"}
                        </button>
                      )}
                    </div>
                  </article>
                ))}
              </div>
            </div>
          )}

          {/* Quiz question list */}
          {set.contentType === "quiz" && set.quizQuestions && set.quizQuestions.length > 0 && (
            <div className="sd-termlist">
              <div className="sd-termlist-header">
                <h2 className="sd-termlist-title">
                  Câu hỏi Quiz
                  <span className="sd-termlist-count">({set.quizQuestions.length})</span>
                </h2>
              </div>
              <div className="sd-cards">
                {set.quizQuestions.map((q, i) => (
                  <article className="sd-card" key={q.id ?? i}>
                    <div className="sd-card-body">
                      <div className="sd-card-term">{i + 1}. {q.questionText}</div>
                      <div className="sd-card-divider" />
                      <span className="sd-quiz-type-badge">
                        {q.questionType === "multiple_choice" ? "Trắc nghiệm" :
                         q.questionType === "true_false" ? "Đúng/Sai" :
                         q.questionType === "written" ? "Tự luận" :
                         q.questionType === "paragraph" ? "Đoạn văn" : "Sắp xếp"}
                        {q.audioUrl && " · 🔊"}
                      </span>
                    </div>
                  </article>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
