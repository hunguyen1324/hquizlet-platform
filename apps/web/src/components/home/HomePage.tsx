// HomePage — assembler: fetches study sets once, renders all 5 sections
import React from "react";
import { useQuery } from "@tanstack/react-query";
import type { StudySet } from "../../types";
import { useAuth } from "../../features/auth/AuthContext";
import { studySetApi } from "../../lib/api";
import { LoadingSkeleton } from "../ui";
import { ContinueLearningCarousel } from "./ContinueLearningCarousel";
import { RecentList } from "./RecentList";
import { LearnModePreview } from "./LearnModePreview";
import { SuggestedCarousel } from "./SuggestedCarousel";
import { PlaySection } from "./PlaySection";

type Props = {
  onOpenSet: (id: number) => void;
  onNavigate: (view: string) => void;
};

export function HomePage({ onOpenSet, onNavigate }: Props) {
  const { token } = useAuth();

  const { data: result, isLoading: loading, error } = useQuery({
    queryKey: ["studySets", "updated", 50],
    queryFn: () => studySetApi.list(token, { sort: "updated", per_page: 50 }),
  });

  const sets = result?.items ?? [];

  if (loading) {
    return (
      <div className="max-w-4xl mx-auto py-8">
        <LoadingSkeleton rows={5} />
      </div>
    );
  }

  if (error) {
    return (
      <div className="max-w-4xl mx-auto py-8 text-center">
        <p className="text-[var(--muted-foreground)] mb-4">
          {error instanceof Error ? error.message : "Không tải được dữ liệu."}
        </p>
        <button className="primary-button" onClick={() => window.location.reload()}>Thử lại</button>
      </div>
    );
  }

  return (
    <div className="max-w-4xl mx-auto py-4 md:py-8">
      <ContinueLearningCarousel sets={sets} onOpenSet={onOpenSet} />
      <RecentList sets={sets} onOpenSet={onOpenSet} />
      <LearnModePreview sets={sets} onOpenSet={onOpenSet} />
      <SuggestedCarousel sets={sets} onOpenSet={onOpenSet} />
      <PlaySection onNavigate={onNavigate} />
    </div>
  );
}
