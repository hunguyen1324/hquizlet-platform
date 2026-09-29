import type { QuizQuestion, QuizQuestionType } from "../../../types";

export type QuizQuestionLike = {
  id?: number;
  questionType: QuizQuestionType;
  questionText: string;
  correctAnswer?: string | null;
  answerExplanation?: string | null;
  paragraphText?: string | null;
  tags?: string[];
  options?: { id?: number; text: string; position?: number; isCorrect?: boolean }[];
  subQuestions?: QuizQuestionLike[] | string | null;
  timeInSeconds?: number | null;
  imageUrl?: string | null;
  audioUrl?: string | null;
  srtUrl?: string | null;
};

export type ExamItem = {
  key: string;
  groupKey: string;
  part: number;
  question: QuizQuestionLike;
  parent?: QuizQuestionLike;
  questionNumber: number;
};

export type ExamResult = {
  questionIndex: number;
  selectedAnswer: string | null;
  correctAnswer: string;
  isCorrect: boolean;
};

export function quizQuestionToLike(q: QuizQuestion): QuizQuestionLike {
  return {
    id: q.id,
    questionType: q.questionType,
    questionText: q.questionText,
    correctAnswer: q.correctAnswer,
    answerExplanation: q.answerExplanation,
    paragraphText: q.paragraphText,
    tags: q.tags,
    options: q.options,
    subQuestions: q.subQuestions as QuizQuestionLike[] | string | null | undefined,
    timeInSeconds: q.timeInSeconds,
    imageUrl: q.imageUrl,
    audioUrl: q.audioUrl,
    srtUrl: q.srtUrl,
  };
}
