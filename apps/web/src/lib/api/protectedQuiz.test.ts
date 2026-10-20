import { describe, expect, it } from "vitest";

import { mergeQuizView, type ProtectedQuizView } from "./protectedQuiz";

const manifest = [
  { number: 1, part: 1, group: 0 },
  { number: 2, part: 1, group: 1 },
];

function view(patch: Partial<ProtectedQuizView>): ProtectedQuizView {
  return {
    id: "session",
    mode: "practice",
    instant: true,
    layout: "default",
    serverTime: "2026-10-20T00:00:00Z",
    submitted: false,
    currentIndex: 0,
    total: 2,
    answered: 0,
    page: [],
    answers: {},
    results: {},
    timeUsed: 0,
    ...patch,
  };
}

describe("mergeQuizView", () => {
  it("keeps the manifest from start when a page response omits it", () => {
    const started = view({ manifest });
    const paged = view({ currentIndex: 1, page: [{ index: 1, question: { questionText: "Q2" } as never, hasAudio: false, revealed: false }] });
    const merged = mergeQuizView(started, paged);
    expect(merged.manifest).toEqual(manifest);
    expect(merged.currentIndex).toBe(1);
    expect(merged.page[0]?.index).toBe(1);
  });

  it("accumulates page-scoped answers and results instead of replacing them", () => {
    const first = mergeQuizView(view({ manifest }), view({ answered: 1, answers: { 0: "A" }, results: { 0: true } }));
    const second = mergeQuizView(first, view({ currentIndex: 1, answered: 2, answers: { 1: "B" }, results: { 1: false } }));
    expect(second.answers).toEqual({ 0: "A", 1: "B" });
    expect(second.results).toEqual({ 0: true, 1: false });
    expect(second.answered).toBe(2);
    expect(second.manifest).toEqual(manifest);
  });

  it("uses the complete maps of a submitted session", () => {
    const submitted = mergeQuizView(
      view({ manifest, answers: { 0: "A" } }),
      view({ submitted: true, score: 1, answered: 1, answers: { 0: "A" }, results: { 0: true } }),
    );
    expect(submitted.submitted).toBe(true);
    expect(submitted.score).toBe(1);
    expect(submitted.results).toEqual({ 0: true });
  });

  it("does not mix state from a different session", () => {
    const merged = mergeQuizView(view({ manifest, answers: { 0: "A" } }), view({ id: "other", answers: {}, answered: 0 }));
    expect(merged.id).toBe("other");
    expect(merged.answers).toEqual({});
    expect(merged.manifest).toBeUndefined();
  });
});
