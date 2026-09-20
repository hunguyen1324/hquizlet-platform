// LearnSettingsDialog — gear icon dialog cho LearnMode
import React from "react";
import "./learning.css";

export type LearnSettings = {
  questionTypes: ("written" | "multipleChoice" | "trueFalse")[];
  starredOnly: boolean;
  swapSides: boolean; // true = hỏi definition thay vì term
};

type Props = {
  settings: LearnSettings;
  onClose: () => void;
  onChange: (s: LearnSettings) => void;
  hasStarred: boolean;
};

export function LearnSettingsDialog({ settings, onClose, onChange, hasStarred }: Props) {
  const [local, setLocal] = React.useState<LearnSettings>(settings);

  function toggleType(t: "written" | "multipleChoice" | "trueFalse") {
    setLocal((prev) => {
      const has = prev.questionTypes.includes(t);
      if (has && prev.questionTypes.length === 1) return prev;
      return {
        ...prev,
        questionTypes: has
          ? prev.questionTypes.filter((x) => x !== t)
          : [...prev.questionTypes, t],
      };
    });
  }

  function handleApply() {
    onChange(local);
    onClose();
  }

  return (
    <div className="ql-settings-overlay" role="dialog" aria-modal="true" aria-label="Cài đặt học">
      <div className="ql-settings-panel">
        <div className="ql-settings-header">
          <h3>Cài đặt</h3>
          <button type="button" className="ql-settings-close" onClick={onClose} aria-label="Đóng">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
              <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
          </button>
        </div>

        <div className="ql-settings-section">
          <p className="ql-settings-label">Loại câu hỏi</p>
          {(["written", "multipleChoice", "trueFalse"] as const).map((t) => (
            <label key={t} className="ql-settings-check">
              <input
                type="checkbox"
                checked={local.questionTypes.includes(t)}
                onChange={() => toggleType(t)}
                id={`learn-type-${t}`}
              />
              <span>
                {t === "written" && "Tự luận"}
                {t === "multipleChoice" && "Trắc nghiệm"}
                {t === "trueFalse" && "Đúng / Sai"}
              </span>
            </label>
          ))}
        </div>

        <div className="ql-settings-section">
          <label className="ql-settings-check">
            <input
              type="checkbox"
              checked={local.swapSides}
              onChange={(e) => setLocal((p) => ({ ...p, swapSides: e.target.checked }))}
              id="learn-swap"
            />
            <span>Hỏi định nghĩa (điền thuật ngữ)</span>
          </label>
        </div>

        {hasStarred && (
          <div className="ql-settings-section">
            <label className="ql-settings-check">
              <input
                type="checkbox"
                checked={local.starredOnly}
                onChange={(e) => setLocal((p) => ({ ...p, starredOnly: e.target.checked }))}
                id="learn-starred"
              />
              <span>Chỉ học thẻ đánh dấu sao ★</span>
            </label>
          </div>
        )}

        <div className="ql-settings-footer">
          <button type="button" className="secondary-button" onClick={onClose}>Hủy</button>
          <button type="button" className="primary-button" onClick={handleApply}>Áp dụng</button>
        </div>
      </div>
    </div>
  );
}
