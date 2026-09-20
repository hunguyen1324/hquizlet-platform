// MatchStartScreen — hiện trước khi bắt đầu ghép cặp
import React from "react";
import "./learning.css";

type Props = {
  totalPairs: number;
  onStart: () => void;
};

export function MatchStartScreen({ totalPairs, onStart }: Props) {
  return (
    <div className="ql-match-start-screen">
      <div className="ql-match-start-icon" aria-hidden="true">
        <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          <rect x="3" y="3" width="7" height="7" rx="1"/>
          <rect x="14" y="3" width="7" height="7" rx="1"/>
          <rect x="3" y="14" width="7" height="7" rx="1"/>
          <rect x="14" y="14" width="7" height="7" rx="1"/>
          <path d="M7 7l3 3M14 10l3-3M7 14l3-3M14 14l3 3" opacity=".4"/>
        </svg>
      </div>
      <h2 className="ql-match-start-title">Ghép cặp</h2>
      <p className="ql-match-start-desc">
        Ghép <strong>{totalPairs}</strong> thuật ngữ với định nghĩa tương ứng.
        Nhanh tay xem bạn mất bao lâu!
      </p>
      <ul className="ql-match-start-rules">
        <li>Click một thẻ, rồi click thẻ khớp của nó</li>
        <li>Ghép sai → thẻ rung và không bị xóa</li>
        <li>Ghép hết tất cả cặp để hoàn thành</li>
      </ul>
      <button
        id="match-start-btn"
        type="button"
        className="primary-button ql-match-start-btn"
        onClick={onStart}
        autoFocus
      >
        Bắt đầu
      </button>
    </div>
  );
}
