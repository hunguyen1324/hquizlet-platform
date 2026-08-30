// LearningContainer — Dev 4
// P2-LEARN-01..04: Entry-point nhận StudySet + mode, chạy với data thật
// Phase 2: không còn mock fallback, empty states đầy đủ

import React from "react";
import type { StudySet, Flashcard } from "./types";
import type { LearningMode } from "./types";
import { FlashcardsMode } from "./FlashcardsMode";
import { LearnMode } from "./LearnMode";
import { TestMode } from "./TestMode";
import { MatchMode } from "./MatchMode";
import { LearningEmptyState } from "../../components/learning/LearningEmptyState";
import "./learning.css";

type Props = {
  set: StudySet;
  mode: LearningMode;
};

export function LearningContainer({ set, mode }: Props) {
  const cards: Flashcard[] = set.flashcards ?? [];

  if (cards.length === 0) {
    return (
      <LearningEmptyState
        message="Học phần này chưa có thẻ nào."
        hint="Thêm thẻ trong phần 'Sửa thẻ' để bắt đầu học."
      />
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
