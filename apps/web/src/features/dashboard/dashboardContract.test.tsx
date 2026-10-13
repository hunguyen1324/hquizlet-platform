import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { DataTable, KpiCard, SectionCard } from "../../components/dashboard";
import { StudentDashboard } from "./StudentDashboard";
import type { StudyStats } from "../../types";

const stats: StudyStats = {
  rangeDays: 28,
  streakDays: 6,
  studiedToday: true,
  sessionsTotal: 40,
  sessionsInRange: 12,
  cardsInRange: 120,
  correctInRange: 90,
  accuracyPct: 75,
  dueCards: 4,
  series: [
    { date: "2026-10-12", sessions: 2, cards: 20, correct: 15 },
    { date: "2026-10-13", sessions: 1, cards: 10, correct: 9 },
  ],
  recentSessions: [
    { studySetId: 7, studySetTitle: "TOEIC 600", mode: "learn", score: 9, total: 10, completedAt: null, createdAt: "2026-10-13T09:00:00Z" },
  ],
  continueLearning: [
    { studySetId: 7, title: "TOEIC 600", lastMode: "learn", sessions: 3, bestScore: 9, lastPlayedAt: "2026-10-13T09:00:00Z" },
  ],
  ownedSets: [],
};

function noop() {}

describe("dashboard components", () => {
  it("KpiCard shows label, value and hint", () => {
    const html = renderToStaticMarkup(<KpiCard label="Chuỗi ngày học" value="6 ngày" hint="Hôm nay đã học 🎉" />);
    expect(html).toContain("Chuỗi ngày học");
    expect(html).toContain("6 ngày");
    expect(html).toContain("Hôm nay đã học");
  });

  it("KpiCard renders a skeleton while loading instead of a stale value", () => {
    const html = renderToStaticMarkup(<KpiCard label="Phiên học" value="99" loading />);
    expect(html).not.toContain("99");
    expect(html).toContain("animate-pulse");
  });

  it("DataTable falls back to its empty state", () => {
    const html = renderToStaticMarkup(
      <DataTable columns={[{ key: "a", header: "A" }]} rows={[]} rowKey={() => 1} empty={<p>Chưa có dữ liệu ABC</p>} />
    );
    expect(html).toContain("Chưa có dữ liệu ABC");
  });

  it("DataTable renders rows with a custom cell renderer", () => {
    const html = renderToStaticMarkup(
      <DataTable
        columns={[
          { key: "name", header: "Tên" },
          { key: "score", header: "Điểm", align: "right", render: (row: { score: number }) => `${row.score}/10` },
        ]}
        rows={[{ name: "Lớp 12A", score: 8 }]}
        rowKey={(row) => row.name}
      />
    );
    expect(html).toContain("Lớp 12A");
    expect(html).toContain("8/10");
  });

  it("SectionCard renders title, description and actions", () => {
    const html = renderToStaticMarkup(
      <SectionCard title="Tiêu đề" description="Mô tả" actions={<button>Hành động</button>}>
        <p>Nội dung</p>
      </SectionCard>
    );
    expect(html).toContain("Tiêu đề");
    expect(html).toContain("Mô tả");
    expect(html).toContain("Hành động");
  });
});

describe("StudentDashboard", () => {
  it("renders the KPI values and continue-learning row from real stats", () => {
    const html = renderToStaticMarkup(
      <StudentDashboard stats={stats} days={28} loading={false} onOpenSet={noop} onNavigate={noop} onRetry={noop} />
    );
    expect(html).toContain("6 ngày");
    expect(html).toContain("75%");
    expect(html).toContain("TOEIC 600");
    expect(html).toContain("Tiếp tục");
  });

  it("surfaces a retryable error instead of a blank page", () => {
    const html = renderToStaticMarkup(
      <StudentDashboard days={28} loading={false} error="Không tải được số liệu" onOpenSet={noop} onNavigate={noop} onRetry={noop} />
    );
    expect(html).toContain("Không tải được số liệu");
    expect(html).toContain("Thử lại");
  });
});
