// GrammarSetEditor — tạo bộ ngữ pháp
// Tham chiếu: hquizlet/apps/nextjs/src/components/study-set/grammar-set-form.tsx

import React, { useState } from "react";
import { useAuth } from "../auth/AuthContext";
import { grammarApi } from "../../lib/api";

interface GrammarExample {
  sentence: string;
  reading: string;
  translation: string;
}

interface GrammarPoint {
  key: string;
  title: string;
  meaning: string;
  usage: string;
  examples: GrammarExample[];
  position: number;
  collapsed: boolean;
}

type Props = {
  onSave: () => void;
  onCancel: () => void;
};

function newExample(): GrammarExample {
  return { sentence: "", reading: "", translation: "" };
}

function newGrammarPoint(position: number): GrammarPoint {
  return {
    key: crypto.randomUUID(),
    title: "",
    meaning: "",
    usage: "",
    examples: [newExample()],
    position,
    collapsed: false,
  };
}

function GrammarPointCard({
  point,
  index,
  onChange,
  onRemove,
  disabled,
}: {
  point: GrammarPoint;
  index: number;
  onChange: (p: GrammarPoint) => void;
  onRemove: () => void;
  disabled: boolean;
}) {
  function setField<K extends keyof GrammarPoint>(key: K, value: GrammarPoint[K]) {
    onChange({ ...point, [key]: value });
  }

  function updateExample(exIdx: number, field: keyof GrammarExample, value: string) {
    const newExamples = point.examples.map((ex, i) =>
      i === exIdx ? { ...ex, [field]: value } : ex
    );
    onChange({ ...point, examples: newExamples });
  }

  function addExample() {
    onChange({ ...point, examples: [...point.examples, newExample()] });
  }

  function removeExample(exIdx: number) {
    onChange({ ...point, examples: point.examples.filter((_, i) => i !== exIdx) });
  }

  return (
    <article className="grammar-point-card draft-card">
      <div className="draft-card-header">
        <div className="draft-title">
          <span className="draft-number">{index + 1}</span>
          <strong>{point.title || <span className="text-muted">Chưa có tên ngữ pháp</span>}</strong>
        </div>
        <div className="card-actions">
          <button
            type="button"
            className="icon-button"
            onClick={() => setField("collapsed", !point.collapsed)}
            aria-label={point.collapsed ? "Mở rộng" : "Thu gọn"}
          >
            {point.collapsed ? "▼" : "▲"}
          </button>
          <button
            type="button"
            className="icon-button"
            onClick={onRemove}
            disabled={disabled}
            aria-label="Xóa"
          >
            ×
          </button>
        </div>
      </div>

      {!point.collapsed && (
        <div className="grammar-point-body">
          <label>
            Tên ngữ pháp <span className="required">*</span>
            <input
              placeholder="Ví dụ: N + は + N + です"
              value={point.title}
              onChange={(e) => setField("title", e.target.value)}
              disabled={disabled}
            />
          </label>

          <label>
            Ý nghĩa <span className="required">*</span>
            <textarea
              rows={2}
              placeholder="Giải thích ý nghĩa của ngữ pháp này..."
              value={point.meaning}
              onChange={(e) => setField("meaning", e.target.value)}
              disabled={disabled}
            />
          </label>

          <label>
            Cách dùng
            <textarea
              rows={3}
              placeholder="Hướng dẫn cách sử dụng, lưu ý đặc biệt..."
              value={point.usage}
              onChange={(e) => setField("usage", e.target.value)}
              disabled={disabled}
            />
          </label>

          <div className="grammar-examples">
            <div className="grammar-examples-header">
              <strong>Ví dụ</strong>
              <button
                type="button"
                className="secondary-button"
                onClick={addExample}
                disabled={disabled}
              >
                + Thêm ví dụ
              </button>
            </div>

            {point.examples.length === 0 && (
              <p className="empty-hint">Chưa có ví dụ nào. Nhấn "Thêm ví dụ" để thêm.</p>
            )}

            {point.examples.map((ex, exIdx) => (
              <div key={exIdx} className="grammar-example-item">
                <div className="grammar-example-header">
                  <span>Ví dụ {exIdx + 1}</span>
                  <button
                    type="button"
                    className="icon-button"
                    onClick={() => removeExample(exIdx)}
                    disabled={disabled}
                    aria-label="Xóa ví dụ"
                  >
                    ×
                  </button>
                </div>
                <input
                  placeholder="Câu ví dụ (ngôn ngữ gốc)　やっとレポートを書き上げた。"
                  value={ex.sentence}
                  onChange={(e) => updateExample(exIdx, "sentence", e.target.value)}
                  disabled={disabled}
                />
                <input
                  placeholder="Cách đọc (romaji/furigana)　Yatto repōto wo kaki ageta."
                  value={ex.reading}
                  onChange={(e) => updateExample(exIdx, "reading", e.target.value)}
                  disabled={disabled}
                  style={{ fontStyle: "italic", color: "var(--muted-foreground)" }}
                />
                <input
                  placeholder="Dịch nghĩa　Cuối cùng mình cũng đã viết xong báo cáo."
                  value={ex.translation}
                  onChange={(e) => updateExample(exIdx, "translation", e.target.value)}
                  disabled={disabled}
                />
              </div>
            ))}
          </div>
        </div>
      )}
    </article>
  );
}

export function GrammarSetEditor({ onSave, onCancel }: Props) {
  const { token } = useAuth();
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [points, setPoints] = useState<GrammarPoint[]>([newGrammarPoint(0)]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  function updatePoint(key: string, updated: GrammarPoint) {
    setPoints((prev) => prev.map((p) => (p.key === key ? updated : p)));
  }

  function removePoint(key: string) {
    setPoints((prev) => {
      if (prev.length === 1) return prev;
      return prev.filter((p) => p.key !== key).map((p, i) => ({ ...p, position: i }));
    });
  }

  function addPoint() {
    setPoints((prev) => [...prev, newGrammarPoint(prev.length)]);
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");

    if (!title.trim()) {
      setError("Vui lòng nhập tiêu đề bộ ngữ pháp.");
      return;
    }
    if (points.some((p) => !p.title.trim() || !p.meaning.trim())) {
      setError("Mỗi mục ngữ pháp cần có tên và ý nghĩa.");
      return;
    }

    setLoading(true);
    try {
      const payload = {
        title: title.trim(),
        description: description.trim(),
        contentType: "grammar" as const,
        grammarPoints: points.map((p, i) => ({
          title: p.title.trim(),
          meaning: p.meaning.trim(),
          usage: p.usage.trim() || undefined,
          position: i,
          examples: p.examples
            .filter((ex) => ex.sentence.trim())
            .map((ex) => ({
              sentence: ex.sentence.trim(),
              reading: ex.reading.trim() || undefined,
              translation: ex.translation.trim() || undefined,
            })),
        })),
      };
      await grammarApi.create(token, payload);
      onSave();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Không thể tạo bộ ngữ pháp. Vui lòng thử lại.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <form className="create-page" onSubmit={handleSubmit}>
      <section className="create-header">
        <div>
          <p className="eyebrow">Tạo bộ học mới</p>
          <h1>Tạo Bộ Ngữ Pháp</h1>
        </div>
        <div className="header-actions">
          <button className="ghost-button" type="button" onClick={onCancel} disabled={loading}>
            Hủy
          </button>
          <button className="primary-button" type="submit" disabled={loading}>
            {loading ? "Đang lưu..." : "Tạo Bộ Ngữ Pháp"}
          </button>
        </div>
      </section>

      <section className="create-meta">
        <label>
          Tiêu đề <span className="required">*</span>
          <input
            autoFocus
            placeholder="Ví dụ: Ngữ pháp N5 — Cấu trúc cơ bản"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            disabled={loading}
          />
        </label>
        <label>
          Mô tả (tùy chọn)
          <textarea
            rows={2}
            placeholder="Mô tả ngắn về bộ ngữ pháp..."
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            disabled={loading}
          />
        </label>
      </section>

      {error && <p className="message message--error">{error}</p>}

      <section className="cards-editor">
        <div className="cards-editor-heading">
          <div>
            <p className="eyebrow">Các mục ngữ pháp</p>
            <h2>Danh sách ngữ pháp ({points.length})</h2>
          </div>
          <button className="secondary-button" type="button" onClick={addPoint} disabled={loading}>
            + Thêm mục
          </button>
        </div>

        {points.map((p, idx) => (
          <GrammarPointCard
            key={p.key}
            point={p}
            index={idx}
            onChange={(updated) => updatePoint(p.key, updated)}
            onRemove={() => removePoint(p.key)}
            disabled={loading}
          />
        ))}

        <button className="add-row" type="button" onClick={addPoint} disabled={loading}>
          + Thêm mục ngữ pháp nữa
        </button>
      </section>
    </form>
  );
}
