import { apiFetch } from "./client";
import type { QuizQuestion } from "../../types";

export type QuizStart = { mode: "practice" | "exam"; instant: boolean; layout: "default" | "toeic"; part: number; durationSeconds: number };
export type ProtectedQuizView = {
 id: string; mode: "practice" | "exam"; instant: boolean; layout: "default" | "toeic";
 serverTime: string; deadline?: string; submitted: boolean; currentIndex: number;
 manifest: { number: number; part: number; group: number }[];
 page: { index: number; question: QuizQuestion; hasAudio: boolean; revealed: boolean }[];
 answers: Record<number, string>; results: Record<number, boolean>; score?: number; timeUsed: number;
};
const path = (setId: number) => `/v1/study-sets/${setId}/play`;
const gateway = import.meta.env.VITE_GATEWAY_URL?.replace(/\/$/, "") ?? "http://localhost:8080";
export const protectedQuizApi = {
 summary: (token: string, setId: number) => apiFetch<{ total: number; parts: Record<number, number>; sessions: string[] }>(path(setId), token),
 start: (token: string, setId: number, input: QuizStart) => apiFetch<ProtectedQuizView>(path(setId), token, { method: "POST", body: JSON.stringify(input) }),
 page: (token: string, setId: number, id: string, index = -1) => apiFetch<ProtectedQuizView>(`${path(setId)}/${id}`, token, {}, index < 0 ? undefined : { index }),
 answer: (token: string, setId: number, id: string, index: number, answer: string) => apiFetch<ProtectedQuizView>(`${path(setId)}/${id}/answer`, token, { method: "POST", body: JSON.stringify({ index, answer }) }),
 submit: (token: string, setId: number, id: string) => apiFetch<ProtectedQuizView>(`${path(setId)}/${id}/submit`, token, { method: "POST" }),
 audio: async (token: string, setId: number, id: string, index: number, signal: AbortSignal) => {
   const response = await fetch(`${gateway}${path(setId)}/${id}/audio-ticket?index=${index}`, { headers: { Authorization: `Bearer ${token}` }, cache: "no-store", signal });
   if (!response.ok) {
     const body = await response.json().catch(() => ({})) as { message?: string };
     throw new Error(body.message ?? "Không tải được âm thanh.");
   }
   const body = await response.json() as { path: string };
   return `${gateway}${body.path}`;
 },
 uploadAudio: (token: string, setId: number, file: File) => apiFetch<{ audioUrl: string }>(`${path(setId)}/audio`, token, { method: "POST", headers: { "Content-Type": "application/octet-stream" }, body: file }),
};
