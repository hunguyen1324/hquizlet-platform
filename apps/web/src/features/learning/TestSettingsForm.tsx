// TestSettingsForm — hiện trước khi bắt đầu bài kiểm tra
import React from "react";
import "./learning.css";

export type QuestionTypeOption = "multipleChoice" | "written" | "trueFalse";
export type TestSettings = {
  limit: number;
  questionTypes: QuestionTypeOption[];
  askTerm: boolean; // true = hỏi term (hiện term, điền def), false = hỏi def
};

type Props = {
  totalCards: number;
  onStart: (settings: TestSettings) => void;
};

const TYPE_LABELS: Record<QuestionTypeOption, string> = {
  multipleChoice: "Trắc nghiệm",
  written: "Tự luận",
  trueFalse: "Đúng / Sai",
};

export function TestSettingsForm({ totalCards, onStart }: Props) {
  const maxCards = Math.min(totalCards, 20);
  const [limit, setLimit] = React.useState(maxCards);
  const [questionTypes, setQuestionTypes] = React.useState<QuestionTypeOption[]>([
    "multipleChoice",
    "written",
    "trueFalse",
  ]);
  const [askTerm, setAskTerm] = React.useState(true);

  function toggleType(t: QuestionTypeOption) {
    setQuestionTypes((prev) => {
      if (prev.includes(t)) {
        // Không cho bỏ chọn hết
        if (prev.length === 1) return prev;
        return prev.filter((x) => x !== t);
      }
      return [...prev, t];
    });
  }

  function handleStart() {
    onStart({ limit, questionTypes, askTerm });
  }

  return (
    <div className="test-settings-wrap">
      <h2 className="test-settings-title">Cài đặt bài kiểm tra</h2>

      {/* Số câu */}
      <div className="test-settings-section">
        <label className="test-settings-label" htmlFor="test-limit">
          Số câu hỏi
        </label>
        <div className="test-settings-slider-wrap">
          <input
            id="test-limit"
            type="range"
            min={2}
            max={maxCards}
            value={limit}
            onChange={(e) => setLimit(Number(e.target.value))}
            className="test-settings-slider"
          />
          <span className="test-settings-slider-val">{limit}</span>
        </div>
        <p className="test-settings-hint">{totalCards} thẻ trong bộ này · tối đa 20</p>
      </div>

      {/* Loại câu hỏi */}
      <div className="test-settings-section">
        <p className="test-settings-label">Loại câu hỏi</p>
        <div className="test-settings-types">
          {(["multipleChoice", "written", "trueFalse"] as QuestionTypeOption[]).map((t) => (
            <label key={t} className="test-settings-type-item">
              <input
                type="checkbox"
                checked={questionTypes.includes(t)}
                onChange={() => toggleType(t)}
                id={`type-${t}`}
              />
              <span>{TYPE_LABELS[t]}</span>
            </label>
          ))}
        </div>
      </div>

      {/* Term hay definition */}
      <div className="test-settings-section">
        <p className="test-settings-label">Hỏi theo</p>
        <div className="test-settings-types">
          <label className="test-settings-type-item">
            <input
              type="radio"
              name="askSide"
              checked={askTerm}
              onChange={() => setAskTerm(true)}
              id="ask-term"
            />
            <span>Thuật ngữ (điền định nghĩa)</span>
          </label>
          <label className="test-settings-type-item">
            <input
              type="radio"
              name="askSide"
              checked={!askTerm}
              onChange={() => setAskTerm(false)}
              id="ask-def"
            />
            <span>Định nghĩa (điền thuật ngữ)</span>
          </label>
        </div>
      </div>

      <button
        type="button"
        id="test-start-btn"
        className="primary-button test-settings-start-btn"
        onClick={handleStart}
      >
        Bắt đầu kiểm tra
      </button>
    </div>
  );
}
