// LearningContainer — Dev 4
// FE-LEARN-06: Alternative entry-point for LearningRouter (Dev 3 integration)
// Accepts StudySet + mode, extracts cards, delegates to each mode component
// Props: { set: StudySet, mode: LearningMode }

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
};

export function LearningContainer({ set, mode }: Props) {
  const cards: Flashcard[] = set.flashcards ?? [];

  if (cards.length === 0) {
    return (
      <div className="learning-empty">
        <p>Học phần này chưa có thẻ nào. Hãy thêm thẻ trước khi học.</p>
      </div>
    );
  }

  switch (mode) {
    case "flashcards":
      return <FlashcardsMode cards={cards} />;
    case "learn":
      return <LearnMode cards={cards} />;
    case "test":
      return <TestMode cards={cards} />;
    case "match":
      return <MatchMode cards={cards} />;
    default:
      return null;
  }
}
