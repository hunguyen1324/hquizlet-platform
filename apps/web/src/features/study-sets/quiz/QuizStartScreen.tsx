import React from "react";
import type { QuizSessionMeta } from "../../../lib/api/protectedQuiz";
import { PART_LABELS } from "../exam/toeic";
import type { QuizSettings, QuizSummary } from "./useProtectedQuiz";
import { formatClock, timeAgo } from "./quizHelpers";

export function QuizStartScreen({ summary, recent, savedId, settings, setSettings, busy, error, sessionLimitError, onStart, onOpen, onManageSessions, studySetId }: {
  summary: QuizSummary | null;
  recent: QuizSessionMeta[];
  savedId: string | null;
  settings: QuizSettings;
  setSettings: React.Dispatch<React.SetStateAction<QuizSettings>>;
  busy: boolean;
  error: string;
  sessionLimitError: boolean;
  onStart: (mode: "practice" | "exam") => void;
  onOpen: (id: string) => void;
  onManageSessions?: (studySetId: number) => void;
  studySetId: number;
}) {
  const set = <K extends keyof QuizSettings>(key: K, value: QuizSettings[K]) => setSettings((s) => ({ ...s, [key]: value }));
  const now = Date.now();
  const resumable = recent.filter((s) => !s.submittedAt && Date.parse(s.activeUntil) > now).slice(0, 3);
  const finished = recent.filter((s) => s.submittedAt).slice(0, 1);
  // If the detailed list could not be loaded, still offer the ids the summary knows about.
  const fallbackIds = recent.length === 0 ? (summary?.sessions ?? []).slice(0, 3) : [];
  const ready = !!summary?.total && !busy;

  return <section className="qz qz-start" aria-busy={!summary}>
    <div>
      <h2>Bắt đầu làm bài</h2>
      <p className="qz-start-sub">{summary ? `${summary.total} câu hỏi` : "Đang tải thông tin bài…"}</p>
    </div>
    {error && <p role="alert" className="qz-error">{error}</p>}
    {sessionLimitError && <p role="alert" className="qz-error">Bạn đang có {summary?.limit ?? 5} phiên chưa nộp cho bài này. Hãy nộp hoặc xoá bớt một phiên rồi thử lại.{onManageSessions && <button onClick={() => onManageSessions(studySetId)}>Quản lý phiên</button>}</p>}

    {(resumable.length > 0 || fallbackIds.length > 0 || finished.length > 0) && <div className="qz-resume-list" aria-label="Phiên gần đây">
      {resumable.map((s) => {
        const left = s.deadline ? Math.max(0, Math.floor((Date.parse(s.deadline) - now) / 1000)) : null;
        return <button key={s.id} className="qz-resume" disabled={busy} onClick={() => onOpen(s.id)}>
          <span className="qz-resume-main">
            <strong>Làm tiếp {s.mode === "exam" ? "thi thử" : "luyện tập"} · {s.answered}/{s.total} câu</strong>
            <small>Lần cuối {timeAgo(s.updatedAt, now)}{left !== null ? ` · còn ${formatClock(left)}` : ""}</small>
            <span className="qz-resume-bar" aria-hidden="true"><span style={{ width: `${s.total ? (s.answered / s.total) * 100 : 0}%` }} /></span>
          </span>
          <b>Tiếp tục</b>
        </button>;
      })}
      {fallbackIds.map((id, i) => <button key={id} className="qz-resume" disabled={busy} onClick={() => onOpen(id)}><span className="qz-resume-main"><strong>Mở phiên trước #{i + 1}</strong></span><b>Mở</b></button>)}
      {finished.map((s) => <button key={s.id} className="qz-resume" disabled={busy} onClick={() => onOpen(s.id)} style={{ background: "transparent", borderColor: "var(--border)" }}>
        <span className="qz-resume-main"><strong>Xem kết quả lần trước</strong><small>{s.answered}/{s.total} câu đã trả lời · {timeAgo(s.updatedAt, now)}</small></span><b>Xem</b>
      </button>)}
      {savedId && recent.length === 0 && fallbackIds.length === 0 && <button className="qz-resume" disabled={busy} onClick={() => onOpen(savedId)}><span className="qz-resume-main"><strong>Mở phiên gần nhất</strong></span><b>Mở</b></button>}
    </div>}

    <div className="qz-modes">
      <button className="qz-mode" disabled={!ready} onClick={() => onStart("practice")}>
        <strong>Luyện tập</strong>
        <span>Xem đúng/sai và giải thích ngay sau mỗi câu. Không bị tính giờ.</span>
        <b>Bắt đầu luyện tập</b>
      </button>
      <button className="qz-mode" disabled={!ready} onClick={() => onStart("exam")}>
        <strong>Thi thử</strong>
        <span>Tính giờ như thi thật, chỉ xem kết quả sau khi nộp bài. Hết giờ tự nộp.</span>
        <b>Vào phòng thi</b>
      </button>
    </div>

    <details className="qz-options-box">
      <summary>Tuỳ chọn</summary>
      <div className="qz-options-grid">
        <div className="qz-field" role="group" aria-label="Giao diện làm bài">
          Giao diện
          <div className="qz-segment">
            <button type="button" aria-pressed={settings.layout === "default"} onClick={() => set("layout", "default")}>Thường</button>
            <button type="button" aria-pressed={settings.layout === "toeic"} onClick={() => set("layout", "toeic")}>TOEIC (IIG)</button>
          </div>
        </div>
        {settings.layout === "toeic" && <label className="qz-field">Chọn Part
          <select value={settings.part} onChange={(e) => set("part", Number(e.target.value))}>
            <option value={0}>Tất cả các Part</option>
            {Object.entries(summary?.parts ?? {}).map(([p, count]) => <option key={p} value={p}>{PART_LABELS[Number(p)]} ({count} câu)</option>)}
          </select>
        </label>}
        <label className="qz-field">Khi luyện tập, xem đáp án
          <select value={settings.instant ? "instant" : "end"} onChange={(e) => set("instant", e.target.value === "instant")}>
            <option value="instant">Ngay sau mỗi câu</option>
            <option value="end">Sau khi nộp bài</option>
          </select>
        </label>
        <label className="qz-field">Thời gian làm bài (phút)
          <input type="number" min={1} max={600} value={settings.minutes} onChange={(e) => set("minutes", Math.max(1, Math.min(600, Number(e.target.value) || 1)))} />
        </label>
        <label className="qz-check"><input type="checkbox" checked={settings.timedPractice} onChange={(e) => set("timedPractice", e.target.checked)} /> Cũng đếm ngược khi luyện tập</label>
      </div>
    </details>
    <p className="qz-muted">Tiến độ được lưu trong 24 giờ. Tối đa {summary?.limit ?? 5} phiên chưa nộp cho mỗi bài.{onManageSessions && <> <button style={{ padding: 0, background: "none", color: "var(--primary)", textDecoration: "underline", border: 0, cursor: "pointer", font: "inherit" }} onClick={() => onManageSessions(studySetId)}>Quản lý các phiên</button></>}</p>
  </section>;
}
