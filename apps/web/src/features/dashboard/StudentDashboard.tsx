// StudentDashboard — Phase 2 (UI-05)
// Student view: continue learning, streak, recent results, wallet shortcut.
import React from "react";
import {
  DataTable,
  EmptyState,
  KpiCard,
  SectionCard,
  TimeSeriesChart,
  formatDateTime,
  formatNumber,
  type Column,
} from "../../components/dashboard";
import type { ContinueItem, RecentSession, StudyStats } from "../../types";

const MODE_LABELS: Record<string, string> = {
  flashcards: "Thẻ ghi nhớ",
  learn: "Học",
  test: "Kiểm tra",
  match: "Ghép thẻ",
};

export function StudentDashboard({
  stats,
  days,
  loading,
  error,
  onOpenSet,
  onNavigate,
  onRetry,
}: {
  stats?: StudyStats;
  days: number;
  loading: boolean;
  error?: string;
  onOpenSet: (id: number) => void;
  onNavigate: (view: string) => void;
  onRetry: () => void;
}) {
  const continueColumns: Array<Column<ContinueItem>> = [
    {
      key: "title",
      header: "Học phần",
      render: (row) => (
        <div>
          <p className="font-medium">{row.title || `Học phần #${row.studySetId}`}</p>
          <p className="text-xs text-[var(--muted-foreground)]">
            {MODE_LABELS[row.lastMode] ?? row.lastMode} · {row.sessions} phiên
          </p>
        </div>
      ),
    },
    { key: "bestScore", header: "Điểm tốt nhất", align: "right", render: (row) => row.bestScore },
    { key: "lastPlayedAt", header: "Lần cuối", align: "right", render: (row) => formatDateTime(row.lastPlayedAt) },
    {
      key: "action",
      header: "",
      align: "right",
      render: (row) => (
        <button className="primary-button" onClick={() => onOpenSet(row.studySetId)}>
          Tiếp tục
        </button>
      ),
    },
  ];

  const recentColumns: Array<Column<RecentSession>> = [
    {
      key: "title",
      header: "Học phần",
      render: (row) => (
        <button className="text-left hover:underline" onClick={() => onOpenSet(row.studySetId)}>
          {row.studySetTitle || `Học phần #${row.studySetId}`}
        </button>
      ),
    },
    { key: "mode", header: "Chế độ", render: (row) => MODE_LABELS[row.mode] ?? row.mode },
    {
      key: "score",
      header: "Kết quả",
      align: "right",
      render: (row) => (
        <span className={row.total > 0 && row.score / row.total >= 0.8 ? "text-green-600 dark:text-green-400" : ""}>
          {row.score}/{row.total}
        </span>
      ),
    },
    { key: "createdAt", header: "Thời gian", align: "right", render: (row) => formatDateTime(row.createdAt) },
  ];

  if (error) {
    return (
      <div className="message message--error" role="alert">
        {error} <button className="ghost-button" onClick={onRetry}>Thử lại</button>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <KpiCard
          label="Chuỗi ngày học"
          value={`${stats?.streakDays ?? 0} ngày`}
          hint={stats?.studiedToday ? "Hôm nay đã học 🎉" : "Hôm nay chưa học"}
          loading={loading}
        />
        <KpiCard label={`Phiên học (${days} ngày)`} value={formatNumber(stats?.sessionsInRange)} loading={loading} />
        <KpiCard label="Độ chính xác" value={`${stats?.accuracyPct ?? 0}%`} loading={loading} />
        <KpiCard
          label="Thẻ cần ôn"
          value={formatNumber(stats?.dueCards)}
          hint="Câu trả lời sai gần nhất"
          loading={loading}
        />
      </div>

      <SectionCard title="Hoạt động học tập" description={`Số phiên học và số thẻ mỗi ngày trong ${days} ngày qua`}>
        <TimeSeriesChart
          data={(stats?.series ?? []).map((p) => ({ date: p.date, sessions: p.sessions, cards: p.cards }))}
          series={[
            { key: "sessions", label: "Phiên học", color: "#0ea5e9" },
            { key: "cards", label: "Thẻ đã học", color: "#8b5cf6" },
          ]}
        />
      </SectionCard>

      <div className="grid gap-4 lg:grid-cols-2">
        <SectionCard
          title="Tiếp tục học"
          description="Học phần bạn đang học dở"
          actions={
            <button className="ghost-button" onClick={() => onNavigate("dashboard")}>
              Thư viện
            </button>
          }
        >
          <DataTable
            columns={continueColumns}
            rows={stats?.continueLearning ?? []}
            rowKey={(row) => row.studySetId}
            loading={loading}
            empty={
              <EmptyState
                title="Chưa có học phần nào đang học"
                description="Mở thư viện và bắt đầu một phiên học đầu tiên."
                action={{ label: "Mở thư viện", onClick: () => onNavigate("dashboard") }}
              />
            }
          />
        </SectionCard>

        <SectionCard
          title="Kết quả gần đây"
          description="5 phiên học mới nhất"
          actions={
            <button className="ghost-button" onClick={() => onNavigate("quiz-sessions")}>
              Lịch sử
            </button>
          }
        >
          <DataTable
            columns={recentColumns}
            rows={stats?.recentSessions ?? []}
            rowKey={(row) => `${row.studySetId}-${row.createdAt}`}
            loading={loading}
            empty={<EmptyState title="Chưa có kết quả nào" description="Hoàn thành một phiên để xem kết quả ở đây." />}
          />
        </SectionCard>
      </div>
    </div>
  );
}
