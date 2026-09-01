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

  const generate = React.useCallback((nextSeed: number) => {
    setSeed(nextSeed);
    setState({ state: "loading" });
    const controller = new AbortController();
    quizApi.generate(token, studySetId, { mode, seed: nextSeed, limit }, controller.signal)
      .then((data) => setState({ state: "ready", data }))
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === "AbortError") return;
        setState({ state: "error", error: error instanceof Error ? error : new Error("Không thể tạo bài học") });
      });
    return () => controller.abort();
  }, [token, studySetId, mode, limit]);

  React.useEffect(() => generate(seed), [generate]);

  return { state, seed, regenerate: () => generate(randomSeed()) };
}

function randomSeed(): number {
  const values = new Uint32Array(2);
  if (typeof crypto !== "undefined" && crypto.getRandomValues) crypto.getRandomValues(values);
  return (values[0] * 2 ** 32 + values[1]) % Number.MAX_SAFE_INTEGER;
}
