import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { StudyModes, modePercent } from "./StudyModes";

const noop = () => {};

describe("modePercent", () => {
  it("rounds the last session score and clamps to 0..100", () => {
    expect(modePercent({ score: 8, total: 13, completedAt: null })).toBe(62);
    expect(modePercent({ score: 0, total: 10, completedAt: null })).toBe(0);
    expect(modePercent({ score: 12, total: 10, completedAt: null })).toBe(100);
  });

  it("returns null when there is no completed session", () => {
    expect(modePercent(undefined)).toBeNull();
    expect(modePercent({ score: 0, total: 0, completedAt: null })).toBeNull();
  });
});

describe("StudyModes hub", () => {
  it("lists every way to study the set", () => {
    const markup = renderToStaticMarkup(<StudyModes activeMode="flashcards" onSelectMode={noop} />);
    for (const label of ["Thẻ ghi nhớ", "Học", "Kiểm tra", "Ghép thẻ"]) {
      expect(markup).toContain(label);
    }
  });

  it("shows the learner's own progress per mode", () => {
    const markup = renderToStaticMarkup(
      <StudyModes
        activeMode="learn"
        onSelectMode={noop}
        progress={{ learn: { score: 8, total: 13, completedAt: "2026-10-19T08:00:00Z" } }}
      />,
    );
    expect(markup).toContain("Đúng 62% · 8/13 câu");
    expect(markup).toContain('width:62%');
    expect(markup).toContain("Chưa học");
    expect(markup).toContain('aria-label="Học, lần trước đúng 62%"');
  });

  it("marks the active mode for assistive tech and styling", () => {
    const markup = renderToStaticMarkup(
      <StudyModes activeMode="test" onSelectMode={noop} progress={{ test: { score: 5, total: 10, completedAt: null } }} />,
    );
    expect(markup).toContain('aria-current="page"');
    expect(markup).toContain("sd-mode-card--active");
  });

  it("offers one primary action that adapts to progress", () => {
    const fresh = renderToStaticMarkup(<StudyModes activeMode="flashcards" onSelectMode={noop} onContinue={noop} />);
    expect(fresh).toContain("Bắt đầu học");

    const returning = renderToStaticMarkup(
      <StudyModes
        activeMode="flashcards"
        onSelectMode={noop}
        onContinue={noop}
        progress={{ match: { score: 4, total: 8, completedAt: null } }}
      />,
    );
    expect(returning).toContain("Tiếp tục học");
  });
});
