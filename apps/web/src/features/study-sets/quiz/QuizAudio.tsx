import React from "react";
import type { ProtectedQuizView } from "../../../lib/api/protectedQuiz";

export type AudioState = { url?: string; error: string; loading: boolean; retry: () => void };

/** Resolves the playback URL of the audio belonging to the page on screen. The browser streams it. */
export function useQuizAudioUrl(
  view: ProtectedQuizView | null,
  getAudioUrl: (sessionId: string, index: number, signal?: AbortSignal) => Promise<string>,
  forgetAudio: (sessionId: string, index: number) => void,
): AudioState {
  const audioItem = view?.page.find((item) => item.hasAudio);
  const sessionId = view?.id;
  const index = audioItem?.index;
  const key = sessionId !== undefined && index !== undefined ? `${sessionId}:${index}` : "";
  const [state, setState] = React.useState<{ key: string; url?: string; error: string }>({ key: "", error: "" });
  const [attempt, setAttempt] = React.useState(0);

  React.useEffect(() => {
    if (!key || sessionId === undefined || index === undefined) return;
    const controller = new AbortController();
    getAudioUrl(sessionId, index, controller.signal)
      .then((url) => { if (!controller.signal.aborted) setState({ key, url, error: "" }); })
      .catch((e: Error) => { if (!controller.signal.aborted) setState({ key, error: e.message || "Không tải được âm thanh." }); });
    return () => controller.abort();
  }, [key, attempt, getAudioUrl, sessionId, index]);

  const retry = React.useCallback(() => {
    if (sessionId !== undefined && index !== undefined) forgetAudio(sessionId, index);
    setState({ key: "", error: "" });
    setAttempt((n) => n + 1);
  }, [forgetAudio, index, sessionId]);

  if (!key) return { error: "", loading: false, retry };
  const ready = state.key === key;
  return { url: ready ? state.url : undefined, error: ready ? state.error : "", loading: !ready || (!state.url && !state.error), retry };
}
