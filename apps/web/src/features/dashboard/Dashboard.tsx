// Dashboard - Dev 3 (FE-CORE-05: Dashboard study set list)
// Shows list of study sets, empty state, reload. Uses mock data until Dev 2 ready.

import React, { useEffect, useState } from "react";
import type { StudySet, HealthStatus } from "../../types";
import { MOCK_SETS } from "../../lib/mock/mockData";

type Props = {
  healthStatus: HealthStatus;
  onOpen: (id: number) => void;
  onCreate: () => void;
};

export function Dashboard({ healthStatus, onOpen, onCreate }: Props) {
  const [sets, setSets] = useState<StudySet[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const loadSets = async () => {
    setLoading(true);
    setError("");
    try {
      // TODO (FE-CORE-07): swap to real API when Dev 2 ready
      // const data = await apiClient.get<StudySet[]>("/v1/study-sets");
      await new Promise((r) => setTimeout(r, 300)); // simulate network
      setSets(MOCK_SETS);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Không tải được study sets.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void loadSets(); }, []);

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

      {error && <p className="message message--error">{error}</p>}

      <section className="set-grid">
        {loading && <p className="loading-state">Đang tải...</p>}

        {!loading && sets.length === 0 && (
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
