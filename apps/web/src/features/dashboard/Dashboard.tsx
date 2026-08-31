// Dashboard — Dev 3 [P2-WEB-03]
// Study set list: search, sort, loading/error/empty states. API thật qua studySetApi.

import React, { useEffect, useState, useMemo } from "react";
import type { StudySet, HealthStatus } from "../../types";
import { useAuth } from "../auth/AuthContext";
import { studySetApi } from "../../lib/api";

type SortKey = "updatedAt" | "title" | "cards";

type Props = {
  healthStatus: HealthStatus;
  onOpen: (id: number) => void;
  onCreate: () => void;
};

function sortSets(sets: StudySet[], key: SortKey): StudySet[] {
  return [...sets].sort((a, b) => {
    if (key === "title") return a.title.localeCompare(b.title, "vi");
    if (key === "cards") return (b.flashcards?.length ?? 0) - (a.flashcards?.length ?? 0);
    // updatedAt — backend may not always include it; fall back to id desc
    return b.id - a.id;
  });
}

export function Dashboard({ healthStatus, onOpen, onCreate }: Props) {
  const { token } = useAuth();
  const [sets, setSets] = useState<StudySet[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [query, setQuery] = useState("");
  const [sort, setSort] = useState<SortKey>("updatedAt");

  const loadSets = async () => {
    setLoading(true);
    setError("");
    try {
      const data = await studySetApi.list(token);
      setSets(data ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Không tải được study sets.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadSets();
  }, [token]); // eslint-disable-line react-hooks/exhaustive-deps

  const displayed = useMemo(() => {
    const q = query.trim().toLowerCase();
    const filtered = q
      ? sets.filter(
          (s) =>
            s.title.toLowerCase().includes(q) ||
            (s.description ?? "").toLowerCase().includes(q)
        )
      : sets;
    return sortSets(filtered, sort);
  }, [sets, query, sort]);

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
          <strong>{sets.length}</strong>
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
          <option value="updatedAt">Mới nhất</option>
          <option value="title">Tên A→Z</option>
          <option value="cards">Nhiều thẻ nhất</option>
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
            <h2>Chưa có học phần</h2>
            <p>Tạo bộ thẻ đầu tiên với thuật ngữ và định nghĩa.</p>
            <button className="primary-button" onClick={onCreate}>
              Tạo học phần
            </button>
          </div>
        )}

        {!loading && sets.length > 0 && displayed.length === 0 && (
          <div className="empty-panel">
            <h2>Không tìm thấy</h2>
            <p>Không có học phần nào khớp với "{query}".</p>
            <button className="ghost-button" onClick={() => setQuery("")}>
              Xóa tìm kiếm
            </button>
          </div>
        )}

        {displayed.map((set) => (
          <button className="set-card" key={set.id} onClick={() => onOpen(set.id)}>
            <span>{set.description || "Chưa có mô tả"}</span>
            <strong>{set.title}</strong>
            <small>
              {set.flashcards?.length ?? 0} thẻ · Mở học phần
            </small>
          </button>
        ))}
      </section>
    </>
  );
}
