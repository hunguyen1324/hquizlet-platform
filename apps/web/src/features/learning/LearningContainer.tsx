// LearningContainer — Dev 4
// FE-LEARN-06: Wraps all 4 learning modes, receives StudySet data from Dev 3
// Drop-in replacement for the placeholder divs in StudyDetail.tsx (Dev 3)
// Props aligned with remote branch: modes accept studySet: StudySet

import React from "react";
import type { StudySet, Flashcard } from "./types";
import type { LearningMode } from "./types";
import { FlashcardsMode } from "./FlashcardsMode";
import { LearnMode } from "./LearnMode";
import { TestMode } from "./TestMode";
import { MatchMode } from "./MatchMode";
import "./learning.css";

type Props = {
  set: StudySet;
  mode: LearningMode;
  onToggleStar?: (card: Flashcard) => void;
};

export function LearningContainer({ set, mode, onToggleStar }: Props) {
  const cards = set.flashcards ?? [];

  if (cards.length === 0) {
    return (
      <div className="learning-empty">
        <p>Học phần này chưa có thẻ nào. Hãy thêm thẻ trước khi học.</p>
      </div>
    );
  }

  switch (mode) {
    case "flashcards":
      return <FlashcardsMode studySet={set} onToggleStar={onToggleStar} />;
    case "learn":
      return <LearnMode studySet={set} />;
    case "test":
      return <TestMode studySet={set} />;
    case "match":
      return <MatchMode studySet={set} />;
    default:
      return null;
  }
}
