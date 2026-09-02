// QuizSetEditor — tạo/sửa quiz set (multiple choice, true/false, written)
// Tham chiếu: hquizlet/apps/nextjs/src/components/study-set/quiz-set-form.tsx

import React, { useState } from "react";
import { useAuth } from "../auth/AuthContext";
import { quizApi } from "../../lib/api";

type QuestionType = "multiple_choice" | "true_false" | "written";

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
  options: QuizOption[];
  position: number;
}

type Props = {
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

export function QuizSetEditor({ onSave, onCancel }: Props) {
  const { token } = useAuth();
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [questions, setQuestions] = useState<QuizQuestion[]>([newQuestion(0)]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

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
        questions: questions.map((q, i) => ({
          questionText: q.questionText.trim(),
          questionType: q.questionType,
          correctAnswer: q.correctAnswer,
          answerExplanation: q.answerExplanation.trim() || undefined,
          position: i,
          options:
            q.questionType === "multiple_choice"
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
      </section>

      <section className="create-meta">
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
      </section>

      {error && <p className="message message--error">{error}</p>}

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
