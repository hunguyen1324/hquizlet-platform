// Dashboard — Dev 3 [P2-WEB-03]
// Fix P0-03: dùng StudySetListResult.items, search/sort qua backend query params.
// Loading skeleton, empty, error states đầy đủ.

import React, { useEffect, useState, useCallback } from "react";
import type { StudySet, HealthStatus } from "../../types";
import { useAuth } from "../auth/AuthContext";
import { studySetApi } from "../../lib/api";

type SortKey = "updated" | "created" | "title";

type Props = {
  healthStatus: HealthStatus;
  onOpen: (id: number) => void;
  onCreate: () => void;
};

export function Dashboard({ healthStatus, onOpen, onCreate }: Props) {
  const { token } = useAuth();
  const [sets, setSets] = useState<StudySet[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [query, setQuery] = useState("");
  const [sort, setSort] = useState<SortKey>("updated");

  const loadSets = useCallback(
    async (search: string, sortBy: SortKey) => {
      setLoading(true);
      setError("");
      try {
        // P0-03: backend trả paginated {items, total, ...} — dùng .items
        const result = await studySetApi.list(token, { search, sortBy });
        setSets(result.items ?? []);
        setTotal(result.total ?? 0);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Không tải được study sets.");
        setSets([]);
      } finally {
        setLoading(false);
      }
    },
    [token]
  );

  // Debounce search 300ms
  useEffect(() => {
    const id = setTimeout(() => void loadSets(query, sort), 300);
    return () => clearTimeout(id);
  }, [query, sort, loadSets]);

  return (
    <>
      <section className="page-heading">
        <div>
          <p className="eyebrow">Dashboard</p>
          <h1>Thư viện học phần</h1>
          <p>Quản lý bộ thẻ và tiếp tục học từ dữ liệu PostgreSQL.</p>
        </div>
        <button className="primary-button" onClick={onCreate}>
          Tạo học phần
        </button>
      </section>

      <section className="summary-grid">
        <div className="metric-card">
          <span>Study sets</span>
          <strong>{total}</strong>
        </div>
        <div className="metric-card">
          <span>Backend</span>
          <strong>{healthStatus}</strong>
        </div>
        <div className="metric-card">
          <span>Tổng thẻ</span>
          <strong>{sets.reduce((n, s) => n + (s.flashcards?.length ?? 0), 0)}</strong>
        </div>
      </section>

      {/* Search + sort bar */}
      <div className="list-controls">
        <input
          className="search-input"
          type="search"
          placeholder="Tìm học phần..."
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          aria-label="Tìm kiếm học phần"
        />
        <select
          className="sort-select"
          value={sort}
          onChange={(e) => setSort(e.target.value as SortKey)}
          aria-label="Sắp xếp"
        >
          <option value="updated">Mới cập nhật</option>
          <option value="created">Mới tạo</option>
          <option value="title">Tên A→Z</option>
        </select>
        <button
          className="ghost-button"
          onClick={() => void loadSets(query, sort)}
          disabled={loading}
          aria-label="Tải lại"
        >
          ↻
        </button>
      </div>

      {error && (
        <p className="message message--error">
          {error}{" "}
          <button className="ghost-button" onClick={() => void loadSets(query, sort)}>
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

        {!loading && sets.length === 0 && !error && query === "" && (
          <div className="empty-panel">
            <h2>Chưa có học phần</h2>
            <p>Tạo bộ thẻ đầu tiên với thuật ngữ và định nghĩa.</p>
            <button className="primary-button" onClick={onCreate}>
              Tạo học phần
            </button>
          </div>
        )}

        {!loading && sets.length === 0 && !error && query !== "" && (
          <div className="empty-panel">
            <h2>Không tìm thấy</h2>
            <p>Không có học phần nào khớp với "{query}".</p>
            <button className="ghost-button" onClick={() => setQuery("")}>
              Xóa tìm kiếm
            </button>
          </div>
        )}

        {sets.map((set) => (
          <button className="set-card" key={set.id} onClick={() => onOpen(set.id)}>
            <span>{set.description || "Chưa có mô tả"}</span>
            <strong>{set.title}</strong>
            <small>{set.flashcards?.length ?? 0} thẻ · Mở học phần</small>
          </button>
        ))}
      </section>
    </>
  );
}
