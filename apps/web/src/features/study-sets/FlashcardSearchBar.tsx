// FlashcardSearchBar — thanh tìm kiếm flashcard trong study set detail
import React from "react";
import "../study-sets/StudyDetail.css";

type Props = {
  value: string;
  resultCount: number;
  totalCount: number;
  onChange: (v: string) => void;
  onClear: () => void;
};

export function FlashcardSearchBar({ value, resultCount, totalCount, onChange, onClear }: Props) {
  const inputRef = React.useRef<HTMLInputElement>(null);

  // Global "/" shortcut focuses search
  React.useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "/" && !(e.target instanceof HTMLInputElement) && !(e.target instanceof HTMLTextAreaElement)) {
        e.preventDefault();
        inputRef.current?.focus();
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  return (
    <div className="ql-search-bar">
      <div className="ql-search-bar-wrap">
        <svg className="ql-search-bar-ico" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
        <input
          ref={inputRef}
          id="flashcard-search-input"
          type="search"
          className="ql-search-bar-input"
          placeholder='Tìm thẻ… (Nhấn "/" để focus)'
          value={value}
          onChange={(e) => onChange(e.target.value)}
          autoComplete="off"
        />
        {value && (
          <button
            type="button"
            className="ql-search-bar-clear"
            onClick={() => { onClear(); inputRef.current?.focus(); }}
            aria-label="Xóa tìm kiếm"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
              <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
          </button>
        )}
      </div>
      {value && (
        <span className="ql-search-bar-count">
          {resultCount} / {totalCount} thẻ
        </span>
      )}
    </div>
  );
}
