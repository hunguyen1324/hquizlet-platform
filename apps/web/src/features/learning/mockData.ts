// mockData.ts — Dev 4
// Phase 2: Mock data chuyển sang lib/mock (Dev 5 convention)
// File này chỉ giữ để không break import cũ - KHÔNG dùng trong production flow

// Re-export from canonical mock location
export { MOCK_SETS as MOCK_STUDY_SET_LIST } from "../../lib/mock/mockData";

// Local dev mock for learning standalone mode only (không dùng trong main flow)
import type { StudySet } from "../../types";

export const MOCK_STUDY_SET: StudySet = {
  id: 0,
  title: "[Dev Mode] Mock Study Set",
  description: "Chỉ dùng khi chạy LearningRouter standalone, không phải trong main app",
  flashcards: [
    { id: 1, studySetId: 0, term: "Apple", definition: "Quả táo", starred: false },
    { id: 2, studySetId: 0, term: "Book", definition: "Quyển sách", starred: true },
    { id: 3, studySetId: 0, term: "Cat", definition: "Con mèo", starred: false },
    { id: 4, studySetId: 0, term: "Dog", definition: "Con chó", starred: true },
    { id: 5, studySetId: 0, term: "Elephant", definition: "Con voi", starred: false },
    { id: 6, studySetId: 0, term: "Fish", definition: "Con cá", starred: false },
  ],
};
