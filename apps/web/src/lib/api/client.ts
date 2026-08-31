// lib/api/client.ts — Dev 5 [P2-INT-02]
// Canonical frontend API client. Feature code should call this module instead of fetch directly.

import type { AuthResponse, StudySet, Flashcard, User, DraftCard } from "../../types";

const gatewayUrl =
  import.meta.env.VITE_GATEWAY_URL?.replace(/\/$/, "") ?? "http://localhost:8080";

export type ApiErrorBody = { code?: string; message?: string; field?: string; error?: string };

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
    public readonly code?: string,
    public readonly field?: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export async function apiFetch<T>(
  path: string,
  token: string,
  init: RequestInit = {},
  params?: Record<string, string | number | undefined>,
): Promise<T> {
  const url = new URL(`${gatewayUrl}${path}`);
  if (params) {
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined && value !== "") url.searchParams.set(key, String(value));
    }
  }

  const res = await fetch(url.toString(), {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(init.headers as Record<string, string> | undefined),
    },
  });

  if (res.status === 204) return undefined as unknown as T;
  const body = (await res.json().catch(() => ({}))) as ApiErrorBody;
  if (!res.ok) {
    throw new ApiError(
      res.status,
      body.message ?? body.error ?? `Request failed ${res.status}`,
      body.code,
      body.field,
    );
  }
  return body as T;
}

export const authApi = {
  login(email: string, password: string): Promise<AuthResponse> {
    return apiFetch("/v1/auth/login", "", { method: "POST", body: JSON.stringify({ email, password }) });
  },
  register(name: string, email: string, password: string): Promise<AuthResponse> {
    return apiFetch("/v1/auth/register", "", { method: "POST", body: JSON.stringify({ name, email, password }) });
  },
  me(token: string): Promise<AuthResponse> { return apiFetch("/v1/auth/me", token); },
  logout(token: string): Promise<void> { return apiFetch("/v1/auth/logout", token, { method: "POST" }); },
  refresh(token: string): Promise<AuthResponse> { return apiFetch("/v1/auth/refresh", token, { method: "POST" }); },
  updateProfile(token: string, payload: { name?: string; image?: string }): Promise<User> {
    return apiFetch("/v1/auth/profile", token, { method: "PATCH", body: JSON.stringify(payload) });
  },
};

export type CreateStudySetPayload = { title: string; description?: string };
export type UpdateStudySetPayload = { title?: string; description?: string };

export type StudySetListParams = {
  search?: string;
  sort?: "updated" | "created" | "title";
  page?: number;
  per_page?: number;
};

export type StudySetListResult = {
  items: StudySet[];
  total: number;
  page: number;
  perPage: number;
  totalPages: number;
};

export const studySetApi = {
  list(token: string, params?: StudySetListParams): Promise<StudySetListResult> {
    return apiFetch("/v1/study-sets", token, {}, params);
  },
  get(token: string, id: number): Promise<StudySet> { return apiFetch(`/v1/study-sets/${id}`, token); },
  create(token: string, payload: CreateStudySetPayload): Promise<StudySet> {
    return apiFetch("/v1/study-sets", token, { method: "POST", body: JSON.stringify(payload) });
  },
  update(token: string, id: number, payload: UpdateStudySetPayload): Promise<StudySet> {
    return apiFetch(`/v1/study-sets/${id}`, token, { method: "PUT", body: JSON.stringify(payload) });
  },
  delete(token: string, id: number): Promise<void> {
    return apiFetch(`/v1/study-sets/${id}`, token, { method: "DELETE" });
  },
};

export type CreateFlashcardPayload = { term: string; definition: string };
export type UpdateFlashcardPayload = { term?: string; definition?: string };
export type BulkFlashcardItem = {
  id?: number;
  term: string;
  definition: string;
  position?: number;
  delete?: boolean;
};
export type BulkSaveResult = { created: Flashcard[]; updated: Flashcard[]; deleted: number[] };

export const flashcardApi = {
  create(token: string, studySetId: number, payload: CreateFlashcardPayload): Promise<Flashcard> {
    return apiFetch(`/v1/study-sets/${studySetId}/flashcards`, token, { method: "POST", body: JSON.stringify(payload) });
  },
  add(token: string, studySetId: number, payload: CreateFlashcardPayload): Promise<Flashcard> {
    return this.create(token, studySetId, payload);
  },
  bulkSave(token: string, studySetId: number, cards: BulkFlashcardItem[]): Promise<BulkSaveResult> {
    return apiFetch(`/v1/study-sets/${studySetId}/flashcards/bulk`, token, {
      method: "POST", body: JSON.stringify({ cards }),
    });
  },
  update(token: string, id: number, payload: UpdateFlashcardPayload): Promise<Flashcard> {
    return apiFetch(`/v1/flashcards/${id}`, token, { method: "PUT", body: JSON.stringify(payload) });
  },
  delete(token: string, id: number): Promise<void> {
    return apiFetch(`/v1/flashcards/${id}`, token, { method: "DELETE" });
  },
  toggleStar(token: string, id: number): Promise<Flashcard> {
    return apiFetch(`/v1/flashcards/${id}/star`, token, { method: "POST" });
  },
};

export type Folder = {
  id: number;
  userId: number;
  name: string;
  description: string;
  createdAt: string;
  updatedAt: string;
};
export type FolderDetail = Folder & { studySets: StudySet[] };

export const folderApi = {
  list(token: string): Promise<Folder[]> { return apiFetch("/v1/folders", token); },
  create(token: string, payload: { name: string; description?: string }): Promise<Folder> {
    return apiFetch("/v1/folders", token, { method: "POST", body: JSON.stringify(payload) });
  },
  get(token: string, id: number): Promise<FolderDetail> { return apiFetch(`/v1/folders/${id}`, token); },
  update(token: string, id: number, payload: { name?: string; description?: string }): Promise<Folder> {
    return apiFetch(`/v1/folders/${id}`, token, { method: "PUT", body: JSON.stringify(payload) });
  },
  delete(token: string, id: number): Promise<void> { return apiFetch(`/v1/folders/${id}`, token, { method: "DELETE" }); },
  addStudySet(token: string, folderId: number, studySetId: number): Promise<void> {
    return apiFetch(`/v1/folders/${folderId}/study-sets`, token, { method: "POST", body: JSON.stringify({ studySetId }) });
  },
  removeStudySet(token: string, folderId: number, studySetId: number): Promise<void> {
    return apiFetch(`/v1/folders/${folderId}/study-sets/${studySetId}`, token, { method: "DELETE" });
  },
};

export type LearningMode = "flashcards" | "learn" | "test" | "match";
export type LearningProgress = {
  id: number;
  userId: number;
  studySetId: number;
  mode: LearningMode;
  score: number;
  total: number;
  completedAt: string | null;
  createdAt: string;
};

// Draft only. Phase 2 learning progress remains local; no production request is made.
export const learningApi = {
  saveProgress: async (): Promise<null> => null,
  getProgress: async (): Promise<LearningProgress[]> => [],
};

export type ServiceHealth = { name: string; url: string; status: string };
export async function fetchHealth(): Promise<ServiceHealth[]> {
  const data = await apiFetch<{ services: ServiceHealth[] }>("/healthz/services", "");
  return data.services ?? [];
}

export const healthApi = {
  check: () => apiFetch<{ service: string; status: string }>("/healthz", ""),
  services: () => apiFetch<{ services: ServiceHealth[] }>("/healthz/services", ""),
};

export type { User, AuthResponse, StudySet, Flashcard, DraftCard };
