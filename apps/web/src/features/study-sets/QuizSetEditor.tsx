// QuizSetEditor — tạo/sửa quiz set (multiple choice, true/false, written)
// Tham chiếu: hquizlet/apps/nextjs/src/components/study-set/quiz-set-form.tsx

import React, { useState } from "react";
import { useAuth } from "../auth/AuthContext";
import { quizApi, importApi } from "../../lib/api";

const LANGUAGES = [
  { code: "en-US", name: "English (US)", flag: "🇺🇸" },
  { code: "vi-VN", name: "Tiếng Việt", flag: "🇻🇳" },
  { code: "ja-JP", name: "日本語", flag: "🇯🇵" },
  { code: "ko-KR", name: "한국어", flag: "🇰🇷" },
  { code: "zh-CN", name: "中文 (简体)", flag: "🇨🇳" },
  { code: "zh-TW", name: "中文 (繁體)", flag: "🇹🇼" },
  { code: "fr-FR", name: "Français", flag: "🇫🇷" },
  { code: "de-DE", name: "Deutsch", flag: "🇩🇪" },
  { code: "es-ES", name: "Español", flag: "🇪🇸" },
];

type QuestionType = "multiple_choice" | "true_false" | "written" | "paragraph" | "sorting";

interface QuizOption {
  text: string;
  position: number;
}

interface QuizQuestion {
  key: string;
  questionText: string;
  questionType: QuestionType;
  correctAnswer: string;
  answerExplanation: string;
  paragraphText: string;
  audioUrl: string;
  options: QuizOption[];
  position: number;
}

type Props = {
  existingSetId?: number;
  onSave: () => void;
  onCancel: () => void;
};

function newQuestion(position: number): QuizQuestion {
  return {
    key: crypto.randomUUID(),
    questionText: "",
    questionType: "multiple_choice",
    correctAnswer: "",
    answerExplanation: "",
    paragraphText: "",
    audioUrl: "",
    options: [
      { text: "", position: 0 },
      { text: "", position: 1 },
      { text: "", position: 2 },
      { text: "", position: 3 },
    ],
    position,
  };
}

function QuestionCard({
  question,
  index,
  onChange,
  onRemove,
  disabled,
}: {
  question: QuizQuestion;
  index: number;
  onChange: (q: QuizQuestion) => void;
  onRemove: () => void;
  disabled: boolean;
}) {
  function setField<K extends keyof QuizQuestion>(key: K, value: QuizQuestion[K]) {
    onChange({ ...question, [key]: value });
  }

  function setOptionText(optIdx: number, text: string) {
    const newOptions = question.options.map((o, i) =>
      i === optIdx ? { ...o, text } : o
    );
    // Nếu đang chọn đáp án đúng là option này, cập nhật theo
    const newQ = { ...question, options: newOptions };
    if (question.correctAnswer === question.options[optIdx]?.text) {
      newQ.correctAnswer = text;
    }
    onChange(newQ);
  }

  function addOption() {
    onChange({
      ...question,
      options: [...question.options, { text: "", position: question.options.length }],
    });
  }

  function removeOption(optIdx: number) {
    const removed = question.options[optIdx];
    const newOptions = question.options
      .filter((_, i) => i !== optIdx)
      .map((o, i) => ({ ...o, position: i }));
    const newQ = { ...question, options: newOptions };
    if (removed?.text === question.correctAnswer) newQ.correctAnswer = "";
    onChange(newQ);
  }

  return (
    <article className="quiz-question-card draft-card">
      <div className="draft-card-header">
        <div className="draft-title">
          <span className="draft-number">{index + 1}</span>
          <strong>Câu hỏi</strong>
        </div>
        <button className="icon-button" type="button" onClick={onRemove} aria-label="Xóa câu hỏi" disabled={disabled}>
          ×
        </button>
      </div>

      <div className="quiz-question-body">
        {/* Question text */}
        <label>
          Nội dung câu hỏi <span className="required">*</span>
          <textarea
            rows={2}
            placeholder="Nhập câu hỏi..."
            value={question.questionText}
            onChange={(e) => setField("questionText", e.target.value)}
            disabled={disabled}
          />
        </label>

        {/* Question type */}
        <div className="quiz-type-row">
          <label>
            Loại câu hỏi
            <select
              value={question.questionType}
              onChange={(e) => {
                const t = e.target.value as QuestionType;
                const updated: QuizQuestion = { ...question, questionType: t, correctAnswer: "" };
                if (t === "multiple_choice" && updated.options.length < 2) {
                  updated.options = [
                    { text: "", position: 0 },
                    { text: "", position: 1 },
                  ];
                }
                onChange(updated);
              }}
              disabled={disabled}
            >
              <option value="multiple_choice">Trắc nghiệm</option>
              <option value="true_false">Đúng / Sai</option>
              <option value="written">Tự luận</option>
              <option value="paragraph">Đoạn văn</option>
              <option value="sorting">Sắp xếp</option>
            </select>
          </label>
        </div>

        {/* Options for multiple choice */}
        {question.questionType === "multiple_choice" && (
          <div className="quiz-options">
            <p className="quiz-options-label">Đáp án (chọn đáp án đúng)</p>
            {question.options.map((opt, optIdx) => (
              <div key={optIdx} className="quiz-option-row">
                <input
                  type="radio"
                  name={`correct-${question.key}`}
                  checked={question.correctAnswer === opt.text && opt.text !== ""}
                  onChange={() => setField("correctAnswer", opt.text)}
                  disabled={disabled || !opt.text}
                />
                <input
                  type="text"
                  placeholder={`Đáp án ${optIdx + 1}`}
                  value={opt.text}
                  onChange={(e) => setOptionText(optIdx, e.target.value)}
                  disabled={disabled}
                  className="quiz-option-input"
                />
                <button
                  type="button"
                  className="icon-button"
                  onClick={() => removeOption(optIdx)}
                  disabled={disabled || question.options.length <= 2}
                  aria-label="Xóa đáp án"
                >
                  ×
                </button>
              </div>
            ))}
            <button type="button" className="secondary-button" onClick={addOption} disabled={disabled}>
              + Thêm đáp án
            </button>
          </div>
        )}

        {/* True/False */}
        {question.questionType === "true_false" && (
          <label>
            Đáp án đúng
            <select
              value={question.correctAnswer}
              onChange={(e) => setField("correctAnswer", e.target.value)}
              disabled={disabled}
            >
              <option value="">-- Chọn đáp án --</option>
              <option value="true">Đúng</option>
              <option value="false">Sai</option>
            </select>
          </label>
        )}

        {/* Written */}
        {question.questionType === "written" && (
          <label>
            Đáp án đúng <span className="required">*</span>
            <input
              type="text"
              placeholder="Nhập đáp án..."
              value={question.correctAnswer}
              onChange={(e) => setField("correctAnswer", e.target.value)}
              disabled={disabled}
            />
          </label>
        )}

        {/* Paragraph */}
        {question.questionType === "paragraph" && (
          <>
            <label>
              Đoạn văn
              <textarea
                rows={4}
                placeholder="Nhập đoạn văn..."
                value={question.paragraphText ?? ""}
                onChange={(e) => setField("paragraphText", e.target.value)}
                disabled={disabled}
              />
            </label>
            <label>
              Đáp án đúng <span className="required">*</span>
              <input
                type="text"
                placeholder="Nhập đáp án cho câu hỏi con..."
                value={question.correctAnswer}
                onChange={(e) => setField("correctAnswer", e.target.value)}
                disabled={disabled}
              />
            </label>
          </>
        )}

        {/* Sorting */}
        {question.questionType === "sorting" && (
          <>
            <p className="quiz-options-label">Các phần tử (nhập theo thứ tự đúng)</p>
            {question.options.map((opt, optIdx) => (
              <div key={optIdx} className="quiz-option-row">
                <span style={{ width: 24, textAlign: "center", color: "var(--muted-foreground)" }}>{optIdx + 1}.</span>
                <input
                  type="text"
                  placeholder={`Phần tử ${optIdx + 1}`}
                  value={opt.text}
                  onChange={(e) => setOptionText(optIdx, e.target.value)}
                  disabled={disabled}
                  className="quiz-option-input"
                />
                <button
                  type="button"
                  className="icon-button"
                  onClick={() => removeOption(optIdx)}
                  disabled={disabled || question.options.length <= 2}
                  aria-label="Xóa phần tử"
                >
                  ×
                </button>
              </div>
            ))}
            <button type="button" className="secondary-button" onClick={addOption} disabled={disabled}>
              + Thêm phần tử
            </button>
            <label style={{ marginTop: 8 }}>
              Thứ tự đúng (cách nhau bằng dấu phẩy)
              <input
                type="text"
                placeholder="VD: B,A,D,C"
                value={question.correctAnswer}
                onChange={(e) => setField("correctAnswer", e.target.value)}
                disabled={disabled}
              />
            </label>
          </>
        )}

        {/* Audio URL */}
        <label>
          URL âm thanh (tùy chọn)
          <input
            type="url"
            placeholder="https://...mp3"
            value={question.audioUrl}
            onChange={(e) => setField("audioUrl", e.target.value)}
            disabled={disabled}
          />
        </label>

        {/* Explanation */}
        <label>
          Giải thích (tùy chọn)
          <textarea
            rows={2}
            placeholder="Giải thích tại sao đây là đáp án đúng..."
            value={question.answerExplanation}
            onChange={(e) => setField("answerExplanation", e.target.value)}
            disabled={disabled}
          />
        </label>
      </div>
    </article>
  );
}

export function QuizSetEditor({ existingSetId, onSave, onCancel }: Props) {
  const { token } = useAuth();
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [questions, setQuestions] = useState<QuizQuestion[]>([newQuestion(0)]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [termLanguage, setTermLanguage] = useState("en-US");
  const [definitionLanguage, setDefinitionLanguage] = useState("en-US");
  const [visibility, setVisibility] = useState("public");
  const [showImport, setShowImport] = useState(false);
  const [importFile, setImportFile] = useState<File | null>(null);
  const [importLoading, setImportLoading] = useState(false);
  const [importResult, setImportResult] = useState<{ imported: number; errors: Array<{ row: number; field: string; reason: string }> } | null>(null);

  function updateQuestion(key: string, updated: QuizQuestion) {
    setQuestions((prev) => prev.map((q) => (q.key === key ? updated : q)));
  }

  function removeQuestion(key: string) {
    setQuestions((prev) => {
      if (prev.length === 1) return prev;
      return prev.filter((q) => q.key !== key).map((q, i) => ({ ...q, position: i }));
    });
  }

  function addQuestion() {
    setQuestions((prev) => [...prev, newQuestion(prev.length)]);
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");

    if (!title.trim()) {
      setError("Vui lòng nhập tiêu đề quiz.");
      return;
    }
    if (questions.some((q) => !q.questionText.trim())) {
      setError("Tất cả câu hỏi cần có nội dung.");
      return;
    }
    if (questions.some((q) => !q.correctAnswer)) {
      setError("Tất cả câu hỏi cần có đáp án đúng.");
      return;
    }

    setLoading(true);
    try {
      const payload = {
        title: title.trim(),
        description: description.trim(),
        contentType: "quiz" as const,
        termLanguage,
        definitionLanguage,
        visibility,
        questions: questions.map((q, i) => ({
          questionText: q.questionText.trim(),
          questionType: q.questionType,
          correctAnswer: q.correctAnswer,
          answerExplanation: q.answerExplanation.trim() || undefined,
          paragraphText: q.paragraphText?.trim() || undefined,
          position: i,
          options:
            q.questionType === "multiple_choice" || q.questionType === "sorting"
              ? q.options
                  .filter((o) => o.text.trim())
                  .map((o, idx) => ({ text: o.text.trim(), position: idx }))
              : [],
        })),
      };
      await quizApi.create(token, payload);
      onSave();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Không thể tạo quiz. Vui lòng thử lại.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <form className="create-page" onSubmit={handleSubmit}>
      <section className="create-header">
        <div>
          <p className="eyebrow">Tạo bộ học mới</p>
          <h1>Tạo Quiz</h1>
        </div>
        <div className="header-actions">
          <button className="ghost-button" type="button" onClick={onCancel} disabled={loading}>
            Hủy
          </button>
          <button className="primary-button" type="submit" disabled={loading}>
            {loading ? "Đang lưu..." : "Tạo Quiz"}
          </button>
        </div>
      </section>        <section className="create-meta">
        <label>
          Tiêu đề <span className="required">*</span>
          <input
            autoFocus
            placeholder="Ví dụ: Kiểm tra Tiếng Anh Unit 1"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            disabled={loading}
          />
        </label>
        <label>
          Mô tả (tùy chọn)
          <textarea
            placeholder="Mô tả ngắn về bộ quiz..."
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            disabled={loading}
            rows={2}
          />
        </label>
        <div style={{ display: "flex", gap: 16 }}>
          <label style={{ flex: 1 }}>
            Ngôn ngữ thuật ngữ
            <select value={termLanguage} onChange={(e) => setTermLanguage(e.target.value)}>
              {LANGUAGES.map((l) => (
                <option key={l.code} value={l.code}>{l.flag} {l.name}</option>
              ))}
            </select>
          </label>
          <label style={{ flex: 1 }}>
            Ngôn ngữ đáp án
            <select value={definitionLanguage} onChange={(e) => setDefinitionLanguage(e.target.value)}>
              {LANGUAGES.map((l) => (
                <option key={l.code} value={l.code}>{l.flag} {l.name}</option>
              ))}
            </select>
          </label>
        </div>
        <label>
          Chế độ hiển thị
          <select value={visibility} onChange={(e) => setVisibility(e.target.value)}>
            <option value="public">Công khai</option>
            <option value="private">Riêng tư</option>
          </select>
        </label>
      </section>

      {error && <p className="message message--error">{error}</p>}

      {existingSetId && (
        <section style={{ padding: 16, border: "1px solid var(--border)", borderRadius: 8, marginBottom: 16 }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
            <strong>Nhập quiz từ Excel</strong>
            <button className="secondary-button" type="button" onClick={() => setShowImport(!showImport)}>
              {showImport ? "Đóng" : "Mở"}
            </button>
          </div>
          {showImport && (
            <div style={{ marginTop: 12 }}>
              <p style={{ fontSize: 14, color: "var(--muted-foreground)" }}>
                Chọn file .xlsx với cột: Question, Type, Option A-D, Correct Answer, Time (s), Audio URL, Answer Explanation
              </p>
              <input type="file" accept=".xlsx,.xls" onChange={(e) => setImportFile(e.target.files?.[0] ?? null)} />
              {importFile && (
                <button
                  className="primary-button"
                  style={{ marginTop: 8 }}
                  disabled={importLoading}
                  type="button"
                  onClick={async () => {
                    if (!importFile) return;
                    setImportLoading(true);
                    setImportResult(null);
                    try {
                      const result = await importApi.quiz(token, existingSetId, importFile);
                      setImportResult(result);
                      if (result.errors.length === 0) onSave();
                    } catch (err) {
                      setError(err instanceof Error ? err.message : "Import failed");
                    } finally {
                      setImportLoading(false);
                    }
                  }}
                >
                  {importLoading ? "Đang nhập..." : "Nhập dữ liệu"}
                </button>
              )}
              {importResult && (
                <div style={{ marginTop: 8 }}>
                  <p>Đã nhập: {importResult.imported} câu hỏi</p>
                  {importResult.errors.length > 0 && (
                    <div style={{ color: "var(--destructive)" }}>
                      <p>Lỗi:</p>
                      <ul>
                        {importResult.errors.map((e, i) => (
                          <li key={i}>Dòng {e.row}: [{e.field}] {e.reason}</li>
                        ))}
                      </ul>
                    </div>
                  )}
                </div>
              )}
            </div>
          )}
        </section>
      )}

      <section className="cards-editor">
        <div className="cards-editor-heading">
          <div>
            <p className="eyebrow">Câu hỏi</p>
            <h2>Danh sách câu hỏi ({questions.length})</h2>
          </div>
          <button className="secondary-button" type="button" onClick={addQuestion} disabled={loading}>
            + Thêm câu hỏi
          </button>
        </div>

        {questions.map((q, idx) => (
          <QuestionCard
            key={q.key}
            question={q}
            index={idx}
            onChange={(updated) => updateQuestion(q.key, updated)}
            onRemove={() => removeQuestion(q.key)}
            disabled={loading}
          />
        ))}

        <button className="add-row" type="button" onClick={addQuestion} disabled={loading}>
          + Thêm một câu hỏi nữa
        </button>
      </section>
    </form>
  );
}
