// TestSettingsForm — Quizlet-style redesign
// Toggle switch thay checkbox, number input thay slider, accordion sections
import React from "react";
import "./learning.css";

export type QuestionTypeOption = "multipleChoice" | "written" | "trueFalse";
export type TestSettings = {
  limit: number;
  questionTypes: QuestionTypeOption[];
  askTerm: boolean; // true = hỏi term (hiện term, điền def)
};

type Props = {
  totalCards: number;
  onStart: (settings: TestSettings) => void;
  onCancel?: () => void;
};

const TYPE_LABELS: Record<QuestionTypeOption, string> = {
  multipleChoice: "Trắc nghiệm",
  written: "Tự luận",
  trueFalse: "Đúng / Sai",
};

export function TestSettingsForm({ totalCards, onStart, onCancel }: Props) {
  const maxCards = Math.min(totalCards, 20);
  const [limit, setLimit] = React.useState(maxCards);
  const [limitError, setLimitError] = React.useState("");
  const [questionTypes, setQuestionTypes] = React.useState<QuestionTypeOption[]>([
    "multipleChoice",
    "written",
    "trueFalse",
  ]);
  const [askTerm, setAskTerm] = React.useState(true);
  const [openFormat, setOpenFormat] = React.useState(false);

  function toggleType(t: QuestionTypeOption) {
    setQuestionTypes((prev) => {
      if (prev.includes(t)) {
        if (prev.length === 1) return prev; // Không cho bỏ chọn hết
        return prev.filter((x) => x !== t);
      }
      return [...prev, t];
    });
  }

  function handleLimitChange(val: string) {
    const n = parseInt(val, 10);
    if (isNaN(n)) { setLimitError("Vui lòng nhập số"); return; }
    if (n < 2) { setLimit(2); setLimitError(""); return; }
    if (n > maxCards) { setLimit(maxCards); setLimitError(""); return; }
    setLimit(n);
    setLimitError("");
  }

  function handleStart() {
    if (questionTypes.length === 0) return;
    onStart({ limit, questionTypes, askTerm });
  }

  return (
    <div className="tsf-overlay" role="dialog" aria-modal="true" aria-label="Tạo bài kiểm tra">
      <div className="tsf-panel">
        <div className="tsf-header">
          <h2 className="tsf-title">Tạo bài kiểm tra</h2>
        </div>

        {/* ── Số câu ── */}
        <div className="tsf-section">
          <div className="tsf-row">
            <div className="tsf-row-label">
              <span className="tsf-label">Số câu hỏi</span>
              <span className="tsf-sublabel">Tối đa {maxCards} câu từ bộ thẻ này</span>
            </div>
            <div className="tsf-number-wrap">
              <input
                id="test-limit-input"
                type="number"
                min={2}
                max={maxCards}
                value={limit}
                onChange={(e) => handleLimitChange(e.target.value)}
                className={`tsf-number-input${limitError ? " tsf-number-input--err" : ""}`}
              />
            </div>
          </div>
          {limitError && <p className="tsf-field-error">{limitError}</p>}
        </div>

        {/* ── Loại câu hỏi ── */}
        <div className="tsf-section">
          <p className="tsf-section-title">Loại câu hỏi</p>
          {(["multipleChoice", "written", "trueFalse"] as QuestionTypeOption[]).map((t) => (
            <label key={t} className="ql-toggle-row">
              <span className="ql-toggle-label">{TYPE_LABELS[t]}</span>
              <span
                className={`ql-toggle${questionTypes.includes(t) ? " ql-toggle--on" : ""}`}
                role="switch"
                aria-checked={questionTypes.includes(t)}
                tabIndex={0}
                id={`test-type-${t}`}
                onKeyDown={(e) => { if (e.key === " " || e.key === "Enter") { e.preventDefault(); toggleType(t); } }}
              >
                <span className="ql-toggle-thumb" />
              </span>
              <input
                type="checkbox"
                checked={questionTypes.includes(t)}
                onChange={() => toggleType(t)}
                className="ql-toggle-input"
                aria-hidden="true"
                tabIndex={-1}
              />
            </label>
          ))}
        </div>

        {/* ── Định dạng câu hỏi (accordion) ── */}
        <div className="tsf-section tsf-accordion">
          <button
            type="button"
            className="tsf-accordion-trigger"
            onClick={() => setOpenFormat((v) => !v)}
            aria-expanded={openFormat}
          >
            <span>Định dạng câu hỏi</span>
            <svg
              className={`tsf-accordion-chevron${openFormat ? " tsf-accordion-chevron--open" : ""}`}
              width="14" height="14" viewBox="0 0 24 24" fill="none"
              stroke="currentColor" strokeWidth="2.5"
            >
              <path d="M6 9l6 6 6-6"/>
            </svg>
          </button>
          {openFormat && (
            <div className="tsf-accordion-body">
              <label className="ql-toggle-row" id="ask-term-row">
                <span className="ql-toggle-label">
                  <strong>Hỏi theo Thuật ngữ</strong>
                  <span className="tsf-sublabel">Hiện thuật ngữ → điền định nghĩa</span>
                </span>
                <span
                  className={`ql-toggle${askTerm ? " ql-toggle--on" : ""}`}
                  role="switch"
                  aria-checked={askTerm}
                  tabIndex={0}
                  onKeyDown={(e) => { if (e.key === " " || e.key === "Enter") { e.preventDefault(); setAskTerm((v) => !v); } }}
                >
                  <span className="ql-toggle-thumb" />
                </span>
                <input
                  type="checkbox"
                  checked={askTerm}
                  onChange={(e) => setAskTerm(e.target.checked)}
                  className="ql-toggle-input"
                  aria-hidden="true"
                  tabIndex={-1}
                  id="ask-term"
                />
              </label>
              <p className="tsf-format-note">
                {askTerm
                  ? "Câu hỏi sẽ hiện Thuật ngữ, bạn cần điền Định nghĩa"
                  : "Câu hỏi sẽ hiện Định nghĩa, bạn cần điền Thuật ngữ"}
              </p>
            </div>
          )}
        </div>

        {/* ── Actions ── */}
        <div className="tsf-actions">
          <button
            type="button"
            id="test-start-btn"
            className="tsf-start-btn"
            onClick={handleStart}
            disabled={questionTypes.length === 0 || limit < 2}
          >
            Tạo bài kiểm tra mới
          </button>
          {onCancel && (
            <button type="button" className="tsf-cancel-link" onClick={onCancel}>
              Hủy
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
