import React from "react";
import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { readFileSync, readdirSync } from "node:fs";
import { KanjiTools, kanjiCharacters } from "./KanjiTools";

describe("Kanji tools", () => {
  it("ignores kana and duplicate kanji while retaining supplementary ideographs", () => {
    expect(kanjiCharacters("日本語・日本ご𠮷")).toEqual(["日", "本", "語", "𠮷"]);
    expect(renderToStaticMarkup(<KanjiTools term="かな English" />)).toBe("");
  });
  it("offers both tools for each distinct character", () => {
    const html = renderToStaticMarkup(<KanjiTools term="休む" />);
    expect(html).toContain("Bộ thủ 休");
    expect(html).toContain("Cách viết 休");
  });
  it("ships stroke paths and radical metadata in valid matching shards", () => {
    const directory = new URL("../../../public/kanji/", import.meta.url);
    let count = 0;
    for (const file of readdirSync(directory).filter(file => file.endsWith(".json"))) {
      const data = JSON.parse(readFileSync(new URL(file, directory), "utf8"));
      for (const [char, value] of Object.entries(data)) {
        const item = value as { strokes: string[] };
        expect(char.codePointAt(0)!.toString(16).padStart(5, "0").slice(0, -2)).toBe(file.slice(0, -5));
        expect(item.strokes.length).toBeGreaterThan(0);
        expect(item.strokes.every(path => /^\s*m/i.test(path))).toBe(true);
        count++;
      }
    }
    expect(count).toBe(6703);
    const wood = JSON.parse(readFileSync(new URL("067.json", directory), "utf8"))["木"];
    expect(wood.strokes).toHaveLength(4);
    expect(wood.tree.children[0].radical).toBe(true);
    const rest = JSON.parse(readFileSync(new URL("04f.json", directory), "utf8"))["休"];
    expect(JSON.stringify(rest.tree)).toContain("亻");
    expect(JSON.stringify(rest.tree)).toContain("木");
  });
});
