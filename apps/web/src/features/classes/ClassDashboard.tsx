// ClassDashboard — Phase 3 (CLS-03/CLS-04/CLS-05)
// Teacher: whole-class progress, student table, CSV export, leaderboard switch.
// Student: own progress and, when the teacher enables it, the class leaderboard.
import React, { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  DataTable,
  EmptyState,
  KpiCard,
  SectionCard,
  formatDateTime,
  type Column,
} from "../../components/dashboard";
import { useAuth } from "../auth/AuthContext";
import { useCan } from "../auth/permissions";
import { classDashboardApi } from "../../lib/api";
import type { ClassDashboardResponse, ClassDashboardSetStat, ClassDashboardStudentRow } from "../../types";

export function ClassDashboard({
  classId,
  onBack,
  onOpenSet,
}: {
  classId: number;
  onBack: () => void;
  onOpenSet?: (studySetId: number) => void;
}) {
  const { token } = useAuth();
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: ["class-dashboard", classId],
    queryFn: () => classDashboardApi.get(token, classId),
    enabled: !!token && classId > 0,
    staleTime: 30_000,
  });
  const canEditClass = useCan("class-content:AssignStudySet", { classId });

  const [studentSort, setStudentSort] = useState("lastActivityAt");
  const [toggleError, setToggleError] = useState("");
  const leaderboard = useMutation({
    mutationFn: (enabled: boolean) => classDashboardApi.setLeaderboard(token, classId, enabled),
    onSuccess: () => {
      setToggleError("");
      void queryClient.invalidateQueries({ queryKey: ["class-dashboard", classId] });
    },
    onError: (err) => setToggleError(err instanceof Error ? err.message : "Không đổi được cài đặt."),
  });

  const data: ClassDashboardResponse | undefined = query.data;

  if (query.isLoading) {
    return (
      <div className="mx-auto max-w-6xl space-y-3 p-1" aria-busy="true">
        <div className="h-8 w-52 animate-pulse rounded bg-[var(--muted)]" />
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="h-24 animate-pulse rounded-2xl bg-[var(--muted)]" />
          ))}
        </div>
      </div>
    );
  }

  if (query.isError || !data) {
    return (
      <div className="mx-auto max-w-3xl p-4">
        <div className="message message--error" role="alert">
          Bạn không có quyền xem dashboard của lớp này, hoặc lớp không tồn tại.
        </div>
        <button className="ghost-button mt-3" onClick={onBack}>
          ← Quay lại lớp
        </button>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-6xl space-y-4 p-1">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <button className="ghost-button" onClick={onBack}>← Quay lại lớp</button>
          <h1 className="mt-1 text-xl font-semibold">Dashboard lớp {data.className}</h1>
          <p className="text-sm text-[var(--muted-foreground)]">
            {data.canViewAll ? "Toàn cảnh tiến độ của lớp" : "Tiến độ học tập của bạn trong lớp"}
          </p>
        </div>
        <div className="flex items-center gap-2">
          {data.canViewAll && (
            <button className="ghost-button" onClick={() => exportStudentsCsv(data)}>
              Xuất CSV
            </button>
          )}
          {canEditClass && (
            <label className="inline-flex items-center gap-2 rounded-lg border border-[var(--border)] px-3 py-1.5 text-xs">
              <input
                type="checkbox"
                checked={data.showLeaderboard}
                disabled={leaderboard.isPending}
                onChange={(e) => leaderboard.mutate(e.target.checked)}
              />
              Hiện bảng xếp hạng cho học sinh
            </label>
          )}
        </div>
      </header>
      {toggleError && <p className="message message--error" role="alert">{toggleError}</p>}

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <KpiCard label="Sĩ số" value={data.memberCount} />
        <KpiCard label="Hoạt động 7 ngày" value={data.activeMembers7d} hint="Thành viên có phiên học" />
        <KpiCard label="Học phần được giao" value={data.assignedSets.length} />
        {data.canViewAll ? (
          <KpiCard
            label="Tỷ lệ hoàn thành TB"
            value={`${averageCompletion(data.assignedSets)}%`}
            hint="Trên toàn bộ học phần"
          />
        ) : (
          <KpiCard
            label="Điểm trung bình của bạn"
            value={`${data.myProgress?.avgScorePct ?? 0}%`}
            hint={`${data.myProgress?.sessions ?? 0} phiên học`}
          />
        )}
      </div>

      <SectionCard
        title="Tiến độ từng học phần được giao"
        description={data.canViewAll ? "Tỷ lệ hoàn thành và điểm trung bình của lớp" : "Kết quả của bạn ở từng học phần"}
      >
        <DataTable
          columns={setColumns(data.canViewAll, onOpenSet)}
          rows={data.assignedSets}
          rowKey={(row) => row.studySetId}
          empty={<EmptyState title="Chưa có học phần nào được giao" description="Giáo viên chưa giao bài cho lớp." />}
        />
      </SectionCard>

      {!data.canViewAll && data.myProgress && (
        <SectionCard title="Tiến độ của tôi">
          <DataTable
            columns={studentColumns}
            rows={[data.myProgress]}
            rowKey={(row) => row.userId}
          />
        </SectionCard>
      )}

      {data.students.length > 0 && (
        <SectionCard
          title={data.canViewAll ? "Học sinh" : "Bảng xếp hạng"}
          description={data.canViewAll ? "Tiến độ và hoạt động của từng học sinh" : "Giáo viên đã bật bảng xếp hạng"}
        >
          {data.canViewAll && <label className="mb-3 flex items-center gap-2 text-sm">Sắp xếp
            <select value={studentSort} onChange={e => setStudentSort(e.target.value)}>
              <option value="lastActivityAt">Hoạt động gần nhất</option>
              <option value="avgScorePct">Điểm trung bình</option>
              <option value="completedSets">Tiến độ hoàn thành</option>
            </select>
          </label>}
          <DataTable
            columns={data.canViewAll ? studentColumns : studentColumns.filter(c => ["name","avgScorePct"].includes(c.key))}
            rows={[...data.students].sort((a, b) => (data.canViewAll ? studentSort : "avgScorePct") === "lastActivityAt" ? (b.lastActivityAt ?? "").localeCompare(a.lastActivityAt ?? "") : (!data.canViewAll || studentSort === "avgScorePct") ? b.avgScorePct - a.avgScorePct : b.completedSets - a.completedSets)}
            rowKey={(row) => row.userId}
            empty={<EmptyState title="Chưa có dữ liệu học tập" />}
          />
        </SectionCard>
      )}

      {data.canViewAll && data.students.length === 0 && (
        <SectionCard title="Học sinh">
          <EmptyState title="Chưa có học sinh nào học" description="Dữ liệu xuất hiện khi học sinh bắt đầu học phần được giao." />
        </SectionCard>
      )}

      {(data.recentActivity?.length ?? 0) > 0 && (
        <SectionCard title="Hoạt động gần đây" description="Dòng thời gian hoạt động của lớp">
          <ul className="space-y-2">
            {data.recentActivity!.map((event) => (
              <li key={event.id} className="flex items-center justify-between rounded-lg border border-[var(--border)] px-3 py-2 text-sm">
                <span>{activityLabel(event.eventType, event.metadata)}</span>
                <span className="text-xs text-[var(--muted-foreground)]">{formatDateTime(event.occurredAt)}</span>
              </li>
            ))}
          </ul>
        </SectionCard>
      )}
    </div>
  );
}

const studentColumns: Array<Column<ClassDashboardStudentRow>> = [
  { key: "name", header: "Học sinh", render: (row) => row.name || `Người dùng #${row.userId}` },
  { key: "sessions", header: "Phiên học", align: "right", render: (row) => row.sessions },
  { key: "completedSets", header: "Đã hoàn thành", align: "right", render: (row) => row.completedSets },
  {
    key: "avgScorePct",
    header: "Điểm TB",
    align: "right",
    render: (row) => (
      <span className={row.avgScorePct >= 80 ? "text-green-600 dark:text-green-400" : row.avgScorePct < 50 ? "text-amber-600 dark:text-amber-400" : ""}>
        {row.avgScorePct}%
      </span>
    ),
  },
  { key: "lastActivityAt", header: "Lần cuối", align: "right", render: (row) => formatDateTime(row.lastActivityAt) },
];

function setColumns(canViewAll: boolean, onOpenSet?: (id: number) => void): Array<Column<ClassDashboardSetStat>> {
  const columns: Array<Column<ClassDashboardSetStat>> = [
    {
      key: "title",
      header: "Học phần",
      render: (row) => (
        <button className="text-left font-medium hover:underline" onClick={() => onOpenSet?.(row.studySetId)}>
          {row.title || `Học phần #${row.studySetId}`}
        </button>
      ),
    },
    { key: "dueAt", header: "Hạn làm bài", render: row => row.dueAt ? new Date(row.dueAt).toLocaleString("vi-VN") : "—" },
    { key: "sessions", header: "Lượt học", align: "right", render: (row) => row.sessions },
    { key: "learners", header: "Người học", align: "right", render: (row) => row.learners },
    {
      key: "completionPct",
      header: "Hoàn thành",
      align: "right",
      render: (row) => (
        <div className="flex items-center justify-end gap-2">
          <div className="h-1.5 w-16 overflow-hidden rounded bg-[var(--muted)]">
            <div className="h-full rounded bg-sky-500" style={{ width: `${Math.min(100, row.completionPct)}%` }} />
          </div>
          <span className="tabular-nums">{row.completionPct}%</span>
        </div>
      ),
    },
    { key: "avgScorePct", header: "Điểm TB", align: "right", render: (row) => `${row.avgScorePct}%` },
  ];
  return canViewAll ? columns : columns.filter((c) => c.key !== "learners");
}

function averageCompletion(sets: ClassDashboardSetStat[]): number {
  if (sets.length === 0) return 0;
  return Math.round(sets.reduce((sum, s) => sum + s.completionPct, 0) / sets.length);
}

function activityLabel(eventType: string, metadata?: Record<string, unknown>): string {
  const className = typeof metadata?.className === "string" ? ` — ${metadata.className}` : "";
  switch (eventType) {
    case "class.member.joined":
      return `Thành viên mới tham gia${className}`;
    case "class.member.added":
      return `Giáo viên thêm thành viên${className}`;
    case "class.member.removed":
      return "Một thành viên đã rời lớp";
    case "class.studyset.added":
      return "Giao học phần mới cho lớp";
    case "class.studyset.removed":
      return "Gỡ học phần khỏi lớp";
    case "study.session_completed":
      return "Hoàn thành phiên học";
    case "quiz.completed":
      return "Hoàn thành bài kiểm tra";
    default:
      return eventType;
  }
}

/** CSV export is client-side so no extra endpoint can leak class-wide data. */
function exportStudentsCsv(data: ClassDashboardResponse) {
  const header = ["userId", "name", "email", "sessions", "completedSets", "avgScorePct", "lastActivityAt"];
  const rows = data.students.map((s) => [
    s.userId,
    csvCell(s.name),
    csvCell(s.email ?? ""),
    s.sessions,
    s.completedSets,
    s.avgScorePct,
    s.lastActivityAt ?? "",
  ]);
  const csv = [header.join(","), ...rows.map((r) => r.join(","))].join("\n");
  const blob = new Blob([`\uFEFF${csv}`], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = `class-${data.classId}-students.csv`;
  link.click();
  URL.revokeObjectURL(url);
}

function csvCell(value: string): string {
  if (/^[\s]*[=+\-@]/.test(value)) value = `'${value}`;
  return /[",\r\n]/.test(value) ? `"${value.replace(/"/g, '""')}"` : value;
}
