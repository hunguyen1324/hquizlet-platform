// Dashboard - Dev 3 (FE-CORE-05)
// Gọi API thật /v1/study-sets qua gateway. Đã bỏ MOCK_SETS (fix regression).

import React, { useEffect, useState } from "react";
import type { StudySet, HealthStatus } from "../../types";
import { useAuth, apiFetch } from "../auth/AuthContext";

type Props = {
  healthStatus: HealthStatus;
  onOpen: (id: number) => void;
  onCreate: () => void;
};

export function Dashboard({ healthStatus, onOpen, onCreate }: Props) {
  const { token } = useAuth();
  const [sets, setSets] = useState<StudySet[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const loadSets = async () => {
    setLoading(true);
    setError("");
    try {
      const data = await apiFetch<StudySet[]>("/v1/study-sets", token);
      setSets(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Không tải được study sets.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void loadSets(); }, [token]);

  return (
    <>
      <section className="page-heading">
        <div>
          <p className="eyebrow">Dashboard</p>
          <h1>Thư viện học phần</h1>
          <p>Quản lý các bộ thẻ và tiếp tục học từ dữ liệu PostgreSQL.</p>
        </div>
        <button className="primary-button" onClick={onCreate}>Tạo học phần</button>
      </section>

      <section className="summary-grid">
        <div className="metric-card"><span>Study sets</span><strong>{sets.length}</strong></div>
        <div className="metric-card"><span>Backend</span><strong>{healthStatus}</strong></div>
        <div className="metric-card"><span>Status</span><strong>{loading ? "loading" : "ready"}</strong></div>
      </section>

      {error && (
        <p className="message message--error">
          {error}{" "}
          <button className="ghost-button" onClick={() => void loadSets()}>Thử lại</button>
        </p>
      )}

      <section className="set-grid">
        {loading && <p className="loading-state">Đang tải...</p>}

        {!loading && sets.length === 0 && !error && (
          <div className="empty-panel">
            <h2>Chưa có học phần</h2>
            <p>Tạo bộ thẻ đầu tiên với thuật ngữ và định nghĩa.</p>
            <button className="primary-button" onClick={onCreate}>Tạo học phần</button>
          </div>
        )}

        {sets.map((set) => (
          <button className="set-card" key={set.id} onClick={() => onOpen(set.id)}>
            <span>{set.description || "No description"}</span>
            <strong>{set.title}</strong>
            <small>{set.flashcards?.length ?? 0} thẻ · Mở học phần</small>
          </button>
        ))}
      </section>

      <div style={{ marginTop: "1rem", textAlign: "right" }}>
        <button className="ghost-button" onClick={() => void loadSets()} disabled={loading}>
          ↻ Tải lại
        </button>
      </div>
    </>
  );
}
