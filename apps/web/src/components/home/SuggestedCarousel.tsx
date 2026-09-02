// SuggestedCarousel — smaller cards of study sets not yet started
import React from "react";
import type { StudySet } from "../../types";
import { Card, Badge, Carousel } from "../ui";

type Props = {
  sets: StudySet[];
  onOpenSet: (id: number) => void;
};

export function SuggestedCarousel({ sets, onOpenSet }: Props) {
  if (sets.length === 0) return null;

  return (
    <section className="mb-8">
      <h2 className="text-lg font-bold mb-3">Gợi ý cho phiên học kế tiếp</h2>
      <Carousel>
        {sets.map((set) => (
          <Card key={set.id} className="min-w-[220px] max-w-[240px] snap-start p-4 flex flex-col gap-2 shrink-0 cursor-pointer hover:border-[var(--primary)] transition-colors" onClick={() => onOpenSet(set.id)}>
            <h3 className="text-sm font-bold line-clamp-2 min-h-[2.5rem]">{set.title}</h3>
            <span className="text-xs text-[var(--muted-foreground)]">
              {set.flashcards?.length ?? set.flashcardCount ?? 0} thẻ
            </span>
            <Badge variant="sky" className="mt-auto w-fit">
              <svg className="w-3 h-3 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
              </svg>
              Mới
            </Badge>
          </Card>
        ))}
      </Carousel>
    </section>
  );
}
