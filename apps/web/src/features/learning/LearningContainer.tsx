// LearningContainer — Dev 4
// P2-LEARN-01..04: Entry-point nhận StudySet + mode, chạy với data thật
// P3-LEARN-01..03: Truyền studySetId xuống modes để enable progress save

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
      return <FlashcardsMode cards={cards} studySetId={set.id} />;
    case "learn":
      return <LearnMode cards={cards} studySetId={set.id} />;
    case "test":
      return <TestMode cards={cards} studySetId={set.id} />;
    case "match":
      return <MatchMode cards={cards} studySetId={set.id} />;
    default:
      return null;
  }
}
