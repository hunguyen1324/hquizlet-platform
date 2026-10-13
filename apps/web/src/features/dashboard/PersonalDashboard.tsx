// PersonalDashboard — Phase 2 (UI-05/UI-06)
// Picks the student or teacher layout from the caller's platform roles.
import React, { useState } from "react";
import { DateRangePicker, type RangeDays } from "../../components/dashboard";
import { useAuth } from "../auth/AuthContext";
import { usePlatformRoles } from "../auth/permissions";
import { useStudyStats } from "./useStudyStats";
import { StudentDashboard } from "./StudentDashboard";
import { TeacherDashboard } from "./TeacherDashboard";
import { ClassAttention } from "./ClassAttention";

export function PersonalDashboard({
  onOpenSet,
  onOpenClass,
  onNavigate,
}: {
  onOpenSet: (id: number) => void;
  onOpenClass: (id: number) => void;
  onNavigate: (view: string) => void;
}) {
  const { user } = useAuth();
  const roles = usePlatformRoles();
  const [days, setDays] = useState<number>(28);
  const query = useStudyStats(days);

  const isTeacher = roles.includes("Teacher") || roles.includes("PlatformAdmin") || roles.includes("OrgTeacher");
  const isStudent = roles.includes("Student") || roles.includes("OrgStudent");
  // A user may be both a teacher and a student (plan §9 #2); let them switch views.
  const [mode, setMode] = useState<"teacher" | "student">(isTeacher ? "teacher" : "student");
  const showToggle = isTeacher && isStudent;

  const body = mode === "teacher" && isTeacher ? (
    <TeacherDashboard
      stats={query.data}
      days={days}
      loading={query.isLoading}
      error={query.isError ? "Không tải được số liệu dashboard." : undefined}
      onOpenClass={onOpenClass}
      onNavigate={onNavigate}
      onRetry={() => void query.refetch()}
    />
  ) : (
    <StudentDashboard
      stats={query.data}
      days={days}
      loading={query.isLoading}
      error={query.isError ? "Không tải được số liệu dashboard." : undefined}
      onOpenSet={onOpenSet}
      onNavigate={onNavigate}
      onRetry={() => void query.refetch()}
    />
  );

  return (
    <div className="mx-auto max-w-6xl space-y-4 p-1">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold">Xin chào, {user?.name ?? "bạn"}</h1>
          <p className="text-sm text-[var(--muted-foreground)]">
            {mode === "teacher" ? "Tổng quan lớp học và học phần của bạn" : "Tiến độ học tập của bạn"}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {showToggle && (
            <div className="inline-flex overflow-hidden rounded-lg border border-[var(--border)] text-xs">
              {(["teacher", "student"] as const).map((m) => (
                <button
                  key={m}
                  type="button"
                  aria-pressed={mode === m}
                  onClick={() => setMode(m)}
                  className={`px-3 py-1.5 font-medium ${
                    mode === m ? "bg-[var(--primary,#0ea5e9)] text-white" : "bg-[var(--card)] hover:bg-[var(--muted)]"
                  }`}
                >
                  {m === "teacher" ? "Giáo viên" : "Học sinh"}
                </button>
              ))}
            </div>
          )}
          <DateRangePicker value={days} onChange={(d: RangeDays) => setDays(d)} />
        </div>
      </header>
      {body}
      <ClassAttention teacher={mode === "teacher" && isTeacher} onOpenClass={onOpenClass} onOpenSet={onOpenSet} />
    </div>
  );
}
