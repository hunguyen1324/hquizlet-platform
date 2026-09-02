// RecentList — vertical list of recently accessed study sets
import React from "react";
import type { StudySet } from "../../types";

type Props = {
  sets: StudySet[];
  onOpenSet: (id: number) => void;
};

export function RecentList({ sets, onOpenSet }: Props) {
  if (sets.length === 0) return null;

  return (
    <section className="mb-8">
      <h2 className="text-lg font-bold mb-3">Gần đây</h2>
      <div className="bg-[var(--card)] border border-[var(--border)] rounded-2xl divide-y divide-[var(--border)] overflow-hidden">
        {sets.slice(0, 8).map((set) => (
          <button
            key={set.id}
            onClick={() => onOpenSet(set.id)}
            className="w-full flex items-center gap-3 px-4 py-3 hover:bg-[var(--accent)] transition-colors text-left"
          >
            <span className="text-xl shrink-0">📖</span>
            <div className="min-w-0 flex-1">
              <p className="text-sm font-bold truncate">{set.title}</p>
              <p className="text-xs text-[var(--muted-foreground)] truncate">
                {set.flashcards?.length ?? set.flashcardCount ?? 0} thẻ
                {set.description ? ` · ${set.description}` : ""}
              </p>
            </div>
            <svg className="w-4 h-4 text-[var(--muted-foreground)] shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
            </svg>
          </button>
        ))}
      </div>
    </section>
  );
}
