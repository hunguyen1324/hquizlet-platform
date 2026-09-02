// LearnModePreview — preview a multiple-choice question from the most recent study set
import React, { useMemo } from "react";
import type { StudySet, Flashcard } from "../../types";
import { Card } from "../ui";

type Props = {
  sets: StudySet[];
  onOpenSet: (id: number) => void;
};

function shuffle<T>(arr: T[]): T[] {
  const a = [...arr];
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [a[i], a[j]] = [a[j], a[i]];
  }
  return a;
}

export function LearnModePreview({ sets, onOpenSet }: Props) {
  const preview = useMemo(() => {
    // Find a set with flashcards
    const setWithCards = sets.find((s) => s.flashcards && s.flashcards.length >= 2);
    if (!setWithCards || !setWithCards.flashcards) return null;

    const cards = setWithCards.flashcards;
    const correctCard = cards[0];
    const wrongCards = shuffle(cards.filter((c) => c.id !== correctCard.id)).slice(0, 3);
    const choices = shuffle([correctCard, ...wrongCards]).map((c) => c.definition);

    return {
      setTitle: setWithCards.title,
      setId: setWithCards.id,
      term: correctCard.term,
      correctAnswer: correctCard.definition,
      choices,
    };
  }, [sets]);

  if (!preview) return null;

  return (
    <section className="mb-8">
      <h2 className="text-lg font-bold mb-3">Câu hỏi chế độ Học</h2>
      <Card className="p-6 bg-sky-50/50 dark:bg-sky-950/20 border-sky-200 dark:border-sky-800">
        <p className="text-xs font-bold text-sky-600 dark:text-sky-400 mb-1">{preview.setTitle}</p>
        <p className="text-lg font-bold mb-4">{preview.term}</p>
        <div className="grid gap-2">
          {preview.choices.map((choice, i) => (
            <div
              key={i}
              className={`px-4 py-3 rounded-xl text-sm font-medium border transition-colors ${
                choice === preview.correctAnswer
                  ? "bg-green-50 dark:bg-green-900/20 border-green-300 dark:border-green-700 text-green-800 dark:text-green-200"
                  : "bg-white dark:bg-slate-800 border-[var(--border)] text-[var(--foreground)]"
              }`}
            >
              {choice}
            </div>
          ))}
        </div>
        <button
          onClick={() => onOpenSet(preview.setId)}
          className="mt-4 px-4 py-2 rounded-xl bg-[var(--primary)] text-white text-sm font-bold hover:opacity-90 transition-opacity"
        >
          Tiếp tục học →
        </button>
      </Card>
    </section>
  );
}
