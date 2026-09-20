// SpacedRepetitionPanel.tsx
// PM: Hiển thị tiến độ Spaced Repetition kiểu Quizlet
// 4 ô: Thẻ mới | Đang học | Sắp thuộc | Thành thạo
// + nút "Học lại từ đầu"

import React from "react";

export type SRStats = {
  newCards: number;
  learning: number;
  almostMastered: number;
  mastered: number;
};

type Props = {
  stats: SRStats;
  onReset?: () => void;
  loading?: boolean;
};

export function SpacedRepetitionPanel({ stats, onReset, loading }: Props) {
  const total = stats.newCards + stats.learning + stats.almostMastered + stats.mastered;

  return (
    <div className="sr-panel">
      <div className="sr-panel__header">
        <span className="sr-panel__icon" aria-hidden="true">⊙</span>
        <h3 className="sr-panel__title">Tiến độ Spaced Repetition</h3>
        {onReset && (
          <button className="sr-panel__reset" onClick={onReset} title="Học lại từ đầu">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <polyline points="1 4 1 10 7 10"/>
              <path d="M3.51 15a9 9 0 1 0 .49-4.15"/>
            </svg>
            Học lại từ đầu
          </button>
        )}
      </div>

      {loading ? (
        <div className="sr-panel__loading">
          <div className="skeleton-row" style={{ height: 80, borderRadius: 12 }} />
        </div>
      ) : (
        <div className="sr-panel__grid">
          <div className="sr-stat sr-stat--new">
            <svg className="sr-stat__icon" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
              <path d="M12 2L2 7l10 5 10-5-10-5z"/>
              <path d="M2 17l10 5 10-5"/>
              <path d="M2 12l10 5 10-5"/>
            </svg>
            <span className="sr-stat__value">{loading ? "—" : stats.newCards}</span>
            <span className="sr-stat__label">THẺ MỚI</span>
          </div>

          <div className="sr-stat sr-stat--learning">
            <svg className="sr-stat__icon" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
              <path d="M21.5 2v6h-6"/>
              <path d="M2.5 12a10 10 0 0 1 17.8-6.3L21.5 8"/>
              <path d="M2.5 22v-6h6"/>
              <path d="M21.5 12a10 10 0 0 1-17.8 6.3l-1.2-2.3"/>
            </svg>
            <span className="sr-stat__value">{loading ? "—" : stats.learning}</span>
            <span className="sr-stat__label">ĐANG HỌC</span>
          </div>

          <div className="sr-stat sr-stat--almost">
            <svg className="sr-stat__icon" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
              <circle cx="12" cy="12" r="10"/>
              <path d="M12 6v6l4 2"/>
            </svg>
            <span className="sr-stat__value">{loading ? "—" : stats.almostMastered}</span>
            <span className="sr-stat__label">SẮP THUỘC</span>
          </div>

          <div className="sr-stat sr-stat--mastered">
            <svg className="sr-stat__icon" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
              <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/>
              <polyline points="22 4 12 14.01 9 11.01"/>
            </svg>
            <span className="sr-stat__value">{loading ? "—" : stats.mastered}</span>
            <span className="sr-stat__label">THÀNH THẠO</span>
          </div>
        </div>
      )}

      {!loading && total > 0 && (
        <div className="sr-panel__progress-bar">
          <div
            className="sr-panel__progress-fill"
            style={{ width: `${Math.round((stats.mastered / total) * 100)}%` }}
            title={`${Math.round((stats.mastered / total) * 100)}% thành thạo`}
          />
        </div>
      )}
    </div>
  );
}
