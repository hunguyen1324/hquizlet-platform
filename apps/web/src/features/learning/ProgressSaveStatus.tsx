// ProgressSaveStatus.tsx — Dev 4
// P3-LEARN-03: UI component hiển thị trạng thái save progress.
// Dùng ở màn "done" của 4 modes. Nhỏ gọn, không block interaction.

import React from "react";
import type { SaveStatus } from "./useProgressSave";

type Props = {
  status: SaveStatus;
  onRetry?: () => void;
};

export function ProgressSaveStatus({ status, onRetry }: Props) {
  if (status.state === "idle") return null;

  if (status.state === "saving") {
    return (
      <p className="progress-save-status saving" aria-live="polite">
        ⏳ Đang lưu kết quả...
      </p>
    );
  }

  if (status.state === "saved") {
    return (
      <p className="progress-save-status saved" aria-live="polite">
        ✅ Đã lưu kết quả
      </p>
    );
  }

  // error state
  const { error, retryable } = status;
  const message =
    error.kind === "unauthorized" || error.kind === "forbidden"
      ? "Không có quyền lưu kết quả. Vui lòng đăng nhập lại."
      : error.kind === "validation"
      ? `Dữ liệu không hợp lệ: ${"message" in error ? error.message : ""}`
      : "Không thể lưu kết quả. Kiểm tra kết nối mạng.";

  return (
    <div className="progress-save-status error" role="alert">
      <span>⚠️ {message}</span>
      {retryable && onRetry && (
        <button className="ghost-button retry-btn" onClick={onRetry}>
          Thử lại
        </button>
      )}
    </div>
  );
}
