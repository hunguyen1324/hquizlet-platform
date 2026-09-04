// JoinClass — Phase 7 [P7-FE-JOIN-01]
// Page to join a class by invite code.

import { useState } from "react";
import { useAuth } from "../auth/AuthContext";
import { classApi } from "../../lib/api";
import type { JoinClassResponse } from "../../types";

type Props = {
  onJoined: (resp: JoinClassResponse) => void;
  onCancel: () => void;
};

export function JoinClass({ onJoined, onCancel }: Props) {
  const { token } = useAuth();
  const [code, setCode] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!token) return;
    const trimmed = code.trim().toUpperCase();
    if (!trimmed) {
      setError("Mã mời không được để trống");
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      const resp = await classApi.join(token, trimmed);
      onJoined(resp);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="group-join-page">
      <button className="ghost-button group-back" onClick={onCancel}>← Quay lại</button>
      <form onSubmit={handleSubmit} className="group-join-card">
        <div className="group-join-icon" aria-hidden="true">⌁</div>
        <p className="group-eyebrow">Tham gia nhanh</p>
        <h1>Nhập mã nhóm hoặc mã game</h1>
        <p>Nhập mã được giáo viên hoặc trưởng nhóm chia sẻ để vào đúng không gian học.</p>
        <label>
          Mã tham gia
          <input
            type="text"
            value={code}
            onChange={(e) => setCode(e.target.value.toUpperCase())}
            placeholder="Q7KM2P3R"
            maxLength={8}
            required
            autoFocus
          />
        </label>
        {error && <div className="error-message">{error}</div>}
        <div className="form-actions group-join-actions">
          <button type="submit" className="primary-button" disabled={submitting || !code.trim()}>
            {submitting ? "Đang tham gia..." : "Tham gia"}
          </button>
        </div>
      </form>
    </div>
  );
}
