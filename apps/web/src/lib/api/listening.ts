import { apiFetch } from "./client";
import { gatewayUrl } from "./gateway";
export type TranscriptionJob = { id: string; status: "queued" | "running" | "completed" | "failed"; error?: string; srtText?: string };
const path = (setId: number) => `/v1/study-sets/${setId}/listening`;
export const listeningApi = {
  start: (token: string, setId: number, audioUrl: string, language: string) => apiFetch<TranscriptionJob>(`${path(setId)}/transcriptions`, token, { method: "POST", body: JSON.stringify({ audioUrl, language }) }),
  job: (token: string, setId: number, id: string, signal?: AbortSignal) => apiFetch<{ job: TranscriptionJob }>(`${path(setId)}/transcriptions/${id}`, token, { signal }),
  latest: (token: string, setId: number, audioUrl: string, signal?: AbortSignal) => apiFetch<{ job: TranscriptionJob | null }>(`${path(setId)}/transcriptions`, token, { signal }, { audioUrl }),
  preview: async (token: string, setId: number, source: string, signal: AbortSignal) => {
    const response = await fetch(`${gatewayUrl}${path(setId)}/audio?source=${encodeURIComponent(source)}`, { headers: { Authorization: `Bearer ${token}` }, signal, cache: "no-store" });
    if (!response.ok) throw new Error("Không tải được bản nghe thử.");
    return response.blob();
  },
};
