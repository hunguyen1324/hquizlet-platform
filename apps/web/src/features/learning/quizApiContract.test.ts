import { describe, expect, it } from "vitest";
import type { QuizAnswer } from "../../lib/api/client";

describe("Phase 4 quiz contract", () => {
  it("uses flashcard identity rather than client-provided correct answers", () => {
    const answer: QuizAnswer = {
      flashcardId: 42,
      answer: "user response",
      attempts: 1,
      responseTimeMs: 250,
    };
    expect(answer.flashcardId).toBe(42);
    expect(answer).not.toHaveProperty("correct");
    expect(answer).not.toHaveProperty("definition");
  });

  it("accepts match identity without a frontend-generated pair truth", () => {
    const answer: QuizAnswer = {
      flashcardId: 42,
      pairId: "pair-42",
      matchedFlashcardId: 42,
      attempts: 2,
    };
    expect(answer.matchedFlashcardId).toBe(answer.flashcardId);
    expect(answer).not.toHaveProperty("correct");
  });
});
