// FlashcardsSettingsDialog — cài đặt cho Flashcards mode
import React from "react";
import "./learning.css";

export type FlashcardsSettings = {
  startFromDefinition: boolean;
  autoPlay: boolean;
};

type Props = {
  settings: FlashcardsSettings;
  onClose: () => void;
  onChange: (s: FlashcardsSettings) => void;
};

export function FlashcardsSettingsDialog({ settings, onClose, onChange }: Props) {
  const [local, setLocal] = React.useState<FlashcardsSettings>(settings);

  return (
    <div className="ql-settings-overlay" role="dialog" aria-modal="true" aria-label="Cài đặt Flashcards">
      <div className="ql-settings-panel">
        <div className="ql-settings-header">
          <h3>Cài đặt Flashcards</h3>
          <button type="button" className="ql-settings-close" onClick={onClose} aria-label="Đóng">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
              <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
          </button>
        </div>

        <div className="ql-settings-section">
          <label className="ql-settings-check">
            <input
              type="checkbox"
              checked={local.startFromDefinition}
              onChange={(e) => setLocal((p) => ({ ...p, startFromDefinition: e.target.checked }))}
              id="fc-start-def"
            />
            <span>Bắt đầu từ mặt Định nghĩa</span>
          </label>
        </div>

        <div className="ql-settings-section">
          <label className="ql-settings-check">
            <input
              type="checkbox"
              checked={local.autoPlay}
              onChange={(e) => setLocal((p) => ({ ...p, autoPlay: e.target.checked }))}
              id="fc-autoplay"
            />
            <span>Tự động phát âm khi lật thẻ</span>
          </label>
        </div>

        <div className="ql-settings-footer">
          <button type="button" className="secondary-button" onClick={onClose}>Hủy</button>
          <button
            type="button"
            className="primary-button"
            onClick={() => { onChange(local); onClose(); }}
          >
            Áp dụng
          </button>
        </div>
      </div>
    </div>
  );
}
