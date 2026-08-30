// mockData.ts — Dev 4
// Local mock data for Sprint 1 development (no backend dependency)
// Remove or replace with real API calls in Sprint 2

import type { StudySet } from "../../types";

export const MOCK_STUDY_SET: StudySet = {
  id: 1,
  title: "Tiếng Anh Cơ Bản",
  description: "Mock data — Dev 4 Sprint 1",
  flashcards: [
    { id: 1, studySetId: 1, term: "Apple", definition: "Quả táo", starred: false },
    { id: 2, studySetId: 1, term: "Book", definition: "Quyển sách", starred: true },
    { id: 3, studySetId: 1, term: "Cat", definition: "Con mèo", starred: false },
    { id: 4, studySetId: 1, term: "Dog", definition: "Con chó", starred: true },
    { id: 5, studySetId: 1, term: "Elephant", definition: "Con voi", starred: false },
    { id: 6, studySetId: 1, term: "Fish", definition: "Con cá", starred: false },
    { id: 7, studySetId: 1, term: "Garden", definition: "Khu vườn", starred: false },
    { id: 8, studySetId: 1, term: "House", definition: "Ngôi nhà", starred: true },
  ],
};
