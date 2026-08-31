// lib/api/index.ts — Dev 3
export { apiFetch, ApiError, authApi, studySetApi, flashcardApi, fetchHealth } from "./client";
export type {
  CreateStudySetPayload,
  UpdateStudySetPayload,
  CreateFlashcardPayload,
  UpdateFlashcardPayload,
  ServiceHealth,
} from "./client";
