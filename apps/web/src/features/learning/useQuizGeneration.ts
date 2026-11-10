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
  const limitRef = React.useRef(limit);
  limitRef.current = limit;
  const activeRequest = React.useRef<AbortController>();

  const generate = React.useCallback(() => {
    activeRequest.current?.abort();
    const controller = new AbortController();
    activeRequest.current = controller;
    const nextSeed = randomSeed();
    setSeed(nextSeed);
    setState({ state: "loading" });
    quizApi.generate(token, studySetId, { mode, seed: nextSeed, limit: limitRef.current }, controller.signal)
      .then(data => { if (!controller.signal.aborted) setState({ state: "ready", data }); })
      .catch((error: unknown) => {
        if (!controller.signal.aborted) setState({ state: "error", error: error instanceof Error ? error : new Error("Không thể tạo bài học") });
      });
  }, [token, studySetId, mode]);

  React.useEffect(() => {
    generate();
    return () => activeRequest.current?.abort();
  }, [generate, limit]);

  return { state, seed, regenerate: generate };
}

function randomSeed(): number {
  const values = new Uint32Array(2);
  if (typeof crypto !== "undefined" && crypto.getRandomValues) crypto.getRandomValues(values);
  return (values[0] * 2 ** 32 + values[1]) % Number.MAX_SAFE_INTEGER;
}
