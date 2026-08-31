// apps/web/src/lib/api/index.ts
// Dev 5 – P2-INT-02: Re-export toàn bộ API client
// Feature khác import từ đây: import { authApi, studySetApi, ... } from "../../lib/api"

export {
  authApi,
  studySetApi,
  flashcardApi,
  folderApi,
  learningApi,
  healthApi,
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
  BulkFlashcardItem,
  ServiceHealth,
} from "./client";
