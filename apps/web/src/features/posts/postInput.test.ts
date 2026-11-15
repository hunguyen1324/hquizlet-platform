import { describe, expect, it } from "vitest";
import { postInput, postTitle } from "./postInput";

describe("community post input", () => {
  it("allows posting content without a separate title", () => {
    expect(postInput("  Hôm nay học Kanji\nCùng học nhé!  ", "public", "")).toEqual({
      kind: "post", title: "Hôm nay học Kanji", body: "Hôm nay học Kanji\nCùng học nhé!",
      visibility: "public", viewerIds: [],
    });
  });
  it("limits derived titles by Unicode characters", () => {
    expect(Array.from(postTitle("🌸".repeat(201)))).toHaveLength(200);
  });
  it("rejects blank and oversized content", () => {
    expect(() => postInput(" \n ", "public", "")).toThrow();
    expect(() => postInput("字".repeat(10001), "public", "")).toThrow();
    expect(postInput("🌸".repeat(10000), "public", "").body).toBeTruthy();
  });
  it("requires valid recipients for selected visibility and removes duplicates", () => {
    for (const value of ["", "0", "-1", "1.5", "abc", "9007199254740992"]) {
      expect(() => postInput("Bài viết", "selected", value)).toThrow();
    }
    expect(postInput("Bài viết", "selected", "2, 3 2").viewerIds).toEqual([2, 3]);
  });
  it("clears selected recipients when changing audience and preserves legacy titles", () => {
    const input = postInput("Nội dung mới", "private", "2,3", "Tiêu đề cũ");
    expect(input.viewerIds).toEqual([]);
    expect(input.title).toBe("Tiêu đề cũ");
  });
});
