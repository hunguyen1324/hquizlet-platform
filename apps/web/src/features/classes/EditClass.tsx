// EditClass — Phase 7 [P7-FE-CLASS-06]
// Form to edit class name and description.

import { useState } from "react";
import { useAuth } from "../auth/AuthContext";
import { classApi } from "../../lib/api";
import type { ClassDetail } from "../../types";

type Props = {
  cls: ClassDetail;
  onSaved: (cls: ClassDetail) => void;
  onCancel: () => void;
};

export function EditClass({ cls, onSaved, onCancel }: Props) {
  const { token } = useAuth();
  const [name, setName] = useState(cls.name);
  const [description, setDescription] = useState(cls.description || "");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!token) return;
    if (!name.trim()) {
      setError("Tên lớp không được để trống");
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      const updated = await classApi.update(token, cls.id, {
        name: name.trim(),
        description: description.trim(),
      });
      onSaved(updated);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="form-page">
      <h1>Sửa lớp: {cls.name}</h1>
      <form onSubmit={handleSubmit} className="class-form">
        <label>
          Tên lớp *
          <input type="text" value={name} onChange={(e) => setName(e.target.value)} maxLength={120} required />
        </label>
        <label>
          Mô tả
          <textarea value={description} onChange={(e) => setDescription(e.target.value)} rows={3} />
        </label>
        {error && <div className="error-message">{error}</div>}
        <div className="form-actions">
          <button type="button" className="ghost-button" onClick={onCancel} disabled={submitting}>Hủy</button>
          <button type="submit" className="primary-button" disabled={submitting}>
            {submitting ? "Đang lưu..." : "Lưu thay đổi"}
          </button>
        </div>
      </form>
    </div>
  );
}
