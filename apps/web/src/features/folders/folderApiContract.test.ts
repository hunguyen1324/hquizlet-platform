import { describe, expect, it } from "vitest";
import type { FolderDetail, FolderSummary } from "../../lib/api";

describe("folder API contract", () => {
  it("uses title and an explicit study-set count in summaries", () => {
    const folder: FolderSummary = { id: 1, title: "English", description: "IELTS", studySetCount: 2, createdAt: "2026-09-01T10:00:00Z", updatedAt: "2026-09-01T10:00:00Z" };
    expect(folder.title).toBe("English");
    expect(folder.studySetCount).toBe(2);
  });

  it("keeps detail count aligned with its study-set collection", () => {
    const folder: FolderDetail = { id: 1, title: "English", description: "", studySetCount: 0, studySets: [], createdAt: "2026-09-01T10:00:00Z", updatedAt: "2026-09-01T10:00:00Z" };
    expect(folder.studySets).toHaveLength(folder.studySetCount);
  });
});
