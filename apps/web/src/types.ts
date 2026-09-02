// Shared domain types for hquizlet web app
// Dev 3 - core types, aligned with OpenAPI contract (Dev 5)

export type HealthStatus = "checking" | "live" | "offline";
export type AuthMode = "login" | "register";
export type AppView = "dashboard" | "editor" | "study" | "folders" | "live" | "classes" | "class-detail" | "class-create" | "class-edit" | "class-join" | "activity";

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

// --- Phase 7: Class & Activity Types ---

export type ClassSummary = {
  id: number;
  name: string;
  description: string;
  inviteCode: string;
  memberCount: number;
  studySetCount: number;
  myRole: "owner" | "teacher" | "student";
  createdAt: string;
  updatedAt: string;
};

export type ClassDetail = ClassSummary & {
  maxMembers: number;
};

export type ClassMember = {
  id: number;
  classId: number;
  userId: number;
  role: "owner" | "teacher" | "student";
  joinedAt: string;
};

export type ClassStudySet = {
  classId: number;
  studySetId: number;
  title?: string;
  flashcardCount?: number;
  addedByUserId: number;
  addedAt: string;
};

export type JoinClassResponse = {
  classId: number;
  className: string;
  myRole: string;
  joinedAt: string;
};

export type ActivityItem = {
  id: number;
  eventType: string;
  entityType: string;
  entityId?: number;
  classId?: number;
  metadata?: Record<string, unknown>;
  occurredAt: string;
};

export type ActivityFeedResponse = {
  items: ActivityItem[];
  nextCursor?: string;
  hasMore: boolean;
};
