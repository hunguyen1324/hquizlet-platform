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
    <div className="form-page">
      <h1>Tham gia lớp</h1>
      <form onSubmit={handleSubmit} className="class-form">
        <label>
          Mã mời
          <input
            type="text"
            value={code}
            onChange={(e) => setCode(e.target.value.toUpperCase())}
            placeholder="VD: Q7KM2P3R"
            maxLength={8}
            required
            autoFocus
          />
        </label>
        {error && <div className="error-message">{error}</div>}
        <div className="form-actions">
          <button type="button" className="ghost-button" onClick={onCancel} disabled={submitting}>Hủy</button>
          <button type="submit" className="primary-button" disabled={submitting}>
            {submitting ? "Đang tham gia..." : "Tham gia"}
          </button>
        </div>
      </form>
    </div>
  );
}
