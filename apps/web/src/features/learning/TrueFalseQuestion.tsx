// TrueFalseQuestion — dùng chung cho LearnMode và TestMode
import React from "react";
import "./learning.css";

type Props = {
  term: string;
  /** Definition hiện lên để user phán đoán đúng/sai */
  displayDefinition: string;
  /** Đáp án thực sự (để so sánh) */
  correctDefinition?: string;
  /** Đã chọn chưa (null = chưa) */
  answer: boolean | null;
  /** Đã submit / reveal chưa */
  revealed: boolean;
  onAnswer: (isTrue: boolean) => void;
};

export function TrueFalseQuestion({
  term,
  displayDefinition,
  answer,
  revealed,
  onAnswer,
}: Props) {
  return (
    <div className="ql-tf-wrap">
      <p className="ql-tf-prompt">Định nghĩa này có đúng với thuật ngữ không?</p>
      <div className="ql-tf-display-def">{displayDefinition}</div>
      <div className="ql-tf-btns">
        <button
          type="button"
          id="tf-btn-true"
          className={[
            "ql-tf-btn",
            answer === true ? "ql-tf-btn--selected" : "",
            revealed && answer === true
              ? answer === true
                ? "ql-tf-btn--correct"
                : "ql-tf-btn--wrong"
              : "",
          ]
            .filter(Boolean)
            .join(" ")}
          onClick={() => onAnswer(true)}
          disabled={revealed}
          aria-pressed={answer === true}
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <polyline points="20 6 9 17 4 12" />
          </svg>
          Đúng
        </button>
        <button
          type="button"
          id="tf-btn-false"
          className={[
            "ql-tf-btn",
            answer === false ? "ql-tf-btn--selected" : "",
            revealed && answer === false
              ? answer === false
                ? "ql-tf-btn--correct"
                : "ql-tf-btn--wrong"
              : "",
          ]
            .filter(Boolean)
            .join(" ")}
          onClick={() => onAnswer(false)}
          disabled={revealed}
          aria-pressed={answer === false}
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" />
          </svg>
          Sai
        </button>
      </div>
      {revealed && (
        <p className="ql-tf-hint">
          Thuật ngữ: <strong>{term}</strong>
        </p>
      )}
    </div>
  );
}
