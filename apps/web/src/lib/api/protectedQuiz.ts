import { apiFetch } from "./client";
import { gatewayUrl as gateway } from "./gateway";
import type { QuizQuestion } from "../../types";

export type QuizStart = { mode: "practice" | "exam"; instant: boolean; layout: "default" | "toeic"; part: number; durationSeconds: number };
/**
 * One response of the protected player.
 *
 * `manifest` only arrives with start and with an explicit resume; the client
 * keeps the copy it already has. `answers`/`results` are page-scoped while the
 * session is open and complete once it is submitted, so the client merges every
 * response into its own state instead of replacing it.
 */
export type ProtectedQuizView = {
  id: string; mode: "practice" | "exam"; instant: boolean; layout: "default" | "toeic";
  serverTime: string; deadline?: string; submitted: boolean; currentIndex: number;
  total: number; answered: number;
  manifest?: { number: number; part: number; group: number }[];
  page: { index: number; question: QuizQuestion; hasAudio: boolean; revealed: boolean }[];
  answers: Record<number, string>; results: Record<number, boolean>; score?: number; timeUsed: number;
};

/**
 * A page/answer response only carries the served page: the manifest stays from
 * start and answered pages accumulate instead of being replaced by the latest
 * one. A submitted session answers with complete maps, so those win outright.
 */
export function mergeQuizView(prev: ProtectedQuizView | null, next: ProtectedQuizView): ProtectedQuizView {
  if (!prev || prev.id !== next.id) return next;
  return {
    ...prev,
    ...next,
    total: next.total || prev.total,
    manifest: next.manifest?.length ? next.manifest : prev.manifest,
    answers: { ...prev.answers, ...next.answers },
    results: { ...prev.results, ...next.results },
  };
}

/** Read-only prefetch payload: page content only, no answers and no audio rights. */
export type ProtectedQuizPeek = {
  id: string;
  serverTime: string;
  currentIndex: number;
  page: ProtectedQuizView["page"];
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
  /** `manifest: true` is only needed when resuming in a tab that has no manifest yet. */
  page: (token: string, setId: number, id: string, index = -1, options?: { manifest?: boolean }) =>
    apiFetch<ProtectedQuizView>(`${path(setId)}/${id}`, token, {}, {
      ...(index < 0 ? {} : { index }),
      ...(options?.manifest ? { manifest: "1" } : {}),
    }),
  answer: (token: string, setId: number, id: string, index: number, answer: string) =>
    apiFetch<ProtectedQuizView>(`${path(setId)}/${id}/answer`, token, { method: "POST", body: JSON.stringify({ index, answer }) }),
  submit: (token: string, setId: number, id: string) =>
    apiFetch<ProtectedQuizView>(`${path(setId)}/${id}/submit`, token, { method: "POST" }),
  /** Warm the client cache for a page without marking it seen or granting audio. */
  peek: (token: string, setId: number, id: string, index: number) =>
    apiFetch<ProtectedQuizPeek>(`${path(setId)}/${id}/peek`, token, {}, { index }),
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
