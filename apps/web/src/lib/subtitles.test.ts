import { describe, expect, it } from "vitest";
import { normalizeSubtitleText, parseSubtitleText } from "./subtitles";

describe("listening subtitles", () => {
  it("accepts UTF-8 BOM, Windows line endings and multi-line cues", () => {
    const raw = "\uFEFF1\r\n00:00:00,100 --> 00:00:02,000\r\nXin chào\r\nこんにちは\r\n\r\n2\r\n00:00:02,000 --> 00:00:03,000\r\n<b>Hello</b>\r\n";
    expect(parseSubtitleText(raw)).toEqual([{ start: .1, end: 2, text: "Xin chào\nこんにちは" }, { start: 2, end: 3, text: "Hello" }]);
    expect(normalizeSubtitleText(raw)).not.toContain("\r");
  });
  it("rejects malformed, reversed and empty cues", () => {
    for (const raw of ["not SRT", "1\n00:00:02,000 --> 00:00:01,000\ntext", "1\n00:66:00,000 --> 00:67:00,000\ntext", "1\n00:00:00,000 --> 00:00:01,000\n"]) expect(() => parseSubtitleText(raw)).toThrow();
  });
  it("allows clearing subtitles and limits encoded byte length", () => {
    expect(normalizeSubtitleText("")).toBe("");
    expect(() => parseSubtitleText("1\n00:00:00,000 --> 00:00:01,000\n" + "日".repeat(400000))).toThrow(/MiB/);
  });
});
