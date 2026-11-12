import { apiFetch } from "../../lib/api/client";
import type { SRStats } from "../../components/progress";
export type Rating = "again" | "hard" | "good" | "easy";
export const repetitionKey = (userId: string, id: number) => ["spaced-repetition", userId, id] as const;
export const repetitionApi = {
 stats: (token: string, id: number) => apiFetch<SRStats & { due: number }>(`/v1/study-sets/${id}/spaced-repetition`, token),
 review: (token: string, id: number, flashcardId: number, rating: Rating) => apiFetch<void>(`/v1/study-sets/${id}/spaced-repetition`, token, { method: "POST", body: JSON.stringify({ flashcardId, rating }) }),
 reset: (token: string, id: number) => apiFetch<void>(`/v1/study-sets/${id}/spaced-repetition`, token, { method: "DELETE" }),
 related: (token: string, id: number, term: string) => apiFetch<{ characters: string[] }>(`/v1/study-sets/${id}/related-kanji`, token, {}, { term }),
};
