// InlineFlashcardEditor — sửa / thêm flashcard ngay trong StudyDetail
import React from "react";
import type { Flashcard } from "../../types";
import "./StudyDetail.css";

type Mode = "edit" | "add";

type Props = {
  mode: Mode;
  initialTerm?: string;
  initialDefinition?: string;
  onSave: (term: string, definition: string) => Promise<void>;
  onCancel: () => void;
};

export function InlineFlashcardEditor({
  mode,
  initialTerm = "",
  initialDefinition = "",
  onSave,
  onCancel,
}: Props) {
  const [term, setTerm] = React.useState(initialTerm);
  const [definition, setDefinition] = React.useState(initialDefinition);
  const [saving, setSaving] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const termRef = React.useRef<HTMLInputElement>(null);

  React.useEffect(() => {
    termRef.current?.focus();
  }, []);

  async function handleSave() {
    if (!term.trim()) {
      setError("Thuật ngữ không được để trống");
      termRef.current?.focus();
      return;
    }
    setSaving(true);
    setError(null);
    try {
      await onSave(term.trim(), definition.trim());
    } catch (e) {
      setError(e instanceof Error ? e.message : "Không thể lưu thẻ");
    } finally {
      setSaving(false);
    }
  }

  function handleKey(e: React.KeyboardEvent) {
    if (e.key === "Escape") onCancel();
    if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) void handleSave();
  }

  return (
    <div className="ql-card-row ql-card-row--editing" role="form" aria-label={mode === "add" ? "Thêm thẻ mới" : "Chỉnh sửa thẻ"}>
      <div className="ql-inline-fields">
        <div className="ql-inline-field">
          <label className="ql-inline-label" htmlFor="inline-term">Thuật ngữ</label>
          <input
            id="inline-term"
            ref={termRef}
            type="text"
            className={`ql-inline-input ${!term.trim() && error ? "ql-inline-input--error" : ""}`}
            value={term}
            onChange={(e) => setTerm(e.target.value)}
            onKeyDown={handleKey}
            placeholder="Nhập thuật ngữ…"
            disabled={saving}
            autoComplete="off"
          />
        </div>
        <div className="ql-inline-sep" aria-hidden="true" />
        <div className="ql-inline-field">
          <label className="ql-inline-label" htmlFor="inline-definition">Định nghĩa</label>
          <input
            id="inline-definition"
            type="text"
            className="ql-inline-input"
            value={definition}
            onChange={(e) => setDefinition(e.target.value)}
            onKeyDown={handleKey}
            placeholder="Nhập định nghĩa…"
            disabled={saving}
            autoComplete="off"
          />
        </div>
      </div>
      {error && <p className="ql-inline-error" role="alert">{error}</p>}
      <p className="ql-inline-hint">Ctrl+Enter để lưu · Esc để hủy</p>
      <div className="ql-inline-actions">
        <button
          type="button"
          className="secondary-button"
          onClick={onCancel}
          disabled={saving}
        >
          Hủy
        </button>
        <button
          type="button"
          className="primary-button"
          onClick={() => void handleSave()}
          disabled={saving || !term.trim()}
        >
          {saving ? "Đang lưu…" : mode === "add" ? "Thêm thẻ" : "Lưu"}
        </button>
      </div>
    </div>
  );
}
