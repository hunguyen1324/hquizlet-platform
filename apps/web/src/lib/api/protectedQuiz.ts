import { apiFetch } from "./client";
import { gatewayUrl as gateway } from "./gateway";
import type { QuizQuestion } from "../../types";

export type QuizStart = { mode: "practice" | "exam"; instant: boolean; layout: "default" | "toeic"; part: number; durationSeconds: number };
export type ProtectedQuizView = {
  id: string; mode: "practice" | "exam"; instant: boolean; layout: "default" | "toeic";
  serverTime: string; deadline?: string; submitted: boolean; currentIndex: number;
  manifest: { number: number; part: number; group: number }[];
  page: { index: number; question: QuizQuestion; hasAudio: boolean; revealed: boolean }[];
  answers: Record<number, string>; results: Record<number, boolean>; score?: number; timeUsed: number;
};

export type QuizSessionMeta = {
  id: string;
  studySetId: number;
  mode: "practice" | "exam";
  instant: boolean;
  layout: "default" | "toeic";
  startedAt: string;
  expiresAt: string;
  activeUntil: string;
  deadline?: string;
  submittedAt?: string;
  updatedAt: string;
  answered: number;
  total: number;
};

export type QuizSessionListResponse = {
  items: QuizSessionMeta[];
  total: number;
  page: number;
  perPage: number;
};

const path = (setId: number) => `/v1/study-sets/${setId}/play`;

export const protectedQuizApi = {
  summary: (token: string, setId: number) =>
    apiFetch<{ total: number; parts: Record<number, number>; sessions: string[]; activeCount: number; limit: number; studySetId: number }>(path(setId), token),
  start: (token: string, setId: number, input: QuizStart) =>
    apiFetch<ProtectedQuizView>(path(setId), token, { method: "POST", body: JSON.stringify(input) }),
  page: (token: string, setId: number, id: string, index = -1) =>
    apiFetch<ProtectedQuizView>(`${path(setId)}/${id}`, token, {}, index < 0 ? undefined : { index }),
  answer: (token: string, setId: number, id: string, index: number, answer: string) =>
    apiFetch<ProtectedQuizView>(`${path(setId)}/${id}/answer`, token, { method: "POST", body: JSON.stringify({ index, answer }) }),
  submit: (token: string, setId: number, id: string) =>
    apiFetch<ProtectedQuizView>(`${path(setId)}/${id}/submit`, token, { method: "POST" }),
  audio: async (token: string, setId: number, id: string, index: number, signal: AbortSignal) => {
    const response = await fetch(`${gateway}${path(setId)}/${id}/audio-ticket?index=${index}`, { headers: { Authorization: `Bearer ${token}` }, cache: "no-store", signal });
    if (!response.ok) {
      const body = await response.json().catch(() => ({})) as { message?: string };
      throw new Error(body.message ?? "Không tải được âm thanh.");
    }
    const body = await response.json() as { path: string };
    return `${gateway}${body.path}`;
  },
  uploadAudio: (token: string, setId: number, file: File) =>
    apiFetch<{ audioUrl: string }>(`${path(setId)}/audio`, token, { method: "POST", headers: { "Content-Type": "application/octet-stream" }, body: file }),

  /** List all quiz sessions for the current user, optionally filtered to one quiz. */
  listSessions: (token: string, params?: { studySetId?: number; page?: number; perPage?: number }) => {
    const q = new URLSearchParams();
    if (params?.studySetId) q.set("study_set_id", String(params.studySetId));
    if (params?.page) q.set("page", String(params.page));
    if (params?.perPage) q.set("per_page", String(params.perPage));
    const qs = q.toString();
    return apiFetch<QuizSessionListResponse>(`/v1/quiz-sessions${qs ? "?" + qs : ""}`, token);
  },

  /** Soft-delete a quiz session owned by the current user. */
  deleteSession: (token: string, id: string) =>
    apiFetch<{ ok: boolean }>(`/v1/quiz-sessions/${id}`, token, { method: "DELETE" }),
};
