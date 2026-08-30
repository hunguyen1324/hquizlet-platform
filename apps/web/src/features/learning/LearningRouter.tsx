// LearningRouter - Dev4
// Phan phoi mode dung, nhan studySet tu prop (co the la real hoac mock)
// FE-LEARN-06: khi Dev3 co study detail provider, swap MOCK_STUDY_SET -> real data
import React, { useState } from "react";
import type { LearningMode, StudySet, Flashcard } from "./types";
import { FlashcardsMode } from "./FlashcardsMode";
import { LearnMode } from "./LearnMode";
import { TestMode } from "./TestMode";
import { MatchMode } from "./MatchMode";
import { MOCK_STUDY_SET } from "./mockData";

type Props = {
  // studySet: real data tu Dev3 StudyDetail, fallback mock neu undefined
  studySet?: StudySet;
  // initialMode: truyen tu Dev3's tab state (Dev3 owns tab switching)
  initialMode?: LearningMode;
  onToggleStar?: (card: Flashcard) => void;
  onBack?: () => void;
};

export function LearningRouter({ studySet, initialMode = "flashcards", onToggleStar, onBack }: Props) {
  // Fallback to mock data if no real data yet (Sprint 1 - no backend dep)
  const activeSet: StudySet = studySet ?? MOCK_STUDY_SET;

  // mode driven by initialMode from Dev3 tabs (no internal tab state when integrated)
  // when used standalone (onBack present), allow internal mode switching
  const [internalMode, setInternalMode] = useState<LearningMode>(initialMode);
  const standalone = Boolean(onBack);

  // sync khi Dev3 doi tab tu ngoai
  React.useEffect(() => {
    setInternalMode(initialMode);
  }, [initialMode]);

  const MODES: { id: LearningMode; label: string }[] = [
    { id: "flashcards", label: "Flashcards" },
    { id: "learn", label: "Hoc" },
    { id: "test", label: "Kiem tra" },
    { id: "match", label: "Ghep cap" },
  ];

  return (
    <div className={standalone ? "learning-shell" : "learning-content-only"}>
      {standalone && (
        <div className="learning-header">
          <div className="learning-title-row">
            <button className="ghost-button" onClick={onBack}>← Quay lai</button>
            <h1 className="learning-title">{activeSet.title}</h1>
            {!studySet && <span className="mock-badge">mock data</span>}
          </div>
          <p className="learning-subtitle">
            {activeSet.flashcards?.length ?? 0} the • {activeSet.description || "Hoc phan"}
          </p>
        </div>
      )}

      {standalone && (
        <div className="learning-mode-tabs">
          {MODES.map(({ id, label }) => (
            <button
              key={id}
              className={`mode-tab ${internalMode === id ? "mode-tab--active" : ""}`}
              onClick={() => setInternalMode(id)}
            >
              {label}
            </button>
          ))}
        </div>
      )}

      <div className="learning-content">
        {internalMode === "flashcards" && (
          <FlashcardsMode cards={activeSet.flashcards ?? []} />
        )}
        {internalMode === "learn" && <LearnMode cards={activeSet.flashcards ?? []} />}
        {internalMode === "test" && <TestMode cards={activeSet.flashcards ?? []} />}
        {internalMode === "match" && <MatchMode cards={activeSet.flashcards ?? []} />}
      </div>
    </div>
  );
}
