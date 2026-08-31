// Shared domain types for hquizlet web app
// Dev 3 - core types, aligned with OpenAPI contract (Dev 5)

export type HealthStatus = "checking" | "live" | "offline";
export type AuthMode = "login" | "register";
export type AppView = "dashboard" | "editor" | "study";

export type User = {
  id: number;
  name: string;
  email: string;
  role: string;
};

export type AuthResponse = {
  authenticated: boolean;
  token: string;
  user: User;
};

export type ServiceHealth = {
  name: string;
  url: string;
  status: string;
};

export type StudySet = {
  id: number;
  title: string;
  description: string;
  flashcards?: Flashcard[];
};

export type Flashcard = {
  id: number;
  studySetId: number;
  term: string;
  definition: string;
  starred: boolean;
};

export type DraftCard = {
  key: string;
  id?: number;
  term: string;
  definition: string;
  starred?: boolean;
};

// Paginated list response từ GET /v1/study-sets
export type StudySetListResult = {
  items: StudySet[];
  total: number;
  page: number;
  perPage: number;
  totalPages: number;
};

// Bulk flashcard save item — ID=0 → create, ID>0 → update, delete=true → delete
export type BulkFlashcardItem = {
  id: number;       // 0 for new cards
  term: string;
  definition: string;
  position: number;
  delete: boolean;
};

export type BulkSavePayload = {
  cards: BulkFlashcardItem[];
};

export type BulkSaveResult = {
  created: Flashcard[];
  updated: Flashcard[];
  deleted: number[];
};
