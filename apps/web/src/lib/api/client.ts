// apps/web/src/lib/api/client.ts
// Dev 5 – P2-INT-02: Typed API client wrapper
// Tất cả feature gọi API qua đây, không gọi fetch trực tiếp.

import type {
  User,
  AuthResponse,
  StudySet,
  Flashcard,
  DraftCard,
} from "../../types";

// ── Re-export thêm types Phase 2 ──────────────────────────────────────────
export type { User, AuthResponse, StudySet, Flashcard, DraftCard };

export type Folder = {
  id: number;
  userId: number;
  title: string;
  description: string;
  createdAt: string;
  updatedAt: string;
};

export type FolderDetail = Folder & {
  studySets: StudySet[];
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

export type StudySetListParams = {
  q?: string;
  sort?: "updatedAt" | "createdAt" | "title";
  order?: "asc" | "desc";
};

export type BulkFlashcardItem = {
  id?: number;
  term: string;
  definition: string;
};

// ── Client factory ────────────────────────────────────────────────────────

const GATEWAY_URL =
  (import.meta.env?.VITE_GATEWAY_URL as string | undefined)?.replace(/\/$/, "") ??
  "http://localhost:8080";

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

async function request<T>(
  path: string,
  token: string,
  init: RequestInit = {},
  params?: Record<string, string>,
): Promise<T> {
  let url = `${GATEWAY_URL}${path}`;
  if (params) {
    const qs = new URLSearchParams(
      Object.fromEntries(Object.entries(params).filter(([, v]) => v !== undefined && v !== "")),
    ).toString();
    if (qs) url += `?${qs}`;
  }

  const res = await fetch(url, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...init.headers,
    },
  });

  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new ApiError(
      res.status,
      (body as { error?: string }).error ?? `Request failed ${res.status}`,
    );
  }
  return body as T;
}

// ── Auth API ─────────────────────────────────────────────────────────────

export const authApi = {
  register: (name: string, email: string, password: string) =>
    request<AuthResponse>("/v1/auth/register", "", {
      method: "POST",
      body: JSON.stringify({ name, email, password }),
    }),

  login: (email: string, password: string) =>
    request<AuthResponse>("/v1/auth/login", "", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    }),

  logout: (token: string) =>
    request<void>("/v1/auth/logout", token, { method: "POST" }),

  me: (token: string) =>
    request<AuthResponse>("/v1/auth/me", token),

  refresh: (token: string) =>
    request<AuthResponse>("/v1/auth/refresh", token, { method: "POST" }),

  updateProfile: (token: string, data: { name?: string; image?: string }) =>
    request<User>("/v1/auth/profile", token, {
      method: "PUT",
      body: JSON.stringify(data),
    }),
};

// ── Study Set API ─────────────────────────────────────────────────────────

export const studySetApi = {
  list: (token: string, params?: StudySetListParams) =>
    request<StudySet[]>("/v1/study-sets", token, {}, params as Record<string, string>),

  create: (token: string, data: { title: string; description?: string }) =>
    request<StudySet>("/v1/study-sets", token, {
      method: "POST",
      body: JSON.stringify(data),
    }),

  get: (token: string, id: number) =>
    request<StudySet & { flashcards: Flashcard[] }>(`/v1/study-sets/${id}`, token),

  update: (token: string, id: number, data: { title?: string; description?: string }) =>
    request<StudySet>(`/v1/study-sets/${id}`, token, {
      method: "PUT",
      body: JSON.stringify(data),
    }),

  delete: (token: string, id: number) =>
    request<void>(`/v1/study-sets/${id}`, token, { method: "DELETE" }),
};

// ── Flashcard API ─────────────────────────────────────────────────────────

export const flashcardApi = {
  add: (token: string, studySetId: number, data: { term: string; definition: string }) =>
    request<Flashcard>(`/v1/study-sets/${studySetId}/flashcards`, token, {
      method: "POST",
      body: JSON.stringify(data),
    }),

  /** P2-STUDY-02: Bulk sync toàn bộ flashcards cho một study set */
  bulkSave: (token: string, studySetId: number, flashcards: BulkFlashcardItem[]) =>
    request<{ flashcards: Flashcard[] }>(
      `/v1/study-sets/${studySetId}/flashcards/bulk`,
      token,
      { method: "PUT", body: JSON.stringify({ flashcards }) },
    ),

  update: (token: string, id: number, data: { term?: string; definition?: string }) =>
    request<Flashcard>(`/v1/flashcards/${id}`, token, {
      method: "PUT",
      body: JSON.stringify(data),
    }),

  delete: (token: string, id: number) =>
    request<void>(`/v1/flashcards/${id}`, token, { method: "DELETE" }),

  toggleStar: (token: string, id: number) =>
    request<{ id: number; starred: boolean }>(`/v1/flashcards/${id}/star`, token, {
      method: "POST",
    }),
};

// ── Folder API ────────────────────────────────────────────────────────────
// P2-STUDY-04 — Folder core

export const folderApi = {
  list: (token: string) =>
    request<Folder[]>("/v1/folders", token),

  create: (token: string, data: { title: string; description?: string }) =>
    request<Folder>("/v1/folders", token, {
      method: "POST",
      body: JSON.stringify(data),
    }),

  get: (token: string, id: number) =>
    request<FolderDetail>(`/v1/folders/${id}`, token),

  update: (token: string, id: number, data: { title?: string; description?: string }) =>
    request<Folder>(`/v1/folders/${id}`, token, {
      method: "PUT",
      body: JSON.stringify(data),
    }),

  delete: (token: string, id: number) =>
    request<void>(`/v1/folders/${id}`, token, { method: "DELETE" }),

  addStudySet: (token: string, folderId: number, studySetId: number) =>
    request<void>(`/v1/folders/${folderId}/study-sets`, token, {
      method: "POST",
      body: JSON.stringify({ studySetId }),
    }),

  removeStudySet: (token: string, folderId: number, studySetId: number) =>
    request<void>(`/v1/folders/${folderId}/study-sets/${studySetId}`, token, {
      method: "DELETE",
    }),
};

// ── Learning Progress API ─────────────────────────────────────────────────
// P2-LEARN-05 — DRAFT. Backend chưa implement. Gọi silent (try-catch).

export const learningApi = {
  /**
   * Lưu kết quả học tập. DRAFT – backend Phase 3 implement.
   * Gọi bằng try-catch silent để không block UI nếu 404/501.
   */
  saveProgress: async (
    token: string,
    data: { studySetId: number; mode: LearningMode; score: number; total: number },
  ): Promise<LearningProgress | null> => {
    try {
      return await request<LearningProgress>("/v1/learning/progress", token, {
        method: "POST",
        body: JSON.stringify(data),
      });
    } catch {
      // Backend chưa sẵn trong Phase 2 — silent fail
      return null;
    }
  },

  getProgress: async (token: string, studySetId: number): Promise<LearningProgress[]> => {
    try {
      return await request<LearningProgress[]>(`/v1/learning/progress/${studySetId}`, token);
    } catch {
      return [];
    }
  },
};

// ── Health API ────────────────────────────────────────────────────────────

export type ServiceHealth = {
  name: string;
  url: string;
  status: "ok" | "offline";
};

export const healthApi = {
  check: () =>
    fetch(`${GATEWAY_URL}/healthz`).then((r) => r.json() as Promise<{ service: string; status: string }>),

  services: () =>
    fetch(`${GATEWAY_URL}/healthz/services`).then((r) =>
      r.json() as Promise<{ services: ServiceHealth[] }>,
    ),
};
