// LearningContainer — Dev 4
// FE-LEARN-06: Wraps all 4 learning modes, receives StudySet data from Dev 3
// Drop-in replacement for the placeholder divs in StudyDetail.tsx (Dev 3)

import React from "react";
import type { StudySet } from "../../types";
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
