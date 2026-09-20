// LearningContainer — Dev 4
// P2-LEARN-01..04: Entry-point nhận StudySet + mode, chạy với data thật
// P3-LEARN-01..03: Truyền studySetId xuống modes để enable progress save
//
// FIX: nhận cards và totalCount riêng thay vì đọc set.flashcards,
// vì set.flashcards chỉ là initial payload (bị giới hạn trang đầu).
// StudyDetail phải truyền allCards (đã infinite-scroll đầy đủ) xuống đây.

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
  /** Toàn bộ thẻ đã load (infinite-scroll), do parent quản lý */
  cards: Flashcard[];
  /** Tổng thẻ thật từ server (để progress bar tính đúng) */
  totalCount: number;
};

export function LearningContainer({ set, mode, cards, totalCount }: Props) {
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
      return <FlashcardsMode cards={cards} studySetId={set.id} totalCount={totalCount} />;
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
