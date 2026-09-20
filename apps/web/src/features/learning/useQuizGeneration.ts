import React from "react";
import { useAuth } from "../auth/AuthContext";
import { ApiError, quizApi, type LearningMode, type QuizGenerateResponse } from "../../lib/api/client";

export type QuizGenerationState =
  | { state: "loading" }
  | { state: "ready"; data: QuizGenerateResponse }
  | { state: "error"; error: ApiError | Error };

export function useQuizGeneration(studySetId: number, mode: LearningMode, limit?: number) {
  const { token } = useAuth();
  const [seed, setSeed] = React.useState(() => randomSeed());
  const [state, setState] = React.useState<QuizGenerationState>({ state: "loading" });
  // Track the limit as a ref so regenerate always uses the latest value
  const limitRef = React.useRef(limit);
  limitRef.current = limit;

  const generateWithLimit = React.useCallback((nextSeed: number, overrideLimit?: number) => {
    setSeed(nextSeed);
    setState({ state: "loading" });
    const controller = new AbortController();
    const useLimit = overrideLimit ?? limitRef.current;
    quizApi.generate(token, studySetId, { mode, seed: nextSeed, limit: useLimit }, controller.signal)
      .then((data) => setState({ state: "ready", data }))
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === "AbortError") return;
        setState({ state: "error", error: error instanceof Error ? error : new Error("Không thể tạo bài học") });
      });
    return () => controller.abort();
  }, [token, studySetId, mode]);

  // Re-trigger when limit changes (for MatchMode dynamic pair count)
  const prevLimitRef = React.useRef(limit);
  React.useEffect(() => {
    if (prevLimitRef.current !== limit) {
      prevLimitRef.current = limit;
      generateWithLimit(randomSeed(), limit);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [limit]);

  React.useEffect(() => {
    return generateWithLimit(seed);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [generateWithLimit]);

  return { state, seed, regenerate: () => generateWithLimit(randomSeed()) };
}

function randomSeed(): number {
  const values = new Uint32Array(2);
  if (typeof crypto !== "undefined" && crypto.getRandomValues) crypto.getRandomValues(values);
  return (values[0] * 2 ** 32 + values[1]) % Number.MAX_SAFE_INTEGER;
}
