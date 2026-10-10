import React from "react";
import { useAuth } from "../auth/AuthContext";
import { protectedQuizApi, type QuizSessionMeta } from "../../lib/api/protectedQuiz";
import { ApiError } from "../../lib/api/client";

// ── helpers ──────────────────────────────────────────────────────────────────
function relativeTime(iso: string): string {
  const diff = Date.now() - Date.parse(iso);
  const m = Math.floor(diff / 60000);
  if (m < 1) return "Vừa xong";
  if (m < 60) return `${m} phút trước`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h} giờ trước`;
  const d = Math.floor(h / 24);
  return `${d} ngày trước`;
}
function timeUntil(iso: string): string {
  const diff = Date.parse(iso) - Date.now();
  if (diff <= 0) return "Đã hết hạn";
  const m = Math.floor(diff / 60000);
  if (m < 60) return `Còn ${m} phút`;
  const h = Math.floor(m / 60);
  if (h < 24) return `Còn ${h} giờ`;
  return `Còn ${Math.floor(h / 24)} ngày`;
}
function modeLabel(m: QuizSessionMeta["mode"]) {
  return m === "exam" ? "Thi thử" : "Luyện tập";
}
function statusBadge(s: QuizSessionMeta): { label: string; color: string } {
  if (s.submittedAt) return { label: "Đã nộp", color: "#10b981" };
  if (Date.parse(s.activeUntil) <= Date.now()) return { label: "Hết hạn", color: "#94a3b8" };
  return { label: "Đang làm", color: "#6366f1" };
}

// ── component ────────────────────────────────────────────────────────────────
type Props = {
  /** When provided, list is pre-filtered to this quiz. */
  studySetId?: number;
  onResume?: (studySetId: number, sessionId: string) => void;
  onBack?: () => void;
};

type State = "idle" | "loading" | "error";

export function QuizSessionsPage({ studySetId, onResume, onBack }: Props) {
  const { token } = useAuth();
  const [state, setState] = React.useState<State>("loading");
  const [items, setItems] = React.useState<QuizSessionMeta[]>([]);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const perPage = 20;
  const [error, setError] = React.useState("");
  const [deletingId, setDeletingId] = React.useState<string | null>(null);
  const [confirmDelete, setConfirmDelete] = React.useState<QuizSessionMeta | null>(null);
  const [filterSetId, setFilterSetId] = React.useState(studySetId ?? 0);
  const [filterInput, setFilterInput] = React.useState(studySetId ? String(studySetId) : "");

  const load = React.useCallback(async (p: number, setId: number) => {
    setState("loading");
    setError("");
    try {
      const res = await protectedQuizApi.listSessions(token, {
        studySetId: setId > 0 ? setId : undefined,
        page: p,
        perPage,
      });
      setItems(res.items);
      setTotal(res.total);
      setPage(p);
      setState("idle");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Không tải được danh sách phiên.");
      setState("error");
    }
  }, [token]);

  React.useEffect(() => { void load(1, filterSetId); }, [load, filterSetId]);

  async function handleDelete(s: QuizSessionMeta) {
    setDeletingId(s.id);
    setConfirmDelete(null);
    try {
      await protectedQuizApi.deleteSession(token, s.id);
      setItems((prev) => prev.filter((x) => x.id !== s.id));
      setTotal((n) => Math.max(0, n - 1));
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) {
        // Already deleted – idempotent
        setItems((prev) => prev.filter((x) => x.id !== s.id));
      } else {
        setError(e instanceof Error ? e.message : "Xóa phiên thất bại.");
      }
    } finally {
      setDeletingId(null);
    }
  }

  function applyFilter() {
    const id = parseInt(filterInput, 10);
    setFilterSetId(isNaN(id) || id <= 0 ? 0 : id);
  }

  const totalPages = Math.max(1, Math.ceil(total / perPage));

  return (
    <div style={{ minHeight: "100vh", background: "linear-gradient(135deg,#0f0c29,#302b63,#24243e)", padding: "24px 16px 64px", color: "#e2e8f0" }}>
      {/* Header */}
      <div style={{ maxWidth: 900, margin: "0 auto" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 32 }}>
          {onBack && (
            <button
              onClick={onBack}
              style={{ background: "rgba(255,255,255,.08)", border: "none", color: "#a5b4fc", borderRadius: 8, padding: "8px 14px", cursor: "pointer", fontWeight: 600, fontSize: "0.9rem", transition: "background .15s" }}
              onMouseEnter={(e) => (e.currentTarget.style.background = "rgba(255,255,255,.14)")}
              onMouseLeave={(e) => (e.currentTarget.style.background = "rgba(255,255,255,.08)")}
            >
              ← Quay lại
            </button>
          )}
          <div>
            <h1 style={{ margin: 0, fontSize: "1.75rem", fontWeight: 800, background: "linear-gradient(90deg,#a5b4fc,#818cf8)", WebkitBackgroundClip: "text", WebkitTextFillColor: "transparent" }}>
              Phiên quiz của tôi
            </h1>
            <p style={{ margin: "4px 0 0", color: "#94a3b8", fontSize: "0.88rem" }}>
              {total > 0 ? `${total} phiên` : "Quản lý tiến độ làm bài của bạn"}
            </p>
          </div>
        </div>

        {/* Filter bar */}
        <div style={{ display: "flex", gap: 10, marginBottom: 24, flexWrap: "wrap" }}>
          <input
            type="number"
            value={filterInput}
            onChange={(e) => setFilterInput(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && applyFilter()}
            placeholder="Lọc theo ID quiz (hoặc để trống)"
            style={{ flex: 1, minWidth: 200, background: "rgba(255,255,255,.06)", border: "1.5px solid rgba(165,180,252,.25)", borderRadius: 10, padding: "10px 14px", color: "#e2e8f0", fontSize: "0.9rem", outline: "none" }}
          />
          <button
            onClick={applyFilter}
            style={{ background: "linear-gradient(135deg,#6366f1,#8b5cf6)", border: "none", color: "#fff", borderRadius: 10, padding: "10px 22px", cursor: "pointer", fontWeight: 700, fontSize: "0.9rem" }}
          >
            Lọc
          </button>
          {filterSetId > 0 && (
            <button
              onClick={() => { setFilterInput(""); setFilterSetId(0); }}
              style={{ background: "rgba(255,255,255,.08)", border: "none", color: "#94a3b8", borderRadius: 10, padding: "10px 18px", cursor: "pointer", fontWeight: 600 }}
            >
              ✕ Xóa lọc
            </button>
          )}
        </div>

        {/* Error */}
        {error && (
          <div role="alert" style={{ background: "rgba(239,68,68,.12)", border: "1px solid rgba(239,68,68,.3)", borderRadius: 10, padding: "12px 16px", color: "#fca5a5", marginBottom: 20, fontSize: "0.9rem" }}>
            {error}
          </div>
        )}

        {/* Loading skeleton */}
        {state === "loading" && (
          <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
            {[1, 2, 3].map((i) => (
              <div key={i} style={{ background: "rgba(255,255,255,.04)", borderRadius: 14, height: 100, animation: "pulse 1.4s ease-in-out infinite" }} />
            ))}
          </div>
        )}

        {/* Empty state */}
        {state === "idle" && items.length === 0 && (
          <div style={{ textAlign: "center", padding: "80px 20px", color: "#64748b" }}>
            <div style={{ fontSize: "3rem", marginBottom: 16 }}>📋</div>
            <h2 style={{ color: "#94a3b8", fontWeight: 700, margin: "0 0 8px" }}>Chưa có phiên nào</h2>
            <p style={{ fontSize: "0.9rem" }}>
              {filterSetId > 0 ? `Quiz #${filterSetId} chưa có phiên nào đang hoạt động.` : "Bắt đầu một bài quiz để phiên của bạn xuất hiện ở đây."}
            </p>
          </div>
        )}

        {/* Session cards */}
        {state === "idle" && items.length > 0 && (
          <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
            {items.map((s) => {
              const badge = statusBadge(s);
              const isDeleting = deletingId === s.id;
              const progress = s.total > 0 ? Math.round((s.answered / s.total) * 100) : 0;
              const isActive = !s.submittedAt && Date.parse(s.activeUntil) > Date.now();

              return (
                <div
                  key={s.id}
                  style={{ background: "rgba(255,255,255,.05)", backdropFilter: "blur(12px)", borderRadius: 16, border: "1px solid rgba(255,255,255,.09)", padding: "18px 22px", transition: "border-color .2s, transform .15s", cursor: "default" }}
                  onMouseEnter={(e) => { e.currentTarget.style.borderColor = "rgba(165,180,252,.35)"; e.currentTarget.style.transform = "translateY(-1px)"; }}
                  onMouseLeave={(e) => { e.currentTarget.style.borderColor = "rgba(255,255,255,.09)"; e.currentTarget.style.transform = "translateY(0)"; }}
                >
                  <div style={{ display: "flex", alignItems: "flex-start", gap: 14, flexWrap: "wrap" }}>
                    {/* Left: info */}
                    <div style={{ flex: 1, minWidth: 200 }}>
                      <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 8, flexWrap: "wrap" }}>
                        <span style={{ background: badge.color + "22", color: badge.color, borderRadius: 6, padding: "2px 10px", fontSize: "0.78rem", fontWeight: 700, letterSpacing: "0.03em" }}>
                          {badge.label}
                        </span>
                        <span style={{ background: "rgba(255,255,255,.07)", color: "#94a3b8", borderRadius: 6, padding: "2px 10px", fontSize: "0.78rem", fontWeight: 600 }}>
                          {modeLabel(s.mode)}
                        </span>
                        <span style={{ background: "rgba(255,255,255,.07)", color: "#94a3b8", borderRadius: 6, padding: "2px 10px", fontSize: "0.78rem", fontWeight: 600 }}>
                          {s.layout === "toeic" ? "TOEIC" : "Mặc định"}
                        </span>
                      </div>
                      <p style={{ margin: "0 0 6px", fontSize: "0.82rem", color: "#64748b" }}>
                        Quiz #{s.studySetId} · Cập nhật {relativeTime(s.updatedAt)}
                      </p>
                      {/* Progress bar */}
                      <div style={{ marginTop: 8 }}>
                        <div style={{ display: "flex", justifyContent: "space-between", marginBottom: 4 }}>
                          <span style={{ fontSize: "0.8rem", color: "#94a3b8" }}>{s.answered}/{s.total} câu đã trả lời</span>
                          <span style={{ fontSize: "0.8rem", color: "#a5b4fc", fontWeight: 700 }}>{progress}%</span>
                        </div>
                        <div style={{ height: 6, background: "rgba(255,255,255,.08)", borderRadius: 999 }}>
                          <div style={{ height: "100%", width: `${progress}%`, background: "linear-gradient(90deg,#6366f1,#8b5cf6)", borderRadius: 999, transition: "width .4s ease" }} />
                        </div>
                      </div>
                      {s.deadline && (
                        <p style={{ margin: "8px 0 0", fontSize: "0.78rem", color: Date.parse(s.deadline) < Date.now() ? "#f87171" : "#fbbf24" }}>
                          ⏱ {timeUntil(s.deadline)}
                        </p>
                      )}
                      {!s.deadline && isActive && (
                        <p style={{ margin: "8px 0 0", fontSize: "0.78rem", color: "#94a3b8" }}>
                          Hạn: {timeUntil(s.expiresAt)}
                        </p>
                      )}
                    </div>

                    {/* Right: actions */}
                    <div style={{ display: "flex", gap: 8, alignSelf: "center", flexShrink: 0 }}>
                      {isActive && onResume && (
                        <button
                          onClick={() => onResume(s.studySetId, s.id)}
                          style={{ background: "linear-gradient(135deg,#6366f1,#8b5cf6)", border: "none", color: "#fff", borderRadius: 10, padding: "9px 20px", cursor: "pointer", fontWeight: 700, fontSize: "0.88rem", whiteSpace: "nowrap" }}
                        >
                          Tiếp tục
                        </button>
                      )}
                      {!s.submittedAt && (
                        <button
                          onClick={() => setConfirmDelete(s)}
                          disabled={isDeleting}
                          style={{ background: "rgba(239,68,68,.1)", border: "1px solid rgba(239,68,68,.25)", color: "#fca5a5", borderRadius: 10, padding: "9px 16px", cursor: "pointer", fontWeight: 600, fontSize: "0.88rem", transition: "background .15s" }}
                          onMouseEnter={(e) => (e.currentTarget.style.background = "rgba(239,68,68,.2)")}
                          onMouseLeave={(e) => (e.currentTarget.style.background = "rgba(239,68,68,.1)")}
                        >
                          {isDeleting ? "Đang xóa…" : "Xóa"}
                        </button>
                      )}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}

        {/* Pagination */}
        {state === "idle" && totalPages > 1 && (
          <div style={{ display: "flex", justifyContent: "center", gap: 8, marginTop: 32 }}>
            <button
              disabled={page <= 1}
              onClick={() => void load(page - 1, filterSetId)}
              style={{ background: "rgba(255,255,255,.08)", border: "none", color: "#a5b4fc", borderRadius: 8, padding: "8px 18px", cursor: page <= 1 ? "not-allowed" : "pointer", opacity: page <= 1 ? 0.4 : 1, fontWeight: 600 }}
            >
              ← Trước
            </button>
            <span style={{ color: "#94a3b8", padding: "8px 12px", fontSize: "0.9rem" }}>
              {page} / {totalPages}
            </span>
            <button
              disabled={page >= totalPages}
              onClick={() => void load(page + 1, filterSetId)}
              style={{ background: "rgba(255,255,255,.08)", border: "none", color: "#a5b4fc", borderRadius: 8, padding: "8px 18px", cursor: page >= totalPages ? "not-allowed" : "pointer", opacity: page >= totalPages ? 0.4 : 1, fontWeight: 600 }}
            >
              Sau →
            </button>
          </div>
        )}
      </div>

      {/* Delete confirmation dialog */}
      {confirmDelete && (
        <div
          role="dialog"
          aria-modal="true"
          aria-label="Xác nhận xóa phiên"
          style={{ position: "fixed", inset: 0, zIndex: 500, display: "flex", alignItems: "center", justifyContent: "center", background: "rgba(0,0,0,.65)", padding: 16 }}
        >
          <div style={{ background: "#1e1b4b", border: "1px solid rgba(165,180,252,.2)", borderRadius: 18, padding: 32, maxWidth: 440, width: "100%", boxShadow: "0 25px 60px rgba(0,0,0,.5)" }}>
            <h3 style={{ margin: "0 0 12px", fontSize: "1.2rem", fontWeight: 800, color: "#e2e8f0" }}>Xóa phiên làm bài?</h3>
            <p style={{ margin: "0 0 24px", color: "#94a3b8", lineHeight: 1.6, fontSize: "0.9rem" }}>
              Tiến độ và đáp án chưa nộp của phiên này sẽ <strong style={{ color: "#fca5a5" }}>bị mất vĩnh viễn</strong>. Hành động này không thể hoàn tác.
            </p>
            <div style={{ display: "flex", gap: 12, justifyContent: "flex-end" }}>
              <button
                onClick={() => setConfirmDelete(null)}
                style={{ background: "rgba(255,255,255,.08)", border: "none", color: "#e2e8f0", borderRadius: 10, padding: "10px 20px", cursor: "pointer", fontWeight: 600 }}
              >
                Hủy
              </button>
              <button
                onClick={() => void handleDelete(confirmDelete)}
                style={{ background: "linear-gradient(135deg,#dc2626,#b91c1c)", border: "none", color: "#fff", borderRadius: 10, padding: "10px 20px", cursor: "pointer", fontWeight: 700 }}
              >
                Xác nhận xóa
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
