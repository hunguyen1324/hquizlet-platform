// Learning feature types — Dev 4
// Aligned with shared types.ts (Dev 3) and OpenAPI contract (Dev 5)
// Fields: term, definition, starred match contract agreed with Dev 5
//
// IMPORTANT: LearningMode and LearningProgress are canonical in lib/api/client.ts (Dev 5).
// Do NOT re-declare them here — import from there to avoid type conflicts.

import type { StudySet, Flashcard } from "../../types";
import type { LearningMode } from "../../lib/api/client";

export type { StudySet, Flashcard, LearningMode };

// Session-local progress state for a single flashcard (not persisted, UI-only)
export type CardProgress = {
  cardId: number;
  seen: boolean;
  correct: boolean;
  attempts: number;
};

// Flashcards mode state
export type FlashcardsState = {
  cards: Flashcard[];
  currentIndex: number;
  flipped: boolean;
  shuffled: boolean;
};

// Learn mode: question/answer
export type LearnQuestion = {
  card: Flashcard;
  userAnswer: string;
  submitted: boolean;
  correct: boolean | null;
};

export type LearnState = {
  queue: Flashcard[];
  currentIndex: number;
  answers: Record<number, LearnQuestion>;
  done: boolean;
};

// Test mode: multiple choice or written
export type TestQuestion = {
  card: Flashcard;
  choices: string[]; // includes definition + 3 distractors
  userAnswer: string | null;
  correct: boolean | null;
};

export type TestState = {
  questions: TestQuestion[];
  currentIndex: number;
  submitted: boolean;
  score: number;
};

// Match mode: click-pair game
export type MatchItem = {
  id: string;       // e.g. "card-1-term" | "card-1-def"
  cardId: number;
  text: string;
  type: "term" | "definition";
  matched: boolean;
  selected: boolean;
};

export type MatchState = {
  items: MatchItem[];
  selectedId: string | null;
  matchedPairs: number;
  totalPairs: number;
  startedAt: number;
  finishedAt: number | null;
};
