// lib/api/client.ts — Dev 5
// Canonical frontend API client. Feature code should call this module instead of fetch directly.
import type {
  ActivityFeedResponse,
  AdminOrderList,
  AdminTxList,
  AuthResponse,
  ClassDetail,
  ClassMember,
  ClassStudySet,
  ClassSummary,
  DepositOrderStatus,
  DraftCard,
  Flashcard,
  JoinClassResponse,
  PaymentOrder,
  PurchaseResult,
  StudySet,
  StudySetAccessInfo,
  User,
  WalletBalance,
  WalletTransactionList,
} from "../../types";

const gatewayUrl = import.meta.env.VITE_GATEWAY_URL?.replace(/\/$/, "") ?? "http://localhost:8080";
export type ApiErrorBody = { code?: string; message?: string; field?: string; error?: string; requestId?: string; details?: Record<string, unknown> };
export class ApiError extends Error {
  constructor(public readonly status: number, message: string, public readonly code?: string, public readonly field?: string, public readonly requestId?: string) { super(message); this.name = "ApiError"; }
}
export async function apiFetch<T>(path: string, token: string, init: RequestInit = {}, params?: Record<string, string | number | undefined>): Promise<T> {
  const url = new URL(`${gatewayUrl}${path}`);
  if (params) for (const [key, value] of Object.entries(params)) if (value !== undefined && value !== "") url.searchParams.set(key, String(value));
  const isFormData = init.body instanceof FormData;
  const res = await fetch(url.toString(), {
    ...init,
    headers: {
      ...(!isFormData ? { "Content-Type": "application/json" } : {}),
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(init.headers as Record<string, string> | undefined),
    },
  });
  if (res.status === 204) return undefined as unknown as T;
  const body = (await res.json().catch(() => ({}))) as ApiErrorBody;
  if (!res.ok) throw new ApiError(res.status, body.message ?? body.error ?? `Request failed ${res.status}`, body.code, body.field, body.requestId);
  return body as T;
}

export const authApi = {
  login: (email: string, password: string): Promise<AuthResponse> => apiFetch("/v1/auth/login", "", { method: "POST", body: JSON.stringify({ email, password }) }),
  register: (name: string, email: string, password: string): Promise<AuthResponse> => apiFetch("/v1/auth/register", "", { method: "POST", body: JSON.stringify({ name, email, password }) }),
  me: (token: string): Promise<AuthResponse> => apiFetch("/v1/auth/me", token),
  logout: (token: string): Promise<void> => apiFetch("/v1/auth/logout", token, { method: "POST" }),
  refresh: (token: string): Promise<AuthResponse> => apiFetch("/v1/auth/refresh", token, { method: "POST" }),
  updateProfile: (token: string, payload: { name?: string; image?: string }): Promise<User> => apiFetch("/v1/auth/profile", token, { method: "PATCH", body: JSON.stringify(payload) }),
};

export type CreateStudySetPayload = {
  title: string;
  description?: string;
  contentType?: "flashcard" | "quiz" | "grammar";
  termLanguage?: string;
  definitionLanguage?: string;
  visibility?: "public" | "private" | string;
};
export type UpdateStudySetPayload = Partial<CreateStudySetPayload>;
export type StudySetListParams = { search?: string; sort?: "updated" | "created" | "title"; page?: number; per_page?: number };
export type StudySetListResult = { items: StudySet[]; total: number; page: number; perPage: number; totalPages: number };
export const studySetApi = {
  list: (token: string, params?: StudySetListParams, signal?: AbortSignal): Promise<StudySetListResult> => apiFetch("/v1/study-sets", token, { signal }, params),
  get: (token: string, id: number): Promise<StudySet> => apiFetch(`/v1/study-sets/${id}`, token),
  create: (token: string, payload: CreateStudySetPayload): Promise<StudySet> => apiFetch("/v1/study-sets", token, { method: "POST", body: JSON.stringify(payload) }),
  update: (token: string, id: number, payload: UpdateStudySetPayload): Promise<StudySet> => apiFetch(`/v1/study-sets/${id}`, token, { method: "PUT", body: JSON.stringify(payload) }),
  delete: (token: string, id: number): Promise<void> => apiFetch(`/v1/study-sets/${id}`, token, { method: "DELETE" }),
};

export type CreateFlashcardPayload = { term: string; definition: string; imageUrl?: string | null };
export type UpdateFlashcardPayload = { term?: string; definition?: string; imageUrl?: string | null };
export type BulkFlashcardItem = {
  id?: number;
  term: string;
  definition: string;
  exampleSentence?: string;
  hintExplanation?: string;
  synonyms?: string;
  position?: number;
  delete?: boolean;
  imageUrl?: string | null;
};
export type BulkSaveResult = { created: Flashcard[]; updated: Flashcard[]; deleted: number[] };
export const flashcardApi = {
  create: (token: string, studySetId: number, payload: CreateFlashcardPayload): Promise<Flashcard> => apiFetch(`/v1/study-sets/${studySetId}/flashcards`, token, { method: "POST", body: JSON.stringify(payload) }),
  add: (token: string, studySetId: number, payload: CreateFlashcardPayload): Promise<Flashcard> => apiFetch(`/v1/study-sets/${studySetId}/flashcards`, token, { method: "POST", body: JSON.stringify(payload) }),
  bulkSave: (token: string, studySetId: number, cards: BulkFlashcardItem[]): Promise<BulkSaveResult> => apiFetch(`/v1/study-sets/${studySetId}/flashcards/bulk`, token, { method: "POST", body: JSON.stringify({ cards }) }),
  update: (token: string, id: number, payload: UpdateFlashcardPayload): Promise<Flashcard> => apiFetch(`/v1/flashcards/${id}`, token, { method: "PUT", body: JSON.stringify(payload) }),
  delete: (token: string, id: number): Promise<void> => apiFetch(`/v1/flashcards/${id}`, token, { method: "DELETE" }),
  toggleStar: (token: string, id: number): Promise<Flashcard> => apiFetch(`/v1/flashcards/${id}/star`, token, { method: "POST" }),
};

export type FolderSummary = { id: number; title: string; description: string; studySetCount: number; createdAt: string; updatedAt: string };
export type FolderDetail = FolderSummary & { studySets: StudySet[] };
export type CreateFolderInput = { title: string; description?: string };
export type UpdateFolderInput = CreateFolderInput;
export const folderApi = {
  listFolders: (token: string): Promise<FolderSummary[]> => apiFetch("/v1/folders", token),
  createFolder: (token: string, payload: CreateFolderInput): Promise<FolderSummary> => apiFetch("/v1/folders", token, { method: "POST", body: JSON.stringify(payload) }),
  getFolder: (token: string, id: number): Promise<FolderDetail> => apiFetch(`/v1/folders/${id}`, token),
  updateFolder: (token: string, id: number, payload: UpdateFolderInput): Promise<FolderSummary> => apiFetch(`/v1/folders/${id}`, token, { method: "PUT", body: JSON.stringify(payload) }),
  deleteFolder: (token: string, id: number): Promise<void> => apiFetch(`/v1/folders/${id}`, token, { method: "DELETE" }),
  addStudySetToFolder: (token: string, folderId: number, studySetId: number): Promise<void> => apiFetch(`/v1/folders/${folderId}/study-sets`, token, { method: "POST", body: JSON.stringify({ studySetId }) }),
  removeStudySetFromFolder: (token: string, folderId: number, studySetId: number): Promise<void> => apiFetch(`/v1/folders/${folderId}/study-sets/${studySetId}`, token, { method: "DELETE" }),
};

export type LearningMode = "flashcards" | "learn" | "test" | "match";
export type LearningProgress = { id: number; userId: number; studySetId: number; mode: LearningMode; score: number; total: number; completedAt: string | null; createdAt: string };

export type QuizGeneratedItem = {
  id: string;
  flashcardId: number;
  kind: "term" | "definition" | "question" | "pair";
  text?: string;
  term?: string;
  definition?: string;
  choices?: string[];
  pairId?: string;
  starred?: boolean;
  position?: number;
};
export type QuizGenerateResponse = {
  mode: LearningMode;
  seed: number;
  items: QuizGeneratedItem[];
  contractVersion: string;
};
export type QuizAnswer = {
  flashcardId: number;
  submitted?: string;
  selectedIndex?: number;
  pairId?: string;
  matchedFlashcardId?: number;
  attempts: number;
  responseTimeMs?: number;
};
export type QuizEvaluateResponse = {
  mode: LearningMode;
  seed: number;
  score: number;
  total: number;
  cardResults: Array<{ flashcardId: number; correct: boolean; attempts: number; responseTimeMs?: number }>;
  contractVersion: string;
};
export const quizApi = {
  generate: (token: string, studySetId: number, payload: { mode: LearningMode; seed: number; limit?: number; options?: Record<string, unknown> }, signal?: AbortSignal): Promise<QuizGenerateResponse> =>
    apiFetch(`/v1/study-sets/${studySetId}/quiz/generate`, token, { method: "POST", body: JSON.stringify(payload), signal }),
  evaluate: (token: string, studySetId: number, payload: { mode: LearningMode; seed: number; limit: number; answers: QuizAnswer[] }, signal?: AbortSignal): Promise<QuizEvaluateResponse> =>
    apiFetch(`/v1/study-sets/${studySetId}/quiz/evaluate`, token, { method: "POST", body: JSON.stringify(payload), signal }),
  create: async (token: string, payload: CreateStudySetPayload & { questions?: QuizQuestionPayload[] }): Promise<StudySet> => {
    const set = await studySetApi.create(token, payload);
    if (payload.questions?.length) {
      await quizQuestionApi.bulkSave(token, set.id, payload.questions);
    }
    return studySetApi.get(token, set.id);
  },
};

export type ServiceHealth = { name: string; url: string; status: string };
export async function fetchHealth(): Promise<ServiceHealth[]> { const data = await apiFetch<{ services: ServiceHealth[] }>("/healthz/services", ""); return data.services ?? []; }
export const healthApi = { check: () => apiFetch<{ service: string; status: string }>("/healthz", ""), services: () => apiFetch<{ services: ServiceHealth[] }>("/healthz/services", "") };

// --- Phase 7: Class & Activity API ---

export type CreateClassPayload = { name: string; description?: string; maxMembers?: number };
export type UpdateClassPayload = { name?: string; description?: string };

export const classApi = {
  list: (token: string): Promise<ClassSummary[]> => apiFetch("/v1/classes", token),
  create: (token: string, payload: CreateClassPayload): Promise<ClassDetail> => apiFetch("/v1/classes", token, { method: "POST", body: JSON.stringify(payload) }),
  get: (token: string, id: number): Promise<ClassDetail> => apiFetch(`/v1/classes/${id}`, token),
  update: (token: string, id: number, payload: UpdateClassPayload): Promise<ClassDetail> => apiFetch(`/v1/classes/${id}`, token, { method: "PUT", body: JSON.stringify(payload) }),
  delete: (token: string, id: number): Promise<void> => apiFetch(`/v1/classes/${id}`, token, { method: "DELETE" }),
  join: (token: string, code: string): Promise<JoinClassResponse> => apiFetch(`/v1/classes/${code}/join`, token, { method: "POST" }),
  resetInviteCode: (token: string, id: number): Promise<{ inviteCode: string }> => apiFetch(`/v1/classes/${id}/invite-code/reset`, token, { method: "POST" }),
};

export const memberApi = {
  list: (token: string, classId: number): Promise<ClassMember[]> => apiFetch(`/v1/classes/${classId}/members`, token),
  add: (token: string, classId: number, payload: { userId: number; role: string }): Promise<ClassMember> => apiFetch(`/v1/classes/${classId}/members`, token, { method: "POST", body: JSON.stringify(payload) }),
  updateRole: (token: string, classId: number, userId: number, role: string): Promise<void> => apiFetch(`/v1/classes/${classId}/members/${userId}`, token, { method: "PUT", body: JSON.stringify({ role }) }),
  remove: (token: string, classId: number, userId: number): Promise<void> => apiFetch(`/v1/classes/${classId}/members/${userId}`, token, { method: "DELETE" }),
  leave: (token: string, classId: number): Promise<void> => apiFetch(`/v1/classes/${classId}/members/me`, token, { method: "DELETE" }),
};

export const classStudySetApi = {
  list: (token: string, classId: number): Promise<ClassStudySet[]> => apiFetch(`/v1/classes/${classId}/study-sets`, token),
  add: (token: string, classId: number, studySetId: number): Promise<void> => apiFetch(`/v1/classes/${classId}/study-sets`, token, { method: "POST", body: JSON.stringify({ studySetId }) }),
  remove: (token: string, classId: number, studySetId: number): Promise<void> => apiFetch(`/v1/classes/${classId}/study-sets/${studySetId}`, token, { method: "DELETE" }),
};

export const activityApi = {
  getFeed: (token: string, cursor?: string, limit?: number): Promise<ActivityFeedResponse> => apiFetch("/v1/activity", token, {}, { cursor, limit: limit?.toString() }),
};

export const walletApi = {
  getBalance: (token: string): Promise<WalletBalance> => apiFetch("/v1/wallet", token),
  getTransactions: (token: string, limit = 20, offset = 0): Promise<WalletTransactionList> =>
    apiFetch("/v1/wallet/transactions", token, {}, { limit, offset }),
};

export const paymentApi = {
  createOrder: (token: string, amountVnd: number): Promise<PaymentOrder> =>
    apiFetch("/v1/payments/orders", token, { method: "POST", body: JSON.stringify({ amountVnd }) }),
  getOrderStatus: (token: string, orderId: number): Promise<DepositOrderStatus> =>
    apiFetch(`/v1/payments/orders/${orderId}`, token),
};

export const entitlementApi = {
  checkAccess: (token: string, studySetId: number): Promise<StudySetAccessInfo> =>
    apiFetch("/v1/entitlements/check", token, {}, { study_set_id: studySetId }),
  purchase: (token: string, studySetId: number): Promise<PurchaseResult> =>
    apiFetch("/v1/entitlements/purchase", token, { method: "POST", body: JSON.stringify({ studySetId }) }),
};

export const priceApi = {
  update: (token: string, studySetId: number, payload: { pricingType: string; priceVnd: number }) =>
    apiFetch(`/v1/study-sets/${studySetId}/price`, token, { method: "PUT", body: JSON.stringify(payload) }),
};

export const adminApi = {
  listOrders: (token: string, limit = 50, offset = 0): Promise<AdminOrderList> =>
    apiFetch("/v1/admin/payments/orders", token, {}, { limit, offset }),
  listTransactions: (token: string, limit = 50, offset = 0): Promise<AdminTxList> =>
    apiFetch("/v1/admin/wallet/transactions", token, {}, { limit, offset }),
  credit: (token: string, userId: number, amountVnd: number, note: string): Promise<void> =>
    apiFetch("/v1/admin/wallet/credit", token, { method: "POST", body: JSON.stringify({ userId, amountVnd, note }) }),
};

export type QuizQuestionPayload = {
  questionText: string;
  questionType: string;
  correctAnswer?: string;
  timeInSeconds?: number;
  audioUrl?: string;
  answerExplanation?: string;
  paragraphText?: string;
  subQuestions?: unknown;
  tags?: string[];
  options?: Array<{ text: string; position: number; isCorrect?: boolean }>;
  position: number;
};

export const quizQuestionApi = {
  list: (token: string, studySetId: number) => apiFetch(`/v1/study-sets/${studySetId}/quiz-questions`, token),
  bulkSave: (token: string, studySetId: number, questions: QuizQuestionPayload[]): Promise<void> =>
    apiFetch(`/v1/study-sets/${studySetId}/quiz-questions`, token, { method: "PUT", body: JSON.stringify({ questions }) }),
  deleteByStudySet: (token: string, studySetId: number): Promise<void> =>
    apiFetch(`/v1/study-sets/${studySetId}/quiz-questions`, token, { method: "DELETE" }),
};

export const grammarApi = {
  create: (token: string, payload: CreateStudySetPayload & { grammarPoints?: unknown[] }): Promise<StudySet> =>
    studySetApi.create(token, payload),
};

export type ImportResult = { imported: number; errors: Array<{ row: number; field: string; reason: string }> };
async function uploadExcel(token: string, path: string, file: File): Promise<ImportResult> {
  const form = new FormData();
  form.append("file", file);
  return apiFetch(path, token, { method: "POST", body: form, headers: {} });
}
export const importApi = {
  flashcards: (token: string, studySetId: number, file: File): Promise<ImportResult> =>
    uploadExcel(token, `/v1/study-sets/${studySetId}/flashcards/import`, file),
  quiz: (token: string, studySetId: number, file: File): Promise<ImportResult> =>
    uploadExcel(token, `/v1/study-sets/${studySetId}/quiz/import`, file),
};

export type Language = { code: string; name: string };
export const languageApi = {
  list: (token: string): Promise<Language[]> => apiFetch("/v1/languages", token),
};

export const ttsApi = {
  getAudio: async (token: string, text: string, lang: string): Promise<Blob> => {
    const url = new URL(`${gatewayUrl}/v1/tts`);
    url.searchParams.set("text", text);
    url.searchParams.set("lang", lang);
    const res = await fetch(url.toString(), { headers: token ? { Authorization: `Bearer ${token}` } : {} });
    if (!res.ok) throw new ApiError(res.status, `Request failed ${res.status}`);
    return res.blob();
  },
};

export type {
  ActivityFeedResponse,
  AuthResponse,
  ClassDetail,
  ClassMember,
  ClassStudySet,
  ClassSummary,
  DraftCard,
  Flashcard,
  JoinClassResponse,
  StudySet,
  User,
} from "../../types";
