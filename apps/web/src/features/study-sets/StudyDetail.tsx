// StudyDetail — header + chế độ học + inline edit flashcard

import React from "react";
import type { StudySet, Flashcard } from "../../types";
import type { LearningMode } from "../learning/types";
import { LearningContainer } from "../learning";
import { useAuth } from "../auth/AuthContext";
import { flashcardApi } from "../../lib/api";
import { StudyModes } from "./StudyModes";
import { FlashcardListCard } from "./FlashcardListCard";
import { FlashcardSearchBar } from "./FlashcardSearchBar";
import { InlineFlashcardEditor } from "./InlineFlashcardEditor";
import "./StudyDetail.css";

type Props = {
  set: StudySet;
  onEdit: () => void;
  onDelete: () => void;
  onBack: () => void;
  onToggleStar?: (card: Flashcard) => void;
};

const CARD_PAGE_SIZE_OPTIONS = [25, 50, 100];

export function StudyDetail({ set, onEdit, onDelete, onBack, onToggleStar }: Props) {
  const { token } = useAuth();
  const [studyMode, setStudyMode] = React.useState<LearningMode>("flashcards");
  const [sortOrder, setSortOrder] = React.useState<"original" | "alphabetical">("original");
  const [cardPageSize, setCardPageSize] = React.useState(50);
  const [menuOpen, setMenuOpen] = React.useState(false);
  const menuRef = React.useRef<HTMLDivElement>(null);

  const [pagedCards, setPagedCards] = React.useState<Flashcard[]>(set.flashcards ?? []);
  const [cardPage, setCardPage] = React.useState(1);
  const [cardTotal, setCardTotal] = React.useState(set.flashcardCount ?? (set.flashcards?.length ?? 0));
  const [cardTotalPages, setCardTotalPages] = React.useState(
    Math.max(1, Math.ceil((set.flashcardCount ?? (set.flashcards?.length ?? 0)) / 50)),
  );
  const [cardLoading, setCardLoading] = React.useState(false);
  const [cardError, setCardError] = React.useState("");

  // Search state
  const [searchQuery, setSearchQuery] = React.useState("");

  // Inline edit state
  const [editingCardId, setEditingCardId] = React.useState<number | null>(null);
  const [addingCard, setAddingCard] = React.useState(false);
  const [deleteConfirmId, setDeleteConfirmId] = React.useState<number | null>(null);
  const [actionError, setActionError] = React.useState<string | null>(null);
  const [actionToast, setActionToast] = React.useState<string | null>(null);

  const loadCards = React.useCallback(async (page: number, perPage = cardPageSize) => {
    setCardLoading(true);
    setCardError("");
    try {
      const result = await flashcardApi.listPaged(token, set.id, page, perPage);
      setPagedCards(result.items);
      setCardPage(result.page);
      setCardTotal(result.total);
      setCardTotalPages(result.totalPages);
    } catch (err) {
      setCardError(err instanceof Error ? err.message : "Không tải được thẻ.");
    } finally {
      setCardLoading(false);
    }
  }, [token, set.id, cardPageSize]);

  React.useEffect(() => {
    setCardPage(1);
    void loadCards(1, cardPageSize);
  }, [set.id, sortOrder, cardPageSize, loadCards]);

  const handleCardPage = (next: number) => {
    if (next === cardPage) return;
    void loadCards(next);
  };

  React.useEffect(() => {
    if (!menuOpen) return;
    const handler = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [menuOpen]);

  function showToast(msg: string) {
    setActionToast(msg);
    window.setTimeout(() => setActionToast(null), 3000);
  }

  // Filter & sort
  const filteredCards = React.useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    const base = q
      ? pagedCards.filter(
          (c) => c.term.toLowerCase().includes(q) || c.definition.toLowerCase().includes(q),
        )
      : pagedCards;
    if (sortOrder === "alphabetical") {
      return [...base].sort((a, b) => a.term.localeCompare(b.term));
    }
    return base;
  }, [pagedCards, searchQuery, sortOrder]);

  const cardRangeStart = cardTotal === 0 ? 0 : (cardPage - 1) * cardPageSize + 1;
  const cardRangeEnd = cardTotal === 0 ? 0 : Math.min(cardPage * cardPageSize, cardTotal);

  const totalItems =
    set.contentType === "quiz" ? (set.quizQuestions?.length ?? 0) : cardTotal;

  const isFlashcardSet = set.contentType === "flashcard" || !set.contentType;

  /* ── Inline edit handlers ── */
  async function handleSaveEdit(card: Flashcard, term: string, definition: string) {
    setActionError(null);
    await flashcardApi.update(token, card.id, { term, definition });
    // Optimistic update
    setPagedCards((prev) =>
      prev.map((c) => (c.id === card.id ? { ...c, term, definition } : c)),
    );
    setEditingCardId(null);
    showToast("Đã cập nhật thẻ");
  }

  async function handleDelete(card: Flashcard) {
    setActionError(null);
    try {
      await flashcardApi.delete(token, card.id);
      setPagedCards((prev) => prev.filter((c) => c.id !== card.id));
      setCardTotal((t) => t - 1);
      setDeleteConfirmId(null);
      showToast("Đã xóa thẻ");
    } catch (e) {
      setActionError(e instanceof Error ? e.message : "Không thể xóa thẻ");
      setDeleteConfirmId(null);
    }
  }

  async function handleAddCard(term: string, definition: string) {
    setActionError(null);
    const newCard = await flashcardApi.add(token, set.id, { term, definition });
    setPagedCards((prev) => [...prev, newCard]);
    setCardTotal((t) => t + 1);
    setAddingCard(false);
    showToast("Đã thêm thẻ mới");
  }

  return (
    <div className="sd-page">
      {/* Toast notification */}
      {actionToast && (
        <div className="sd-toast" role="status" aria-live="polite">{actionToast}</div>
      )}

      <div className="sd-header">
        <button type="button" className="sd-back" onClick={onBack}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"><path d="M19 12H5"/><path d="M12 19l-7-7 7-7"/></svg>
          Quay lại
        </button>

        <div className="sd-title-row">
          <div className="sd-title-copy">
            <div className="sd-eyebrow">
              {set.visibility === "private" && <span className="sd-private-badge">🔒 Riêng tư</span>}
              <span>
                {set.contentType === "quiz" ? "Quiz" : set.contentType === "grammar" ? "Ngữ pháp" : "Học phần"}
              </span>
              <span className="sd-dot">·</span>
              <span>{totalItems} thuật ngữ</span>
            </div>
            <h1 className="sd-title">{set.title}</h1>
            {set.description && <p className="sd-description">{set.description}</p>}
          </div>

          <div className="sd-more-wrap" ref={menuRef}>
            <button
              type="button"
              className="sd-more-btn"
              onClick={() => setMenuOpen((v) => !v)}
              aria-label="Xem thêm"
              title="Xem thêm"
            >
              <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor">
                <circle cx="5" cy="12" r="2"/><circle cx="12" cy="12" r="2"/><circle cx="19" cy="12" r="2"/>
              </svg>
            </button>

            {menuOpen && (
              <div className="sd-dropdown">
                <button
                  type="button"
                  className="sd-dropdown-item"
                  onClick={() => { setMenuOpen(false); onEdit(); }}
                >
                  Sửa
                </button>
                <div className="sd-dropdown-divider" />
                <button
                  type="button"
                  className="sd-dropdown-item sd-dropdown-item--danger"
                  onClick={() => { setMenuOpen(false); onDelete(); }}
                >
                  Xóa
                </button>
              </div>
            )}
          </div>
        </div>

        {isFlashcardSet && (
          <StudyModes activeMode={studyMode} onSelectMode={setStudyMode} />
        )}
      </div>

      {isFlashcardSet && (
        <div className="sd-learning-wrapper">
          <LearningContainer set={set} mode={studyMode} />
        </div>
      )}

      {isFlashcardSet && studyMode === "flashcards" && (
        <div className="sd-termlist sd-termlist--below-game">
          <div className="sd-termlist-header">
            <h2 className="sd-termlist-title">
              Thuật ngữ trong học phần này
              <span className="sd-termlist-count">({cardTotal})</span>
            </h2>
            <div className="sd-termlist-controls">
              {cardTotal > 0 && (
                <span className="sd-page-range">
                  {cardRangeStart}-{cardRangeEnd} / {cardTotal}
                </span>
              )}
              <select
                className="sd-sort-select"
                value={cardPageSize}
                aria-label="Số thẻ mỗi trang"
                onChange={(e) => setCardPageSize(Number(e.target.value))}
              >
                {CARD_PAGE_SIZE_OPTIONS.map((size) => (
                  <option key={size} value={size}>{size}/trang</option>
                ))}
              </select>
              <select
                className="sd-sort-select"
                value={sortOrder}
                onChange={(e) => setSortOrder(e.target.value as "original" | "alphabetical")}
                disabled={cardLoading}
              >
                <option value="original">Thứ tự gốc</option>
                <option value="alphabetical">A → Z</option>
              </select>
            </div>
          </div>

          {/* Search bar */}
          <FlashcardSearchBar
            value={searchQuery}
            resultCount={filteredCards.length}
            totalCount={pagedCards.length}
            onChange={setSearchQuery}
            onClear={() => setSearchQuery("")}
          />

          {actionError && (
            <div className="sd-action-error" role="alert">{actionError}</div>
          )}

          {cardError && (
            <div className="sd-empty">
              <p>{cardError}</p>
              <button type="button" className="ghost-button" onClick={() => void loadCards(cardPage)}>Thử lại</button>
            </div>
          )}

          {!cardError && pagedCards.length === 0 && !cardLoading && (
            <div className="sd-empty"><p>Chưa có thẻ học nào.</p></div>
          )}

          {cardLoading && (
            <div className="loading-skeleton" aria-busy="true">
              {[1, 2, 3].map((i) => <div key={i} className="skeleton-row" />)}
            </div>
          )}

          {!cardLoading && !cardError && (
            <div className="sd-fc-list">
              {filteredCards.map((card) => (
                <div key={card.id} className="sd-fc-row-wrap">
                  {/* Delete confirmation */}
                  {deleteConfirmId === card.id ? (
                    <div className="ql-delete-confirm" role="alertdialog" aria-label="Xác nhận xóa thẻ">
                      <p>Xóa thẻ "<strong>{card.term}</strong>"?</p>
                      <div className="ql-delete-confirm-actions">
                        <button type="button" className="secondary-button" onClick={() => setDeleteConfirmId(null)}>
                          Hủy
                        </button>
                        <button type="button" className="danger-button" onClick={() => void handleDelete(card)}>
                          Xóa
                        </button>
                      </div>
                    </div>
                  ) : editingCardId === card.id ? (
                    /* Inline edit mode */
                    <InlineFlashcardEditor
                      mode="edit"
                      initialTerm={card.term}
                      initialDefinition={card.definition}
                      onSave={(term, def) => handleSaveEdit(card, term, def)}
                      onCancel={() => setEditingCardId(null)}
                    />
                  ) : (
                    /* Normal display */
                    <article className="sd-fc-row ql-card-row">
                      <div className="sd-fc-row-main">
                        {card.imageUrl && (
                          <div className="sd-fc-row-thumb">
                            <img src={card.imageUrl} alt="" />
                          </div>
                        )}
                        <div className="sd-fc-row-term">{card.term}</div>
                        <div className="sd-fc-row-sep" aria-hidden />
                        <div className="sd-fc-row-def">{card.definition}</div>
                        <div className="sd-fc-row-actions">
                          {onToggleStar && (
                            <button
                              type="button"
                              className={`sd-star-btn ${card.starred ? "starred" : ""}`}
                              onClick={() => onToggleStar(card)}
                              title={card.starred ? "Bỏ đánh dấu" : "Đánh dấu"}
                            >
                              {card.starred ? "★" : "☆"}
                            </button>
                          )}
                          <button
                            type="button"
                            className="sd-edit-btn"
                            onClick={() => { setEditingCardId(card.id); setAddingCard(false); }}
                            title="Sửa thẻ"
                            aria-label={`Sửa thẻ "${card.term}"`}
                          >
                            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                              <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
                              <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
                            </svg>
                          </button>
                          <button
                            type="button"
                            className="sd-delete-btn"
                            onClick={() => setDeleteConfirmId(card.id)}
                            title="Xóa thẻ"
                            aria-label={`Xóa thẻ "${card.term}"`}
                          >
                            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                              <polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/>
                              <path d="M10 11v6M14 11v6"/><path d="M9 6V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/>
                            </svg>
                          </button>
                        </div>
                      </div>
                    </article>
                  )}
                </div>
              ))}

              {searchQuery && filteredCards.length === 0 && (
                <div className="sd-empty">
                  <p>Không tìm thấy thẻ nào khớp với "<strong>{searchQuery}</strong>"</p>
                  <button type="button" className="ghost-button" onClick={() => setSearchQuery("")}>Xóa tìm kiếm</button>
                </div>
              )}

              {/* Add new card */}
              {addingCard ? (
                <InlineFlashcardEditor
                  mode="add"
                  onSave={handleAddCard}
                  onCancel={() => setAddingCard(false)}
                />
              ) : (
                <button
                  type="button"
                  id="add-card-btn"
                  className="sd-add-card-btn"
                  onClick={() => { setAddingCard(true); setEditingCardId(null); }}
                >
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round">
                    <path d="M12 5v14M5 12h14"/>
                  </svg>
                  Thêm thẻ mới
                </button>
              )}
            </div>
          )}

          {cardTotalPages > 1 && (
            <div className="sd-pagination" aria-label="Phân trang thuật ngữ">
              <button type="button" className="sd-page-btn" disabled={cardPage <= 1 || cardLoading} onClick={() => handleCardPage(cardPage - 1)} aria-label="Trang trước">‹</button>
              <span className="sd-page-info">Trang {cardPage} / {cardTotalPages}<span className="sd-page-total"> · {cardTotal} thẻ</span></span>
              <button type="button" className="sd-page-btn" disabled={cardPage >= cardTotalPages || cardLoading} onClick={() => handleCardPage(cardPage + 1)} aria-label="Trang sau">›</button>
            </div>
          )}
        </div>
      )}

      {set.contentType === "quiz" && set.quizQuestions && set.quizQuestions.length > 0 && (
        <div className="sd-termlist">
          <div className="sd-termlist-header">
            <h2 className="sd-termlist-title">Câu hỏi Quiz<span className="sd-termlist-count">({set.quizQuestions.length})</span></h2>
          </div>
          <div className="sd-cards">
            {set.quizQuestions.map((q, i) => (
              <article className="sd-card" key={q.id ?? i}>
                <div className="sd-card-body">
                  <div className="sd-card-term">{i + 1}. {q.questionText}</div>
                </div>
              </article>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
