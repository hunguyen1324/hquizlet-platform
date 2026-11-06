// Dashboard — Dev 3 [P2-WEB-03]
// Study set list: server-side search/sort + paginated API response.
// Debounced search + AbortController: an in-flight request from a stale
// query/sort never overwrites the result of a newer one.

import React, { useEffect, useRef, useState } from "react";
import type { StudySet, HealthStatus } from "../../types";
import { useAuth } from "../auth/AuthContext";
import { studySetApi } from "../../lib/api";

type SortKey = "updated" | "title";

type Props = {
  healthStatus: HealthStatus;
  onOpen: (id: number) => void;
  onCreate: () => void;
};

export function Dashboard({ healthStatus, onOpen, onCreate }: Props) {
  const { token } = useAuth();
  const [sets, setSets] = useState<StudySet[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [query, setQuery] = useState("");
  const [sort, setSort] = useState<SortKey>("updated");
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);

  const abortRef = useRef<AbortController | null>(null);

  const loadSets = async (signal?: AbortSignal) => {
    setLoading(true);
    setError("");
    try {
      const result = await studySetApi.list(
        token,
        { search: query.trim() || undefined, sort, page, per_page: 24 },
        signal
      );
      setSets(result.items ?? []);
      setTotal(result.total ?? 0);
      setTotalPages(result.totalPages ?? 1);
    } catch (err) {
      if (err instanceof DOMException && err.name === "AbortError") return;
      setError(err instanceof Error ? err.message : "Không tải được study sets.");
      setSets([]);
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  };

  useEffect(() => {
    setLoading(true);
    // Debounce 300ms, hủy request cũ khi query/sort đổi trước khi nó kịp trả về
    const timer = setTimeout(() => {
      abortRef.current?.abort();
      const controller = new AbortController();
      abortRef.current = controller;
      void loadSets(controller.signal);
    }, 300);

    return () => {
      clearTimeout(timer);
      abortRef.current?.abort();
    };
  }, [token, query, sort, page]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <>
      <section className="page-heading">
        <div>
          <p className="eyebrow">KHÔNG GIAN KIẾN THỨC</p>
          <h1>Thư viện học phần</h1>
          <p>Tất cả học phần của bạn, sẵn sàng cho lần ôn tập tiếp theo.</p>
        </div>
        <button className="primary-button" onClick={onCreate}>
          Tạo học phần
        </button>
      </section>

      <section className="summary-grid">
        <div className="metric-card">
          <span>Học phần trong thư viện</span>
          <strong>{total}</strong>
        </div>
        <div className="metric-card">
          <span>Học phần trên trang này</span>
          <strong>{sets.length}</strong>
        </div>
        <div className="metric-card">
          <span>Thẻ trong danh sách hiện tại</span>
          <strong>{sets.reduce((n, s) => n + (s.flashcardCount ?? s.flashcards?.length ?? 0), 0)}</strong>
        </div>
      </section>

      <div className="list-controls">
        <input
          className="search-input"
          type="search"
          placeholder="Tìm học phần..."
          value={query}
          onChange={(e) => { setQuery(e.target.value); setPage(1); }}
          aria-label="Tìm kiếm học phần"
        />
        <select
          className="sort-select"
          value={sort}
          onChange={(e) => { setSort(e.target.value as SortKey); setPage(1); }}
          aria-label="Sắp xếp"
        >
          <option value="updated">Mới nhất</option>
          <option value="title">Tên A→Z</option>
        </select>
        <button
          className="ghost-button"
          onClick={() => void loadSets()}
          disabled={loading}
          aria-label="Tải lại"
        >
          ↻
        </button>
      </div>

      {error && (
        <p className="message message--error">
          {error}{" "}
          <button className="ghost-button" onClick={() => void loadSets()}>
            Thử lại
          </button>
        </p>
      )}

      <section className="set-grid">
        {loading && (
          <div className="loading-skeleton" aria-busy="true">
            {[1, 2, 3].map((i) => (
              <div key={i} className="skeleton-row" />
            ))}
          </div>
        )}

        {!loading && sets.length === 0 && !error && (
          <div className="empty-panel">
            <h2>{query ? "Không tìm thấy" : "Chưa có học phần"}</h2>
            <p>
              {query
                ? `Không có học phần nào khớp với "${query}".`
                : "Tạo bộ thẻ đầu tiên với thuật ngữ và định nghĩa."}
            </p>
            {query ? (
              <button className="ghost-button" onClick={() => setQuery("")}>
                Xóa tìm kiếm
              </button>
            ) : (
              <button className="primary-button" onClick={onCreate}>
                Tạo học phần
              </button>
            )}
          </div>
        )}

        {!loading && !error && sets.map((set) => (
          <button className="set-card" key={set.id} onClick={() => onOpen(set.id)}>
            <span>{set.description || "Chưa có mô tả"}</span>
            <strong>
              {set.visibility === "private" && <span title="Riêng tư">🔒 </span>}
              {set.title}
            </strong>
            <small>
              {set.contentType === "quiz" ? "Quiz" : set.contentType === "grammar" ? "Ngữ pháp" : "Thẻ học"}
              {' · '}{set.flashcardCount ?? set.flashcards?.length ?? 0} thẻ · Mở học phần →
            </small>
          </button>
        ))}
      </section>
      {totalPages > 1 && <nav className="library-pagination" aria-label="Phân trang học phần"><button className="ghost-button" disabled={page <= 1 || loading} onClick={() => setPage(p => p - 1)}>← Trang trước</button><span aria-live="polite">Trang {page} / {totalPages}</span><button className="ghost-button" disabled={page >= totalPages || loading} onClick={() => setPage(p => p + 1)}>Trang sau →</button></nav>}
    </>
  );
}
