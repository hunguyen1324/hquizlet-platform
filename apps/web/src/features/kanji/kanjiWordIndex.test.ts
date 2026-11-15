import { describe, it, expect } from "vitest";
import { indexKanjiWords, queryKanjiWords } from "./kanjiWordIndex";
describe("Kanji inverted index", () => {
  it("connects words by unique ideographs and keeps readings per card", () => {
    const cards = [
      { id: 1, term: "時々", definition: "ときどき" },
      { id: 2, term: "時間", definition: "じかん" },
      { id: 3, term: "間", definition: "あいだ" },
      { id: 4, term: "かな", definition: "kana" },
    ];
    const groups = queryKanjiWords(indexKanjiWords(cards), "時間時");
    expect(groups.map((g) => g.char)).toEqual(["時", "間"]);
    expect(groups[0].words.map((w) => w.id)).toEqual([1, 2]);
    expect(groups[1].words.map((w) => w.id)).toEqual([2, 3]);
  });
  it("does not retain removed cards between deck versions", () => {
    const before = indexKanjiWords([
      { id: 1, term: "改造", definition: "かいぞう" },
    ]);
    expect(queryKanjiWords(before, "改")[0].words).toHaveLength(1);
    expect(queryKanjiWords(indexKanjiWords([]), "改")[0].words).toEqual([]);
  });
  it("supports supplementary ideographs without splitting surrogate pairs", () => {
    const index = indexKanjiWords([
      { id: 1, term: "𠮷野", definition: "よしの" },
    ]);
    expect(queryKanjiWords(index, "𠮷")[0].words).toHaveLength(1);
    expect(queryKanjiWords(index, "かな")).toEqual([]);
  });
});
