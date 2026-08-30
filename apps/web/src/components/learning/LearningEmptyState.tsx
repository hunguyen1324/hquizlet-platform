// LearningEmptyState — Dev 4
// Reusable empty state for learning modes when a study set has no cards

import React from "react";

type Props = {
  message?: string;
  hint?: string;
};

export function LearningEmptyState({
  message = "Học phần này chưa có thẻ nào.",
  hint = "Thêm thẻ trong phần 'Sửa thẻ' để bắt đầu học.",
}: Props) {
  return (
    <div
      style={{
        textAlign: "center",
        padding: "3rem 1rem",
        color: "#6b7280",
      }}
    >
      <p style={{ fontSize: "2rem", marginBottom: "0.5rem" }}>📭</p>
      <p style={{ fontSize: "1rem", fontWeight: 500, color: "#374151" }}>{message}</p>
      <p style={{ fontSize: "0.875rem", marginTop: "0.25rem" }}>{hint}</p>
    </div>
  );
}
