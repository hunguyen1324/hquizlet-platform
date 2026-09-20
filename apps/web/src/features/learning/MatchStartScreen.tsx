// MatchStartScreen — hiện trước khi bắt đầu ghép cặp
import React from "react";
import "./learning.css";

type Props = {
  totalCards: number;
  onStart: (pairCount: number) => void;
};

export function MatchStartScreen({ totalCards, onStart }: Props) {
  const maxPairs = Math.min(totalCards, 20);
  const [pairCount, setPairCount] = React.useState(Math.min(maxPairs, 6));

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
        Ghép các thuật ngữ với định nghĩa tương ứng. Nhanh tay xem bạn mất bao lâu!
      </p>

      <div className="match-start-slider-wrap">
        <label htmlFor="pair-slider" className="match-start-slider-label">
          Số cặp: <strong>{pairCount}</strong>
        </label>
        <input
          id="pair-slider"
          type="range"
          min={Math.min(4, maxPairs)}
          max={maxPairs}
          value={pairCount}
          onChange={(e) => setPairCount(parseInt(e.target.value, 10))}
          className="match-start-slider"
        />
      </div>

      <ul className="ql-match-start-rules">
        <li>Click một thẻ, rồi click thẻ khớp của nó</li>
        <li>Ghép sai → thẻ rung và không bị xóa</li>
        <li>Ghép hết tất cả cặp để hoàn thành</li>
      </ul>
      <button
        id="match-start-btn"
        type="button"
        className="primary-button ql-match-start-btn"
        onClick={() => onStart(pairCount)}
        autoFocus
      >
        Bắt đầu
      </button>
    </div>
  );
}
