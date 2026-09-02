// CreateTypeSelector — màn hình chọn loại bộ học mới
// Giống hquizlet/apps/nextjs/src/app/create-set/page.tsx

import React from "react";

type Props = {
  onSelect: (type: "flashcard" | "quiz" | "grammar") => void;
  onCancel: () => void;
};

const TYPES = [
  {
    key: "flashcard" as const,
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.8} className="w-8 h-8">
        <rect x="2" y="5" width="20" height="14" rx="2" />
        <path d="M2 10h20" />
      </svg>
    ),
    title: "Flashcards",
    description: "Tạo thẻ từ vựng truyền thống với thuật ngữ và định nghĩa để ghi nhớ.",
  },
  {
    key: "quiz" as const,
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.8} className="w-8 h-8">
        <rect x="3" y="3" width="18" height="18" rx="2" />
        <path d="M9 9h6M9 13h4" />
        <circle cx="7" cy="9" r="1" fill="currentColor" />
        <circle cx="7" cy="13" r="1" fill="currentColor" />
        <circle cx="7" cy="17" r="1" fill="currentColor" />
      </svg>
    ),
    title: "Quiz",
    description: "Tạo câu hỏi trắc nghiệm, đúng/sai và tự luận với đáp án và giải thích.",
  },
  {
    key: "grammar" as const,
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.8} className="w-8 h-8">
        <path d="M4 19V5h16v14" />
        <path d="M4 5h16" />
        <path d="M9 9h6M9 13h4M9 17h6" />
      </svg>
    ),
    title: "Ngữ pháp",
    description: "Tạo bộ ngữ pháp với ý nghĩa, cách dùng và ví dụ minh hoạ chi tiết.",
  },
];

export function CreateTypeSelector({ onSelect, onCancel }: Props) {
  return (
    <div className="create-type-selector">
      <div className="create-type-header">
        <div>
          <h1>Tạo bộ học mới</h1>
          <p className="create-type-subtitle">Chọn loại nội dung bạn muốn tạo.</p>
        </div>
        <button className="ghost-button" type="button" onClick={onCancel}>
          Hủy
        </button>
      </div>

      <div className="create-type-grid">
        {TYPES.map((t) => (
          <button
            key={t.key}
            className="create-type-card"
            onClick={() => onSelect(t.key)}
            type="button"
          >
            <div className="create-type-icon">{t.icon}</div>
            <strong className="create-type-title">{t.title}</strong>
            <p className="create-type-desc">{t.description}</p>
          </button>
        ))}
      </div>
    </div>
  );
}
