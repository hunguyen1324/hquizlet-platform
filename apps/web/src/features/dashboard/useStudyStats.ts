// useStudyStats — Phase 2 (UI-04)
import { useQuery } from "@tanstack/react-query";
import { useAuth } from "../auth/AuthContext";
import { statsApi } from "../../lib/api";
import type { StudyStats } from "../../types";

export function useStudyStats(days: number) {
  const { token } = useAuth();
  return useQuery<StudyStats>({
    queryKey: ["study-stats", days],
    queryFn: () => statsApi.studyStats(token, days),
    enabled: !!token,
    staleTime: 60_000,
  });
}
