// WrittenQuestion — text input, submit, feedback đúng/sai
import React from "react";
import "./learning.css";

type Props = {
  value: string;
  onChange: (v: string) => void;
  onSubmit: () => void;
  onNext: () => void;
  feedback: boolean | null;
  submitting: boolean;
  correctAnswer: string;
  inputRef?: React.RefObject<HTMLInputElement | null>;
};

export function WrittenQuestion({
  value,
  onChange,
  onSubmit,
  onNext,
  feedback,
  submitting,
  correctAnswer,
  inputRef,
}: Props) {
  return (
    <div className="ql-written-wrap">
      <label className="ql-written-label" htmlFor="written-answer">
        Nhập định nghĩa
      </label>
      <input
        id="written-answer"
        ref={inputRef as React.RefObject<HTMLInputElement>}
        type="text"
        className={[
          "ql-written-input",
          feedback === true ? "ql-written-input--correct" : "",
          feedback === false ? "ql-written-input--wrong" : "",
        ]
          .filter(Boolean)
          .join(" ")}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            if (feedback === null) void onSubmit();
            else onNext();
          }
        }}
        disabled={submitting || feedback !== null}
        autoComplete="off"
        autoFocus
        placeholder="Gõ câu trả lời của bạn…"
      />
      {feedback !== null && (
        <div
          className={`ql-written-feedback ${
            feedback ? "ql-written-feedback--correct" : "ql-written-feedback--wrong"
          }`}
        >
          {feedback ? (
            <>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" aria-hidden="true"><polyline points="20 6 9 17 4 12" /></svg>
              Chính xác!
            </>
          ) : (
            <>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" aria-hidden="true"><line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" /></svg>
              Chưa đúng. Đáp án: <strong>{correctAnswer}</strong>
            </>
          )}
        </div>
      )}
      <div className="ql-written-actions">
        {feedback === null ? (
          <button
            type="button"
            className="primary-button"
            onClick={() => void onSubmit()}
            disabled={!value.trim() || submitting}
          >
            {submitting ? "Đang chấm…" : "Kiểm tra"}
          </button>
        ) : (
          <button type="button" className="primary-button" onClick={onNext}>
            Tiếp theo →
          </button>
        )}
      </div>
    </div>
  );
}
