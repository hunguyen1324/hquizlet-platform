// Shared UI components - Dev 3 (CORE-FB-04)
// Reusable: Button, EmptyState, LoadingSkeleton, ErrorAlert

import React from "react";

export function EmptyState({
  title,
  description,
  action,
}: {
  title: string;
  description?: string;
  action?: { label: string; onClick: () => void };
}) {
  return (
    <div className="empty-panel">
      <h2>{title}</h2>
      {description && <p>{description}</p>}
      {action && (
        <button className="primary-button" onClick={action.onClick}>
          {action.label}
        </button>
      )}
    </div>
  );
}

export function LoadingSkeleton({ rows = 3 }: { rows?: number }) {
  return (
    <div className="loading-skeleton" aria-busy="true">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="skeleton-row" style={{ opacity: 1 - i * 0.2 }} />
      ))}
    </div>
  );
}

export function ErrorAlert({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div className="error-alert" role="alert">
      <span>{message}</span>
      {onRetry && (
        <button className="ghost-button" onClick={onRetry}>
          Thử lại
        </button>
      )}
    </div>
  );
}
