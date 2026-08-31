// apps/web/src/lib/api/index.ts — Dev 5 [P2-INT-02]
// Single public entrypoint for the typed API client.

export {
  apiFetch,
  authApi,
  studySetApi,
  flashcardApi,
  folderApi,
  learningApi,
  healthApi,
  fetchHealth,
  ApiError,
} from "./client";

export type {
  User,
  AuthResponse,
  StudySet,
  Flashcard,
  DraftCard,
  Folder,
  FolderDetail,
  LearningMode,
  LearningProgress,
  StudySetListParams,
  StudySetListResult,
  BulkFlashcardItem,
  BulkSaveResult,
  CreateStudySetPayload,
  UpdateStudySetPayload,
  CreateFlashcardPayload,
  UpdateFlashcardPayload,
  ServiceHealth,
} from "./client";
