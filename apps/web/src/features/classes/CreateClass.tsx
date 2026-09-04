// CreateClass — Phase 7 [P7-FE-CLASS-02]
// Form to create a new class.

import { useState } from "react";
import { useAuth } from "../auth/AuthContext";
import { classApi } from "../../lib/api";
import type { ClassDetail } from "../../types";

type Props = {
  onCreated: (cls: ClassDetail) => void;
  onCancel: () => void;
};

export function CreateClass({ onCreated, onCancel }: Props) {
  const { token } = useAuth();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [maxMembers, setMaxMembers] = useState(100);
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
      const cls = await classApi.create(token, {
        name: name.trim(),
        description: description.trim(),
        maxMembers,
      });
      onCreated(cls);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="form-page group-form-page">
      <p className="group-eyebrow">Nhóm học mới</p>
      <h1>Tạo không gian học tập</h1>
      <p className="group-form-intro">Dùng một nhóm cho flashcard, quiz và các phiên thi trực tiếp.</p>
      <form onSubmit={handleSubmit} className="class-form group-form">
        <label>
          Tên nhóm *
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="VD: TOEIC 900 — Buổi tối"
            maxLength={120}
            required
          />
        </label>
        <label>
          Mô tả
          <textarea
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Mục tiêu và nội dung học chính của nhóm..."
            rows={3}
          />
        </label>
        <label>
          Số thành viên tối đa
          <input
            type="number"
            value={maxMembers}
            onChange={(e) => setMaxMembers(Number(e.target.value))}
            min={2}
            max={1000}
          />
        </label>
        {error && <div className="error-message">{error}</div>}
        <div className="form-actions">
          <button type="button" className="ghost-button" onClick={onCancel} disabled={submitting}>Hủy</button>
          <button type="submit" className="primary-button" disabled={submitting}>
            {submitting ? "Đang tạo..." : "Tạo nhóm"}
          </button>
        </div>
      </form>
    </div>
  );
}
