// lib/api/client.ts — Dev 3 [P2-WEB-02 / P2-INT-02]
// Typed API client. Tất cả gọi backend đi qua đây, không gọi fetch trực tiếp trong component.

import type {
  AuthResponse,
  StudySet,
  Flashcard,
  User,
} from "../../types";

const gatewayUrl =
  import.meta.env.VITE_GATEWAY_URL?.replace(/\/$/, "") ?? "http://localhost:8080";

// ── Core fetch helper ──────────────────────────────────────────────────────

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export async function apiFetch<T>(
  path: string,
  token: string,
  init: RequestInit = {}
): Promise<T> {
  const res = await fetch(`${gatewayUrl}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(init.headers as Record<string, string> | undefined),
    },
  });

  // 204 No Content – không có body
  if (res.status === 204) return undefined as unknown as T;

  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new ApiError(
      res.status,
      (body as { error?: string }).error ?? `Request failed ${res.status}`
    );
  }
  return body as T;
}

// ── Auth API ──────────────────────────────────────────────────────────────

export const authApi = {
  login(email: string, password: string): Promise<AuthResponse> {
    return apiFetch("/v1/auth/login", "", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    });
  },

  register(name: string, email: string, password: string): Promise<AuthResponse> {
    return apiFetch("/v1/auth/register", "", {
      method: "POST",
      body: JSON.stringify({ name, email, password }),
    });
  },

  me(token: string): Promise<AuthResponse> {
    return apiFetch("/v1/auth/me", token);
  },

  logout(token: string): Promise<void> {
    return apiFetch("/v1/auth/logout", token, { method: "POST" });
  },
};

// ── Study Set API ─────────────────────────────────────────────────────────

export type CreateStudySetPayload = {
  title: string;
  description?: string;
};

export type UpdateStudySetPayload = {
  title?: string;
  description?: string;
};

export const studySetApi = {
  list(token: string): Promise<StudySet[]> {
    return apiFetch("/v1/study-sets", token);
  },

  get(token: string, id: number): Promise<StudySet> {
    return apiFetch(`/v1/study-sets/${id}`, token);
  },

  create(token: string, payload: CreateStudySetPayload): Promise<StudySet> {
    return apiFetch("/v1/study-sets", token, {
      method: "POST",
      body: JSON.stringify(payload),
    });
  },

  update(token: string, id: number, payload: UpdateStudySetPayload): Promise<StudySet> {
    return apiFetch(`/v1/study-sets/${id}`, token, {
      method: "PUT",
      body: JSON.stringify(payload),
    });
  },

  delete(token: string, id: number): Promise<void> {
    return apiFetch(`/v1/study-sets/${id}`, token, { method: "DELETE" });
  },
};

// ── Flashcard API ─────────────────────────────────────────────────────────

export type CreateFlashcardPayload = {
  term: string;
  definition: string;
};

export type UpdateFlashcardPayload = {
  term?: string;
  definition?: string;
};

export const flashcardApi = {
  create(token: string, studySetId: number, payload: CreateFlashcardPayload): Promise<Flashcard> {
    return apiFetch(`/v1/study-sets/${studySetId}/flashcards`, token, {
      method: "POST",
      body: JSON.stringify(payload),
    });
  },

  update(token: string, id: number, payload: UpdateFlashcardPayload): Promise<Flashcard> {
    return apiFetch(`/v1/flashcards/${id}`, token, {
      method: "PUT",
      body: JSON.stringify(payload),
    });
  },

  delete(token: string, id: number): Promise<void> {
    return apiFetch(`/v1/flashcards/${id}`, token, { method: "DELETE" });
  },

  toggleStar(token: string, id: number): Promise<Flashcard> {
    return apiFetch(`/v1/flashcards/${id}/star`, token, { method: "POST" });
  },
};

// ── Health ────────────────────────────────────────────────────────────────

export type ServiceHealth = {
  name: string;
  url: string;
  status: string;
};

export async function fetchHealth(): Promise<ServiceHealth[]> {
  const data = await apiFetch<{ services: ServiceHealth[] }>(
    "/healthz/services",
    ""
  );
  return data.services ?? [];
}
