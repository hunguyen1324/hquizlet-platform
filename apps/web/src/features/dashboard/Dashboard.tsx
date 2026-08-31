// Dashboard — Dev 3 [P2-WEB-03]
// Fix P0-03: dùng StudySetListResult.items, search/sort qua backend query params.
// Fix sort_by → sort (đúng param tên backend).
// Fix race condition: dùng AbortController — response của query cũ không ghi đè query mới.
// Loading skeleton, empty, error states đầy đủ.

import React, { useEffect, useState, useRef } from "react";
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

  // AbortController ref — hủy request cũ khi query/sort thay đổi
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    // Debounce 300ms
    const timer = setTimeout(() => {
      // Hủy request đang chạy nếu có
      abortRef.current?.abort();
      const controller = new AbortController();
      abortRef.current = controller;

      setLoading(true);
      setError("");

      studySetApi
        .list(token, { search: query, sortBy: sort }, controller.signal)
        .then((result) => {
          setSets(result.items ?? []);
          setTotal(result.total ?? 0);
        })
        .catch((err) => {
          // Bỏ qua lỗi abort — không phải lỗi thật
          if (err instanceof DOMException && err.name === "AbortError") return;
          setError(err instanceof Error ? err.message : "Không tải được study sets.");
          setSets([]);
        })
        .finally(() => setLoading(false));
    }, 300);

    return () => {
      clearTimeout(timer);
      abortRef.current?.abort();
    };
  }, [query, sort, token]);

  function reload() {
    // Trigger re-run bằng cách force effect — đổi state phụ không ảnh hưởng UI
    setError("");
    // Workaround: toggle một lần để effect chạy lại với cùng query/sort
    setSort((s) => s);
  }

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
          onClick={reload}
          disabled={loading}
          aria-label="Tải lại"
        >
          ↻
        </button>
      </div>

      {error && (
        <p className="message message--error">
          {error}{" "}
          <button className="ghost-button" onClick={reload}>
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
