// TeacherDashboard — Phase 2 (UI-06)
// Teacher view: own classes, study set usage, quick actions.
import React from "react";
import { useQuery } from "@tanstack/react-query";
import {
  DataTable,
  EmptyState,
  KpiCard,
  SectionCard,
  TimeSeriesChart,
  formatNumber,
  type Column,
} from "../../components/dashboard";
import { useAuth } from "../auth/AuthContext";
import { classApi } from "../../lib/api";
import type { ClassSummary, OwnedSetStat, StudyStats } from "../../types";

export function TeacherDashboard({
  stats,
  days,
  loading,
  error,
  onOpenClass,
  onNavigate,
  onRetry,
}: {
  stats?: StudyStats;
  days: number;
  loading: boolean;
  error?: string;
  onOpenClass: (id: number) => void;
  onNavigate: (view: string) => void;
  onRetry: () => void;
}) {
  const { token } = useAuth();
  const classesQuery = useQuery({
    queryKey: ["dashboard-classes"],
    queryFn: () => classApi.list(token),
    enabled: !!token,
    staleTime: 30_000,
  });

  const classes: ClassSummary[] = classesQuery.data ?? [];
  const members = classes.reduce((sum, c) => sum + (c.memberCount ?? 0), 0);
  const ownedSets = stats?.ownedSets ?? [];
  const totalLearners = ownedSets.reduce((sum, s) => sum + s.learners, 0);

  const needsAttention = ownedSets.filter((s) => s.learners === 0 || s.avgScorePct < 60);

  const classColumns: Array<Column<ClassSummary>> = [
    {
      key: "name",
      header: "Lớp",
      render: (row) => (
        <button className="text-left font-medium hover:underline" onClick={() => onOpenClass(row.id)}>
          {row.name}
        </button>
      ),
    },
    { key: "memberCount", header: "Sĩ số", align: "right", render: (row) => formatNumber(row.memberCount) },
    { key: "studySetCount", header: "Học phần", align: "right", render: (row) => formatNumber(row.studySetCount) },
    { key: "myRole", header: "Vai trò", render: (row) => roleLabel(row.myRole) },
  ];

  const setColumns: Array<Column<OwnedSetStat>> = [
    {
      key: "title",
      header: "Học phần",
      render: (row) => (
        <button className="text-left hover:underline" onClick={() => onNavigate("dashboard")}>
          {row.title || `Học phần #${row.studySetId}`}
        </button>
      ),
    },
    { key: "sessions", header: "Lượt học", align: "right", render: (row) => formatNumber(row.sessions) },
    { key: "learners", header: "Người học", align: "right", render: (row) => formatNumber(row.learners) },
    { key: "avgScorePct", header: "Điểm TB", align: "right", render: (row) => `${row.avgScorePct}%` },
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
        <KpiCard label="Lớp của tôi" value={formatNumber(classes.length)} loading={classesQuery.isLoading} />
        <KpiCard label="Tổng học sinh" value={formatNumber(members)} loading={classesQuery.isLoading} />
        <KpiCard label="Lượt học học phần" value={formatNumber(stats?.sessionsInRange)} hint={`${days} ngày qua`} loading={loading} />
        <KpiCard label="Người học học phần của tôi" value={formatNumber(totalLearners)} loading={loading} />
      </div>

      <div className="flex flex-wrap gap-2">
        <button className="primary-button" onClick={() => onNavigate("class-create")}>+ Tạo lớp</button>
        <button className="ghost-button" onClick={() => onNavigate("create-type")}>+ Tạo học phần</button>
        <button className="ghost-button" onClick={() => onNavigate("live")}>Bắt đầu Live Quiz</button>
      </div>

      <SectionCard title="Hoạt động học tập" description={`Lượt học và số thẻ theo ngày (${days} ngày)`}>
        <TimeSeriesChart
          data={(stats?.series ?? []).map((p) => ({ date: p.date, sessions: p.sessions, cards: p.cards }))}
          series={[
            { key: "sessions", label: "Phiên học", color: "#0ea5e9" },
            { key: "cards", label: "Thẻ đã học", color: "#22c55e" },
          ]}
        />
      </SectionCard>

      <div className="grid gap-4 lg:grid-cols-2">
        <SectionCard
          title="Lớp của tôi"
          description="Sĩ số và học phần đã giao"
          actions={<button className="ghost-button" onClick={() => onNavigate("classes")}>Xem tất cả</button>}
        >
          <DataTable
            columns={classColumns}
            rows={classes}
            rowKey={(row) => row.id}
            loading={classesQuery.isLoading}
            onRowClick={(row) => onOpenClass(row.id)}
            empty={
              <EmptyState
                title="Chưa có lớp nào"
                description="Tạo lớp để giao bài và theo dõi tiến độ học sinh."
                action={{ label: "Tạo lớp", onClick: () => onNavigate("class-create") }}
              />
            }
          />
        </SectionCard>

        <SectionCard title="Cần chú ý" description="Học phần chưa có người học hoặc điểm trung bình thấp">
          {needsAttention.length === 0 && !loading ? (
            <p className="p-3 text-sm text-[var(--muted-foreground)]">Không có học phần nào cần chú ý. 🎉</p>
          ) : (
            <ul className="space-y-2">
              {needsAttention.map((s) => (
                <li key={s.studySetId} className="rounded-lg border border-[var(--border)] p-3 text-sm">
                  <p className="font-medium">{s.title || `Học phần #${s.studySetId}`}</p>
                  <p className="text-xs text-[var(--muted-foreground)]">
                    {s.learners === 0 ? "Chưa có người học" : `Điểm trung bình ${s.avgScorePct}%`}
                  </p>
                </li>
              ))}
            </ul>
          )}
        </SectionCard>
      </div>

      <SectionCard
        title="Học phần của tôi"
        description="Lượt dùng và điểm trung bình"
        actions={<button className="ghost-button" onClick={() => onNavigate("dashboard")}>Thư viện</button>}
      >
        <DataTable
          columns={setColumns}
          rows={ownedSets}
          rowKey={(row) => row.studySetId}
          loading={loading}
          empty={
            <EmptyState
              title="Chưa có học phần nào"
              description="Tạo học phần đầu tiên để bắt đầu."
              action={{ label: "Tạo học phần", onClick: () => onNavigate("create-type") }}
            />
          }
        />
      </SectionCard>
    </div>
  );
}

function roleLabel(role?: string) {
  switch (role) {
    case "owner":
      return "Chủ lớp";
    case "teacher":
      return "Giáo viên";
    case "student":
      return "Học sinh";
    default:
      return role ?? "—";
  }
}
