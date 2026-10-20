import { describe, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import type { ProtectedQuizView } from "../../../lib/api/protectedQuiz";
import { nextPageIndex, prevPageIndex } from "./useProtectedQuiz";
import { correctText, formatClock, isCorrectOption, timeAgo } from "./quizHelpers";
import { QuizQuestionView } from "./QuizQuestionView";
import { QuizStartScreen } from "./QuizStartScreen";
import { QuizResult } from "./QuizResult";

vi.mock("../../../components/learning/ViewerWatermark", () => ({ ViewerWatermark: () => null }));

const base = { id: "s", mode: "practice" as const, instant: true, serverTime: "2026-10-18T00:00:00Z", submitted: false, answers: {}, results: {}, timeUsed: 0, page: [] };
const toeic = (currentIndex: number): ProtectedQuizView => ({
  ...base, layout: "toeic", currentIndex,
  manifest: [{ number: 1, part: 3, group: 0 }, { number: 2, part: 3, group: 0 }, { number: 3, part: 3, group: 0 }, { number: 4, part: 4, group: 1 }, { number: 5, part: 4, group: 1 }, { number: 6, part: 5, group: 2 }],
});

describe("page navigation", () => {
  it("moves a whole passage at a time in the TOEIC layout", () => {
    expect(nextPageIndex(toeic(0))).toBe(3);
    expect(nextPageIndex(toeic(1))).toBe(3);
    expect(nextPageIndex(toeic(4))).toBe(5);
    expect(nextPageIndex(toeic(5))).toBeNull();
    expect(prevPageIndex(toeic(5))).toBe(3);
    expect(prevPageIndex(toeic(3))).toBe(0);
    expect(prevPageIndex(toeic(0))).toBeNull();
  });
  it("moves one question at a time in the default layout", () => {
    const view = { ...toeic(2), layout: "default" as const };
    expect(nextPageIndex(view)).toBe(3);
    expect(prevPageIndex(view)).toBe(1);
  });
});

describe("answer helpers", () => {
  const q = { questionType: "multiple_choice" as const, questionText: "?", position: 0, correctAnswer: "B", options: [
    { text: "no", position: 0, isCorrect: false }, { text: "yes", position: 1, isCorrect: false },
  ] };
  it("resolves a letter answer to the option text", () => {
    expect(correctText(q)).toBe("yes");
    expect(isCorrectOption(q, "yes")).toBe(true);
    expect(isCorrectOption(q, "no")).toBe(false);
  });
  it("formats clocks and relative times", () => {
    expect(formatClock(65)).toBe("01:05");
    expect(formatClock(3725)).toBe("1:02:05");
    expect(timeAgo("2026-10-18T00:00:00Z", Date.parse("2026-10-18T02:30:00Z"))).toBe("2 giờ trước");
  });
});

describe("rendering", () => {
  const item = { index: 0, hasAudio: false, revealed: false, question: { questionType: "multiple_choice" as const, questionText: "Pick one", position: 0, options: [
    { text: "alpha", position: 0, isCorrect: false }, { text: "beta", position: 1, isCorrect: false },
  ] } };
  it("shows lettered choices and keeps them clickable until answered", () => {
    const html = renderToStaticMarkup(<QuizQuestionView item={item} number={1} total={2} instant submitted={false} onAnswer={() => undefined} />);
    expect(html).toContain("alpha");
    expect(html).toContain(">A<");
    expect(html).not.toContain("disabled");
  });
  it("marks the right and wrong choice once revealed, and locks them", () => {
    const revealed = { ...item, revealed: true, question: { ...item.question, answerExplanation: "Because.", options: [{ text: "alpha", position: 0, isCorrect: true }, { text: "beta", position: 1, isCorrect: false }] } };
    const html = renderToStaticMarkup(<QuizQuestionView item={revealed} number={1} total={2} answer="beta" result={false} instant submitted={false} onAnswer={() => undefined} />);
    expect(html).toContain("qz-choice correct");
    expect(html).toContain("qz-choice selected wrong");
    expect(html).toContain("Chưa đúng");
    expect(html).toContain("Because.");
    expect(html).toContain("disabled");
  });
  it("offers to resume an unfinished session", () => {
    const html = renderToStaticMarkup(<QuizStartScreen summary={{ total: 10, parts: {}, sessions: [], activeCount: 1, limit: 5, studySetId: 1 }}
      recent={[{ id: "a", studySetId: 1, mode: "exam", instant: false, layout: "default", startedAt: "2026-10-18T00:00:00Z", expiresAt: "2099-01-01T00:00:00Z", activeUntil: "2099-01-01T00:00:00Z", updatedAt: new Date().toISOString(), answered: 4, total: 10 }]}
      savedId={null} settings={{ layout: "default", part: 0, instant: true, minutes: 120, timedPractice: false }} setSettings={() => undefined}
      busy={false} error="" sessionLimitError={false} onStart={() => undefined} onOpen={() => undefined} studySetId={1} />);
    expect(html).toContain("Làm tiếp thi thử · 4/10 câu");
    expect(html).toContain("Luyện tập");
  });
  it("summarises a finished quiz", () => {
    const html = renderToStaticMarkup(<QuizResult view={{ ...toeic(0), layout: "default", submitted: true, score: 3, timeUsed: 125, answers: { 0: "a" }, results: { 0: true } }} onReview={() => undefined} onRestart={() => undefined} />);
    expect(html).toContain("Đúng 3/6 câu");
    expect(html).toContain("2 phút 5 giây");
  });
});
