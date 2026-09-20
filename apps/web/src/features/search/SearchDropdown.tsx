// SearchDropdown — kết quả tìm kiếm global từ Navbar
import React from "react";
import { studySetApi } from "../../lib/api/client";
import type { StudySet } from "../../types";

type Props = {
  query: string;
  token: string;
  onSelect: (set: StudySet) => void;
  onClose: () => void;
};

type SearchState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "done"; results: StudySet[] }
  | { status: "error"; message: string };

export function SearchDropdown({ query, token, onSelect, onClose }: Props) {
  const [state, setState] = React.useState<SearchState>({ status: "idle" });
  const abortRef = React.useRef<AbortController | null>(null);

  React.useEffect(() => {
    if (!query.trim()) {
      setState({ status: "idle" });
      return;
    }

    // Debounce 300ms
    const timer = window.setTimeout(async () => {
      abortRef.current?.abort();
      const controller = new AbortController();
      abortRef.current = controller;
      setState({ status: "loading" });
      try {
        const result = await studySetApi.list(token, { search: query, per_page: 8 }, controller.signal);
        if (!controller.signal.aborted) {
          setState({ status: "done", results: result.items });
        }
      } catch (e) {
        if (!controller.signal.aborted) {
          setState({ status: "error", message: e instanceof Error ? e.message : "Lỗi tìm kiếm" });
        }
      }
    }, 300);

    return () => {
      window.clearTimeout(timer);
      abortRef.current?.abort();
    };
  }, [query, token]);

  // Close on Escape
  React.useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  if (!query.trim()) return null;

  return (
    <div className="ql-search-dropdown" role="listbox" aria-label="Kết quả tìm kiếm">
      {state.status === "loading" && (
        <div className="ql-search-dropdown-loading">
          <div className="ql-spinner-sm" />
          Đang tìm…
        </div>
      )}

      {state.status === "error" && (
        <div className="ql-search-dropdown-empty">{state.message}</div>
      )}

      {state.status === "done" && state.results.length === 0 && (
        <div className="ql-search-dropdown-empty">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
            <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
          </svg>
          Không tìm thấy kết quả cho "<strong>{query}</strong>"
        </div>
      )}

      {state.status === "done" && state.results.length > 0 && (
        <>
          <div className="ql-search-dropdown-label">Học phần</div>
          {state.results.map((set) => (
            <button
              key={set.id}
              type="button"
              className="ql-search-result-item"
              role="option"
              onClick={() => { onSelect(set); onClose(); }}
            >
              <div className="ql-search-result-icon" aria-hidden="true">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <rect x="2" y="5" width="20" height="14" rx="2"/><path d="M2 10h20"/>
                </svg>
              </div>
              <div className="ql-search-result-content">
                <span className="ql-search-result-title">{set.title}</span>
                <span className="ql-search-result-meta">
                  {set.flashcardCount ?? set.flashcards?.length ?? 0} thẻ
                  {set.visibility === "public" && " · Công khai"}
                </span>
              </div>
            </button>
          ))}
        </>
      )}
    </div>
  );
}
