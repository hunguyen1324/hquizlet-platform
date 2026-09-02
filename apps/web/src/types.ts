// Shared domain types for hquizlet web app
// Dev 3 - core types, aligned with OpenAPI contract (Dev 5)

export type HealthStatus = "checking" | "live" | "offline";
export type AuthMode = "login" | "register";
export type AppView = "dashboard" | "editor" | "study" | "folders";

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
  flashcardCount?: number;
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
