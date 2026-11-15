import { describe, it, expect } from "vitest";
import type { MangaVolume } from "../../lib/api/manga";
import { groupMangaSeries, volumeLabel } from "./mangaSeries";
const volume = (overrides: Partial<MangaVolume> = {}): MangaVolume => ({
  id: 1,
  ownerId: 1,
  classId: 0,
  seriesId: 10,
  sourceUuid: "first",
  title: "A story",
  volume: "Tập 1",
  visibility: "shared",
  status: "ready",
  pageCount: 20,
  uploaded: 20,
  canEdit: true,
  canDelete: true,
  ...overrides,
});
describe("multi-volume manga library", () => {
  it("keeps a single existing volume as a story without changing its id or data", () => {
    const old = volume();
    const stories = groupMangaSeries([old]);
    expect(stories).toHaveLength(1);
    expect(stories[0].volumes[0]).toBe(old);
    expect(stories[0].volumes[0].pageCount).toBe(20);
  });
  it("groups by stable series identity and sorts 2 before 10", () => {
    const original = [
      volume({ id: 10, volume: "Tập 10" }),
      volume({ id: 2, volume: "Tập 2" }),
      volume({ id: 1, volume: "Tập 1" }),
    ];
    const groups = groupMangaSeries(original);
    expect(groups).toHaveLength(1);
    expect(groups[0].volumes.map((v) => v.id)).toEqual([1, 2, 10]);
    expect(original.map((v) => v.id)).toEqual([10, 2, 1]);
  });
  it("does not merge identically named stories from different owners or spaces", () => {
    const groups = groupMangaSeries([
      volume({ seriesId: 0 }),
      volume({ id: 2, seriesId: 0, ownerId: 2 }),
      volume({ id: 3, seriesId: 0, classId: 12 }),
    ]);
    expect(groups).toHaveLength(3);
  });
  it("shows a fallback name for an old volume with no label", () => {
    expect(volumeLabel({ volume: "" }, 0)).toBe("Tập 1");
  });
});
