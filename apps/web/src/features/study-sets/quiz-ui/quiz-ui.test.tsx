import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

import { AnswerOption } from "./AnswerOption";
import { QuestionGrid } from "./QuestionGrid";
import { QuizPageSkeleton } from "./QuizShell";
import { StatusPill } from "./StatusPill";

describe("quiz answer option", () => {
  it("shows a pending state instead of an unchanged screen", () => {
    const markup = renderToStaticMarkup(<AnswerOption letter="B" text="Đáp án B" state="pending" disabled onSelect={() => {}} />);
    expect(markup).toContain("Đang chấm…");
    expect(markup).toContain('aria-busy="true"');
    expect(markup).toContain("disabled");
  });

  it("never encodes correctness with colour alone", () => {
    const correct = renderToStaticMarkup(<AnswerOption letter="A" text="Đúng rồi" state="correct" disabled onSelect={() => {}} />);
    expect(correct).toContain(">✓</span>");
    expect(correct).toContain(">Đúng</span>");
    expect(correct).toContain('aria-pressed="true"');
    const wrong = renderToStaticMarkup(<AnswerOption letter="C" text="Sai rồi" state="wrong" disabled onSelect={() => {}} />);
    expect(wrong).toContain(">✗</span>");
    expect(wrong).toContain(">Sai</span>");
  });
});

describe("save status pill", () => {
  it("stays silent when idle and announces every other state", () => {
    expect(renderToStaticMarkup(<StatusPill state="idle" />)).toBe("");
    expect(renderToStaticMarkup(<StatusPill state="saving" />)).toContain("Đang lưu…");
    expect(renderToStaticMarkup(<StatusPill state="saved" />)).toContain("Đã lưu");
  });

  it("offers a retry when saving failed", () => {
    const markup = renderToStaticMarkup(<StatusPill state="error" onRetry={() => {}} />);
    expect(markup).toContain("Chưa lưu được");
    expect(markup).toContain("Thử lại");
    expect(markup).toContain('aria-live="polite"');
  });
});

describe("question grid", () => {
  const items = [
    { number: 1, part: 1 },
    { number: 2, part: 1 },
    { number: 3, part: 5 },
  ];

  it("renders nothing while closed", () => {
    expect(
      renderToStaticMarkup(
        <QuestionGrid open={false} items={items} answers={{}} flags={[]} currentIndex={0} onNavigate={() => {}} onClose={() => {}} />,
      ),
    ).toBe("");
  });

  it("labels every cell with its state and marks flagged questions", () => {
    const markup = renderToStaticMarkup(
      <QuestionGrid
        open
        items={items}
        answers={{ 0: "A" }}
        flags={[false, true, false]}
        currentIndex={1}
        onNavigate={() => {}}
        onClose={() => {}}
      />,
    );
    expect(markup).toContain("Câu 1, Part 1, đã làm");
    expect(markup).toContain("Câu 2, Part 1, chưa làm, đã đánh dấu");
    expect(markup).toContain("Chưa làm (2)");
    expect(markup).toContain("Đánh dấu (1)");
    expect(markup).toContain("⚑");
    expect(markup).toContain('aria-current="true"');
  });

  it("shows correct and wrong results after submission", () => {
    const markup = renderToStaticMarkup(
      <QuestionGrid
        open
        items={items}
        answers={{ 0: "A", 1: "B" }}
        flags={[]}
        results={{ 0: true, 1: false }}
        currentIndex={0}
        onNavigate={() => {}}
        onClose={() => {}}
      />,
    );
    expect(markup).toContain("✓");
    expect(markup).toContain("✗");
    expect(markup).toContain("qz-grid-cell--correct");
    expect(markup).toContain("qz-grid-cell--wrong");
  });
});

describe("page skeleton", () => {
  it("announces loading instead of leaving the screen blank", () => {
    const markup = renderToStaticMarkup(<QuizPageSkeleton />);
    expect(markup).toContain('role="status"');
    expect(markup).toContain("Đang tải câu hỏi");
  });
});
