// Dev 4 - Learning feature public exports - Phase 2
// LearningMode and LearningProgress are canonical in lib/api/client.ts (Dev 5).
// Import them from there, not from this module.

export { FlashcardsMode } from "./FlashcardsMode";
export { LearnMode } from "./LearnMode";
export { TestMode } from "./TestMode";
export { MatchMode } from "./MatchMode";
export { LearningContainer } from "./LearningContainer";
export { LearningRouter } from "./LearningRouter";

// Session-local UI types only (not duplicates of client.ts types)
export type {
  CardProgress,
  FlashcardsState,
  LearnQuestion,
  LearnState,
  TestQuestion,
  TestState,
  MatchItem,
  MatchState,
} from "./types";

// Progress API stubs and Phase 3 request types
export type { ProgressSaveRequest, CardResult } from "./progressContract";
export { saveProgress, fetchProgress } from "./progressContract";
