// ProgressPanel.tsx — Dev 3
// Phase 3: reusable progress summary/history presentation.
// Deliberately API-agnostic: no endpoint, response shape, or mock data is hard-coded here.

export type ProgressPanelStatus = "idle" | "loading" | "success" | "empty" | "error";

export type ProgressSummaryView = {
  latestScore: number | null;
  latestTotal: number | null;
  attemptCount: number;
  latestMode: string | null;
};

export type ProgressHistoryItem = {
  id: number | string;
  mode: string;
  score: number;
  total: number;
  completedAt: string | null;
};

type Props = {
  status: ProgressPanelStatus;
  summary?: ProgressSummaryView | null;
  history?: ProgressHistoryItem[];
  errorMessage?: string;
  onRetry?: () => void;
};

function formatScore(summary: ProgressSummaryView) {
  if (summary.latestScore === null || summary.latestTotal === null) return "—";
  return `${summary.latestScore}/${summary.latestTotal}`;
}

function formatDate(value: string | null) {
  if (!value) return "Chưa hoàn thành";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("vi-VN", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}

export function ProgressPanel({
  status,
  summary = null,
  history = [],
  errorMessage = "Không tải được tiến độ học.",
  onRetry,
}: Props) {
  return (
    <section className="panel progress-panel" aria-labelledby="progress-panel-title">
      <div className="page-heading progress-panel__heading">
        <div>
          <p className="eyebrow">Learning Progress</p>
          <h2 id="progress-panel-title">Tiến độ học</h2>
        </div>
      </div>

      {status === "loading" && (
        <div className="loading-skeleton" aria-busy="true" aria-live="polite">
          {[1, 2, 3].map((item) => (
            <div key={item} className="skeleton-row" />
          ))}
        </div>
      )}

      {status === "error" && (
        <div className="message message--error" role="alert">
          <span>{errorMessage}</span>
          {onRetry && (
            <button className="ghost-button" type="button" onClick={onRetry}>
              Thử lại
            </button>
          )}
        </div>
      )}

      {status === "idle" && (
        <div className="empty-panel" aria-live="polite">
          <h3>Tiến độ sẽ hiển thị tại đây</h3>
          <p>Chưa có yêu cầu tải dữ liệu tiến độ.</p>
        </div>
      )}

      {status === "empty" && (
        <div className="empty-panel" aria-live="polite">
          <h3>Chưa có lịch sử học</h3>
          <p>Hoàn thành một lượt học để bắt đầu lưu tiến độ.</p>
        </div>
      )}

      {status === "success" && summary && (
        <>
          <div className="summary-grid">
            <div className="metric-card">
              <span>Điểm gần nhất</span>
              <strong>{formatScore(summary)}</strong>
            </div>
            <div className="metric-card">
              <span>Số lần học</span>
              <strong>{summary.attemptCount}</strong>
            </div>
            <div className="metric-card">
              <span>Mode gần nhất</span>
              <strong>{summary.latestMode ?? "—"}</strong>
            </div>
          </div>

          <div className="progress-history" aria-label="Lịch sử học">
            <div className="progress-history__header">
              <h3>Lịch sử</h3>
            </div>
            {history.length === 0 ? (
              <p className="empty">Không có lượt học nào trong lịch sử.</p>
            ) : (
              <div className="progress-history__list">
                {history.map((item) => (
                  <article className="mini-card" key={item.id}>
                    <div>
                      <strong>{item.mode}</strong>
                      <span>
                        {item.score}/{item.total} · {formatDate(item.completedAt)}
                      </span>
                    </div>
                  </article>
                ))}
              </div>
            )}
          </div>
        </>
      )}

      {status === "success" && !summary && (
        <div className="empty-panel" aria-live="polite">
          <h3>Chưa có dữ liệu tóm tắt</h3>
          <p>API chưa trả về summary cho lượt học này.</p>
        </div>
      )}
    </section>
  );
}
