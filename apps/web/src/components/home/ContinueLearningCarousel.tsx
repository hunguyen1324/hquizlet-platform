// ContinueLearningCarousel — horizontal scroll of study sets (top 5 most recent)
import React from "react";
import type { StudySet } from "../../types";
import { Card, Badge, ProgressBar, Carousel } from "../ui";

type Props = {
  sets: StudySet[];
  onOpenSet: (id: number) => void;
};

export function ContinueLearningCarousel({ sets, onOpenSet }: Props) {
  if (sets.length === 0) return null;

  return (
    <section className="mb-8">
      <h2 className="text-lg font-bold mb-3">Học tiếp</h2>
      <Carousel>
        {sets.slice(0, 5).map((set) => (
          <Card key={set.id} className="min-w-[260px] max-w-[280px] snap-start p-4 flex flex-col gap-3 shrink-0">
            <h3 className="text-sm font-bold line-clamp-2 min-h-[2.5rem]">{set.title}</h3>
            <span className="text-xs text-[var(--muted-foreground)]">
              {set.flashcards?.length ?? set.flashcardCount ?? 0} thẻ
            </span>
            <Badge variant="sky">0%</Badge>
            <ProgressBar value={0} />
            <div className="flex gap-2 mt-auto">
              <button
                className="flex-1 text-xs font-bold px-3 py-2 rounded-lg bg-[var(--muted)] hover:bg-[var(--accent)] transition-colors"
                onClick={() => onOpenSet(set.id)}
              >
                Xem chi tiết
              </button>
              <button
                className="flex-1 text-xs font-bold px-3 py-2 rounded-lg bg-[var(--primary)] text-white hover:opacity-90 transition-opacity"
                onClick={() => onOpenSet(set.id)}
              >
                Tiếp tục →
              </button>
            </div>
          </Card>
        ))}
      </Carousel>
    </section>
  );
}
