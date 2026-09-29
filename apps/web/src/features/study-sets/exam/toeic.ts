import type { ExamItem, QuizQuestionLike } from "./types";

export const TOEIC_PART_RANGES = [
  { part: 1, from: 1, to: 6 },
  { part: 2, from: 7, to: 31 },
  { part: 3, from: 32, to: 70 },
  { part: 4, from: 71, to: 100 },
  { part: 5, from: 101, to: 130 },
  { part: 6, from: 131, to: 146 },
  { part: 7, from: 147, to: 200 },
] as const;

export const PART_LABELS: Record<number, string> = {
  1: "Part 1: Photographs",
  2: "Part 2: Question-Response",
  3: "Part 3: Conversations",
  4: "Part 4: Talks",
  5: "Part 5: Incomplete Sentences",
  6: "Part 6: Text Completion",
  7: "Part 7: Reading Comprehension",
};

export const PART_DIRECTIONS: Record<number, string> = {
  1: "Directions: For each question in this part, you will hear four statements about the picture in your test book. When you hear the statements, you must select the one statement that best describes what you see in the picture. Then find the number of the question on your answer sheet and mark your answer. The statements will not be printed in your test book and will be spoken only one time.",
  2: "Directions: You will hear a question or statement and three responses spoken in English. They will not be printed in your test book and will be spoken only one time. Select the best response to the question or statement and mark the letter (A), (B), or (C) on your answer sheet.",
  3: "Directions: You will hear some conversations between two or more people. You will be asked to answer three questions about what the speakers say in each conversation. Select the best response to each question and mark the letter (A), (B), (C), or (D) on your answer sheet. The conversations will not be printed in your test book and will be spoken only one time.",
  4: "Directions: You will hear some talks given by a single speaker. You will be asked to answer three questions about what the speaker says in each talk. Select the best response to each question and mark the letter (A), (B), (C), or (D) on your answer sheet. The talks will not be printed in your test book and will be spoken only one time.",
  5: "Directions: A word or phrase is missing in each of the sentences below. Four answer choices are given below each sentence. Select the best answer to complete the sentence. Then mark the letter (A), (B), (C), or (D) on your answer sheet.",
  6: "Directions: Read the texts that follow. A word or phrase is missing in some of the sentences beneath each of the texts. Four answer choices are given below each of the sentences. Select the best answer to complete the sentence. Then mark the letter (A), (B), (C), or (D) on your answer sheet.",
  7: "Directions: In this part you will read a selection of texts, such as magazine and newspaper articles, e-mails, and instant messages. Each text is followed by several questions. Select the best answer for each question and mark the letter (A), (B), (C), or (D) on your answer sheet.",
};

export function isListeningPart(part: number): boolean {
  return part <= 4;
}

function normalize(value: string) {
  return value.trim().toLocaleLowerCase();
}

function correctOptionText(question: QuizQuestionLike): string {
  const marked = question.options?.find((option) => option.isCorrect)?.text;
  if (marked) return marked;
  const ca = question.correctAnswer ?? "";
  const index = /^[a-d]$/i.test(ca) ? ca.toUpperCase().charCodeAt(0) - 65 : -1;
  return index >= 0 ? question.options?.[index]?.text ?? ca : ca;
}

export function isAnswerCorrect(selected: string | null, question: QuizQuestionLike): boolean {
  if (selected == null || !selected.trim()) return false;
  if (question.questionType === "multiple_choice" || question.questionType === "true_false") {
    if (question.options?.find((option) => option.text === selected)?.isCorrect) return true;
    return normalize(selected) === normalize(correctOptionText(question));
  }
  return normalize(selected) === normalize(question.correctAnswer ?? "");
}

export function partFromTags(tags?: string[]): number | null {
  if (!tags) return null;
  for (const tag of tags) {
    const m = /^part[\s_-]*([1-7])$/.exec(tag.trim().toLowerCase());
    if (m) return Number(m[1]);
  }
  return null;
}

export function partFromQuestionNumber(n: number): number {
  for (const r of TOEIC_PART_RANGES) {
    if (n >= r.from && n <= r.to) return r.part;
  }
  return 7;
}

function partForQuestion(q: QuizQuestionLike, questionNumber: number): number {
  const fromTags = partFromTags(q.tags);
  if (fromTags !== null) return fromTags;
  return partFromQuestionNumber(questionNumber);
}

export function buildExamItems(questions: QuizQuestionLike[]): ExamItem[] {
  const items: ExamItem[] = [];

  questions.forEach((q, qi) => {
    let subs: QuizQuestionLike[] = [];
    if (q.questionType === "paragraph") {
      try {
        subs =
          typeof q.subQuestions === "string"
            ? (JSON.parse(q.subQuestions) as QuizQuestionLike[])
            : (q.subQuestions ?? []);
      } catch {
        subs = [];
      }
    }

    subs = Array.isArray(subs) ? subs.filter((sub) => sub && typeof sub === "object") : [];
    if (q.questionType === "paragraph" && subs.length > 0) {
      subs.forEach((sub, si) => {
        const number = items.length + 1;
        items.push({
          key: `${q.id ?? qi}-sub-${si}`,
          groupKey: `parent-${q.id ?? qi}`,
          part: partFromTags(sub.tags) ?? partForQuestion(q, number),
          question: sub,
          parent: q,
          questionNumber: number,
        });
      });
    } else {
      const number = items.length + 1;
      items.push({
        key: `q-${q.id ?? qi}`,
        groupKey: `parent-${q.id ?? qi}`,
        part: partForQuestion(q, number),
        question: q,
        questionNumber: number,
      });
    }
  });

  return items;
}
