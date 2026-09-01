// LearningRouter — Dev 4
// P2: production learning entrypoint, no mock fallback.
// P3-LEARN-01..03: Truyền studySetId xuống 4 modes để enable progress save.

import React, { useEffect, useState } from "react";
import type { LearningMode, StudySet, Flashcard } from "./types";
import { FlashcardsMode } from "./FlashcardsMode";
import { LearnMode } from "./LearnMode";
import { TestMode } from "./TestMode";
import { MatchMode } from "./MatchMode";
import { LearningEmptyState } from "../../components/learning/LearningEmptyState";
import "./learning.css";

type Props = {
  studySet: StudySet;
  initialMode?: LearningMode;
  onToggleStar?: (card: Flashcard) => void;
  onBack?: () => void;
};

const MODES: { id: LearningMode; label: string }[] = [
  { id: "flashcards", label: "Flashcards" },
  { id: "learn", label: "Học" },
  { id: "test", label: "Kiểm tra" },
  { id: "match", label: "Ghép cặp" },
];

export function LearningRouter({ studySet, initialMode = "flashcards", onBack }: Props) {
  const [mode, setMode] = useState<LearningMode>(initialMode);
  const cards = studySet.flashcards ?? [];
  const standalone = Boolean(onBack);

  useEffect(() => {
    setMode(initialMode);
  }, [initialMode]);

  return (
    <div className={standalone ? "learning-shell" : "learning-content-only"}>
      {standalone && (
        <div className="learning-header">
          <div className="learning-title-row">
            <button className="ghost-button" onClick={onBack}>← Quay lại</button>
            <h1 className="learning-title">{studySet.title}</h1>
          </div>
          <p className="learning-subtitle">
            {cards.length} thẻ • {studySet.description || "Học phần"}
          </p>
        </div>
      )}

      {standalone && (
        <div className="learning-mode-tabs" role="tablist" aria-label="Chế độ học">
          {MODES.map(({ id, label }) => (
            <button
              key={id}
              role="tab"
              aria-selected={mode === id}
              className={`mode-tab ${mode === id ? "mode-tab--active" : ""}`}
              onClick={() => setMode(id)}
            >
              {label}
            </button>
          ))}
        </div>
      )}

      {cards.length === 0 ? (
        <LearningEmptyState
          message="Học phần này chưa có thẻ nào."
          hint="Thêm thẻ trong phần 'Sửa thẻ' để bắt đầu học."
        />
      ) : (
        <div className="learning-content">
          {mode === "flashcards" && <FlashcardsMode cards={cards} studySetId={studySet.id} />}
          {mode === "learn" && <LearnMode cards={cards} studySetId={studySet.id} />}
          {mode === "test" && <TestMode cards={cards} studySetId={studySet.id} />}
          {mode === "match" && <MatchMode cards={cards} studySetId={studySet.id} />}
        </div>
      )}
    </div>
  );
}
