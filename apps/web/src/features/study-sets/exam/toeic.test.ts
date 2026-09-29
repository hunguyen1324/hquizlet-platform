import { describe, expect, it } from "vitest";

import { buildExamItems, isAnswerCorrect } from "./toeic";
import type { QuizQuestionLike } from "./types";

describe("TOEIC exam mapping", () => {
  it("keeps paragraph sub-questions in one group and honors a part tag", () => {
    const questions: QuizQuestionLike[] = [
      {
        id: 10,
        questionType: "paragraph",
        questionText: "Read the email.",
        paragraphText: "Hello team...",
        tags: ["part7"],
        subQuestions: [
          { questionType: "multiple_choice", questionText: "Question one", correctAnswer: "A" },
          { questionType: "multiple_choice", questionText: "Question two", correctAnswer: "B" },
        ],
      },
    ];

    expect(buildExamItems(questions)).toMatchObject([
      { groupKey: "parent-10", part: 7, questionNumber: 1 },
      { groupKey: "parent-10", part: 7, questionNumber: 2 },
    ]);
  });

  it("accepts both an option marked correct and a letter-based answer", () => {
    const marked: QuizQuestionLike = {
      questionType: "multiple_choice",
      questionText: "Marked answer",
      options: [
        { text: "Wrong" },
        { text: "Right", isCorrect: true },
      ],
    };
    const letter: QuizQuestionLike = {
      questionType: "multiple_choice",
      questionText: "Letter answer",
      correctAnswer: "B",
      options: [{ text: "Wrong" }, { text: "Right" }],
    };

    expect(isAnswerCorrect("Right", marked)).toBe(true);
    expect(isAnswerCorrect("Right", letter)).toBe(true);
    expect(isAnswerCorrect("Wrong", letter)).toBe(false);
  });
});


describe("Part filtering metadata", () => {
  it("uses each child Part before its parent and keeps original numbers after filtering", () => {
    const items = buildExamItems([{ questionType: "paragraph", questionText: "", tags: ["part7"], subQuestions: [
      { questionType: "multiple_choice", questionText: "one", tags: ["Part 3"] },
      { questionType: "multiple_choice", questionText: "two", tags: ["part_4"] },
      { questionType: "multiple_choice", questionText: "three" },
    ] }]);
    expect(items.map((item) => item.part)).toEqual([3, 4, 7]);
    expect(items.filter((item) => item.part === 4)[0]?.questionNumber).toBe(2);
  });
});
