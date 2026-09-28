// features/folders/CreateFolderDialog.tsx
// Dialog tạo/sửa thư mục — logic giống folder-dialog.tsx trong hquizlet gốc.
// Dùng <dialog> native (không cần thư viện) để phù hợp với codebase hiện tại.

import React, { useEffect, useRef } from "react";
import { folderApi } from "../../lib/api";
import type { FolderSummary } from "../../lib/api";
import { useAuth } from "../auth/AuthContext";

interface CreateFolderDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Truyền vào khi muốn sửa thư mục có sẵn */
  defaultValues?: { id: number; title: string; description: string };
  onSuccess?: (folder: FolderSummary) => void;
}

export function CreateFolderDialog({
  open,
  onOpenChange,
  defaultValues,
  onSuccess,
}: CreateFolderDialogProps) {
  const { token } = useAuth();
  const dialogRef = useRef<HTMLDialogElement>(null);
  const isEdit = Boolean(defaultValues);

  const [title, setTitle] = React.useState(defaultValues?.title ?? "");
  const [description, setDescription] = React.useState(defaultValues?.description ?? "");
  const [saving, setSaving] = React.useState(false);
  const [error, setError] = React.useState("");

  // Sync controlled open prop → dialog element
  useEffect(() => {
    const el = dialogRef.current;
    if (!el) return;
    if (open && !el.open) {
      el.showModal();
    } else if (!open && el.open) {
      el.close();
    }
  }, [open]);

  // Reset form khi mở dialog
  useEffect(() => {
    if (open) {
      setTitle(defaultValues?.title ?? "");
      setDescription(defaultValues?.description ?? "");
      setError("");
    }
  }, [open, defaultValues]);

  // Đóng khi nhấn Escape
  function handleClose() {
    onOpenChange(false);
  }

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    const trimmedTitle = title.trim();
    if (!trimmedTitle) {
      setError("Tiêu đề là bắt buộc.");
      return;
    }
    setSaving(true);
    setError("");
    try {
      let result: FolderSummary;
      if (isEdit && defaultValues) {
        result = await folderApi.updateFolder(token, defaultValues.id, {
          title: trimmedTitle,
          description: description.trim(),
        });
      } else {
        result = await folderApi.createFolder(token, {
          title: trimmedTitle,
          description: description.trim(),
        });
      }
      onSuccess?.(result);
      onOpenChange(false);
    } catch (value) {
      setError(value instanceof Error ? value.message : "Không thể lưu thư mục, thử lại.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <dialog
      ref={dialogRef}
      className="cf-dialog"
      aria-labelledby="cf-dialog-title"
      onClose={handleClose}
    >
      {/* Backdrop click → đóng */}
      <div
        className="cf-dialog-backdrop"
        role="presentation"
        onClick={handleClose}
      />
      <div className="cf-dialog-content">
        <header className="cf-dialog-header">
          <div>
            <p className="eyebrow">Thư mục</p>
            <h2 id="cf-dialog-title">
              {isEdit ? "Sửa thư mục" : "Tạo thư mục"}
            </h2>
            <p className="cf-dialog-subtitle">
              Sắp xếp học phần theo chủ đề mà không thay đổi dữ liệu gốc.
            </p>
          </div>
          <button
            type="button"
            className="cf-dialog-close ghost-button"
            aria-label="Đóng"
            onClick={handleClose}
          >
            ×
          </button>
        </header>

        <form onSubmit={(e) => void handleSubmit(e)} className="cf-dialog-form stack">
          <label className="cf-field">
            <span className="cf-label">Tiêu đề</span>
            <input
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Ví dụ: Từ vựng N2"
              disabled={saving}
              autoFocus
            />
          </label>

          <label className="cf-field">
            <span className="cf-label">Mô tả (không bắt buộc)</span>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Mô tả ngắn về thư mục này..."
              rows={3}
              disabled={saving}
            />
          </label>

          {error && (
            <p className="message message--error" role="alert">
              {error}
            </p>
          )}

          <div className="cf-dialog-actions">
            <button
              type="button"
              className="ghost-button"
              onClick={handleClose}
              disabled={saving}
            >
              Hủy
            </button>
            <button
              type="submit"
              className="primary-button"
              disabled={saving}
            >
              {saving ? "Đang lưu..." : isEdit ? "Lưu thay đổi" : "Tạo thư mục"}
            </button>
          </div>
        </form>
      </div>
    </dialog>
  );
}
