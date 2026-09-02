// Mock data adapter (Dev 3)
// Used while backend (Dev 1, Dev 2) is not yet ready.
// Swap to real API client when Dev 5 provides api-contracts.

import type { StudySet, Flashcard, User, AuthResponse } from "../../types";

export const MOCK_USER: User = {
  id: 1,
  name: "Demo User",
  email: "demo@hquizlet.local",
  role: "user",
};

export const MOCK_FLASHCARDS: Flashcard[] = [
  { id: 1, studySetId: 1, term: "apple", definition: "quả táo", starred: false },
  { id: 2, studySetId: 1, term: "book", definition: "cuốn sách", starred: true },
  { id: 3, studySetId: 1, term: "computer", definition: "máy tính", starred: false },
];

export const MOCK_SETS: StudySet[] = [
  {
    id: 1,
    title: "English Vocabulary Unit 1",
    description: "Basic English words for beginners",
    contentType: "flashcard",
    termLanguage: "en-US",
    definitionLanguage: "vi-VN",
    visibility: "public",
    flashcards: MOCK_FLASHCARDS,
  },
  {
    id: 2,
    title: "Từ vựng Tiếng Anh cơ bản",
    description: "Học từ vựng tiếng Anh hàng ngày",
    contentType: "flashcard",
    termLanguage: "en-US",
    definitionLanguage: "vi-VN",
    visibility: "public",
    flashcards: [],
  },
];

export function mockLogin(email: string, _password: string): AuthResponse {
  return {
    authenticated: true,
    token: "mock-token-" + Date.now(),
    user: { ...MOCK_USER, email },
  };
}

export function mockRegister(name: string, email: string, _password: string): AuthResponse {
  return {
    authenticated: true,
    token: "mock-token-" + Date.now(),
    user: { id: Date.now(), name, email, role: "user" },
  };
}
