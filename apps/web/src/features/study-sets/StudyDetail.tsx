// StudyDetail - Dev 3
// Shows study set detail page. Learning modes (flashcards, learn, test, match) are Dev 4's scope.
// This component provides the study set data to Dev 4's learning feature via props/context.

import React from "react";
import type { StudySet, Flashcard } from "../../types";

type StudyMode = "dashboard" | "flashcards" | "learn" | "test" | "match";

type Props = {
  set: StudySet;
  onEdit: () => void;
  onDelete: () => void;
  onBack: () => void;
  // Learning mode handlers - will be filled by Dev 4 integration
  onToggleStar?: (card: Flashcard) => void;
};

export function StudyDetail({ set, onEdit, onDelete, onBack, onToggleStar }: Props) {
  const [studyMode, setStudyMode] = React.useState<StudyMode>("dashboard");
  const cards = set.flashcards ?? [];

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
          <div className="card-list">
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
        )}

        {/* Learning modes: Dev 4 will replace these placeholders */}
        {studyMode !== "dashboard" && (
          <div className="mode-placeholder">
            <p>
              Chế độ <strong>{studyMode}</strong> — Dev 4 đang phát triển.
            </p>
            <p style={{ fontSize: "0.85rem", color: "#888" }}>
              ({cards.length} thẻ sẵn sàng để dùng)
            </p>
          </div>
        )}
      </section>
    </>
  );
}
