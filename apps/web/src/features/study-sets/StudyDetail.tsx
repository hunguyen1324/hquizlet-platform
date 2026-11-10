// StudyDetail — header + chế độ học + inline edit flashcard

import React from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import type { StudySet, Flashcard } from "../../types";
import type { LearningMode } from "../learning/types";
import type { LearningProgress } from "../../lib/api/client";
import { LearningContainer, fetchLatestProgress } from "../learning";
import { useAuth } from "../auth/AuthContext";
import { flashcardApi, folderApi, studySetApi } from "../../lib/api";
import type { FolderSummary } from "../../lib/api";
import { StudyModes } from "./StudyModes";
import { FlashcardListCard } from "./FlashcardListCard";
import { FlashcardSearchBar } from "./FlashcardSearchBar";
import { InlineFlashcardEditor } from "./InlineFlashcardEditor";
import { QuizDetailOverview } from "./QuizDetailOverview";
import "./StudyDetail.css";
import { StudyActionIcon } from "./StudyActionIcon";

type Props = {
  set: StudySet;
  onEdit: () => void;
  onDelete: () => void;
  onBack: () => void;
  onToggleStar?: (card: Flashcard) => void;
  resumeSessionId?: string;
  onManageSessions?: (setId: number) => void;
};

const CARD_PAGE_SIZE = 50;

export function StudyDetail({ set, onEdit, onDelete, onBack, onToggleStar, onManageSessions, resumeSessionId }: Props) {
  const { token } = useAuth();
  const queryClient = useQueryClient();
  const [studyMode, setStudyMode] = React.useState<LearningMode>("flashcards");
  const [modeProgress, setModeProgress] = React.useState<Partial<Record<LearningMode, LearningProgress>>>({});
  const learningRef = React.useRef<HTMLDivElement>(null);
  const [sortOrder, setSortOrder] = React.useState<"original" | "alphabetical">("original");
  const [menuOpen, setMenuOpen] = React.useState(false);
  const [visibility, setVisibility] = React.useState(set.visibility);
  const [folders, setFolders] = React.useState<FolderSummary[]>([]);
  const [folderDialogOpen, setFolderDialogOpen] = React.useState(false);
  const menuRef = React.useRef<HTMLDivElement>(null);

  // Cards accumulate only when the learner requests another page.
  const [allCards, setAllCards] = React.useState<Flashcard[]>((set.flashcards ?? []).slice(0, CARD_PAGE_SIZE));
  const [cardPage, setCardPage] = React.useState(1);
  const [cardTotal, setCardTotal] = React.useState(set.flashcardCount ?? (set.flashcards?.length ?? 0));
  const [cardTotalPages, setCardTotalPages] = React.useState(
    Math.max(1, Math.ceil((set.flashcardCount ?? (set.flashcards?.length ?? 0)) / CARD_PAGE_SIZE)),
  );
  const [cardLoading, setCardLoading] = React.useState(false);
  const [cardError, setCardError] = React.useState("");
  const cardRequest = React.useRef(0);
  const cardLoadingRef = React.useRef(false);
  const hasMore = cardPage < cardTotalPages;

  // Search state
  const [searchQuery, setSearchQuery] = React.useState("");


  // Inline edit state
  const [editingCardId, setEditingCardId] = React.useState<number | null>(null);
  const [addingCard, setAddingCard] = React.useState(false);
  const [deleteConfirmId, setDeleteConfirmId] = React.useState<number | null>(null);
  const [actionError, setActionError] = React.useState<string | null>(null);
  const [actionToast, setActionToast] = React.useState<string | null>(null);

  const loadNextPage = React.useCallback(async (page: number, reset = false) => {
    if (cardLoadingRef.current) return;
    cardLoadingRef.current = true;
    const request = ++cardRequest.current;
    setCardLoading(true);
    setCardError("");
    try {
      const result = await flashcardApi.listPaged(token, set.id, page, CARD_PAGE_SIZE);
      if (request !== cardRequest.current) return;
      setAllCards((prev) => reset ? result.items : [...prev, ...result.items.filter((r) => !prev.some((p) => p.id === r.id))]);
      setCardPage(result.page);
      setCardTotal(result.total);
      setCardTotalPages(result.totalPages);
    } catch (err) {
      if (request !== cardRequest.current) return;
      setCardError(err instanceof Error ? err.message : "Không tải được thẻ.");
    } finally {
      if (request === cardRequest.current) {
        cardLoadingRef.current = false;
        setCardLoading(false);
      }
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token, set.id]);

  React.useEffect(() => {
    cardRequest.current++;
    cardLoadingRef.current = false;
    setCardLoading(false);
    setCardError("");
    setSearchQuery("");
    setCardPage(1);
    const total = set.flashcardCount ?? set.flashcards?.length ?? 0;
    setCardTotal(total);
    setCardTotalPages(Math.max(1, Math.ceil(total / CARD_PAGE_SIZE)));
    const initial = (set.flashcards ?? []).slice(0, CARD_PAGE_SIZE);
    setAllCards(initial);
    if (set.contentType !== "quiz" && set.contentType !== "grammar" && (set.flashcards === undefined || initial.length < Math.min(CARD_PAGE_SIZE, total))) {
      void loadNextPage(1, true);
    }
    return () => { cardRequest.current++; cardLoadingRef.current = false; };
  }, [set.id, token, loadNextPage]);

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

  async function toggleVisibility() {
    const next = visibility === "public" ? "private" : "public";
    try {
      await studySetApi.update(token, set.id, { title: set.title, description: set.description, termLanguage: set.termLanguage, definitionLanguage: set.definitionLanguage, visibility: next });
      setVisibility(next);
      setMenuOpen(false);
      showToast(next === "public" ? "Quiz hiện được xem công khai" : "Quiz đã chuyển sang riêng tư");
    } catch (error) { setActionError(error instanceof Error ? error.message : "Không đổi được quyền xem"); }
  }

  async function openFolderDialog() {
    try { const result = await folderApi.listFolders(token, { per_page: 100 }); setFolders(result.items); setFolderDialogOpen(true); setMenuOpen(false); }
    catch (error) { setActionError(error instanceof Error ? error.message : "Không tải được thư mục"); }
  }

  async function addToFolder(folderId: number) {
    try { await folderApi.addStudySetToFolder(token, folderId, set.id); setFolderDialogOpen(false); showToast("Đã thêm quiz vào thư mục"); }
    catch (error) { setActionError(error instanceof Error ? error.message : "Không thêm được vào thư mục"); }
  }

  // Filter & sort (client-side on loaded cards)
  const filteredCards = React.useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    const base = q
      ? allCards.filter(
          (c) => c.term.toLowerCase().includes(q) || c.definition.toLowerCase().includes(q),
        )
      : allCards;
    if (sortOrder === "alphabetical") {
      return [...base].sort((a, b) => a.term.localeCompare(b.term));
    }
    return base;
  }, [allCards, searchQuery, sortOrder]);

  const totalItems =
    set.contentType === "quiz" ? set.flashcardCount : cardTotal;

  const isFlashcardSet = set.contentType === "flashcard" || !set.contentType;

  // Last saved session per mode powers the "how far did I get" hints on the mode hub.
  React.useEffect(() => {
    if (!token || !isFlashcardSet) return;
    let active = true;
    fetchLatestProgress(token, set.id)
      .then((latest) => { if (active) setModeProgress(latest); })
      .catch(() => { if (active) setModeProgress({}); });
    return () => { active = false; };
  }, [token, set.id, isFlashcardSet]);

  // Most recently finished mode, so "Tiếp tục học" resumes where the learner stopped.
  const resumeMode = React.useMemo<LearningMode | null>(() => {
    let best: { mode: LearningMode; at: number } | null = null;
    for (const [mode, entry] of Object.entries(modeProgress) as [LearningMode, LearningProgress][]) {
      const stamp = Date.parse(entry?.completedAt ?? entry?.createdAt ?? "");
      if (Number.isNaN(stamp)) continue;
      if (!best || stamp > best.at) best = { mode, at: stamp };
    }
    return best?.mode ?? null;
  }, [modeProgress]);

  function handleContinueLearning() {
    if (resumeMode && resumeMode !== studyMode) setStudyMode(resumeMode);
    learningRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
  }

  /* ── Inline edit handlers ── */
  const updateMutation = useMutation({
    mutationFn: ({ cardId, term, definition }: { cardId: number, term: string, definition: string }) => flashcardApi.update(token, cardId, { term, definition }),
    onSuccess: (_, { cardId, term, definition }) => {
      setAllCards((prev) => prev.map((c) => (c.id === cardId ? { ...c, term, definition } : c)));
      queryClient.invalidateQueries({ queryKey: ["studySets"] });
      setEditingCardId(null);
      showToast("Đã cập nhật thẻ");
    },
    onError: (e) => setActionError(e instanceof Error ? e.message : "Không thể cập nhật thẻ"),
  });

  function handleSaveEdit(card: Flashcard, term: string, definition: string) {
    setActionError(null);
    updateMutation.mutate({ cardId: card.id, term, definition });
  }

  const deleteMutation = useMutation({
    mutationFn: (cardId: number) => flashcardApi.delete(token, cardId),
    onSuccess: (_, cardId) => {
      setAllCards((prev) => prev.filter((c) => c.id !== cardId));
      setCardTotal((t) => t - 1);
      queryClient.invalidateQueries({ queryKey: ["studySets"] });
      setDeleteConfirmId(null);
      showToast("Đã xóa thẻ");
    },
    onError: (e) => {
      setActionError(e instanceof Error ? e.message : "Không thể xóa thẻ");
      setDeleteConfirmId(null);
    }
  });

  function handleDelete(card: Flashcard) {
    setActionError(null);
    deleteMutation.mutate(card.id);
  }

  const addMutation = useMutation({
    mutationFn: ({ term, definition }: { term: string, definition: string }) => flashcardApi.add(token, set.id, { term, definition }),
    onSuccess: (newCard) => {
      setAllCards((prev) => [...prev, newCard]);
      setCardTotal((t) => t + 1);
      queryClient.invalidateQueries({ queryKey: ["studySets"] });
      setAddingCard(false);
      showToast("Đã thêm thẻ mới");
    },
    onError: (e) => setActionError(e instanceof Error ? e.message : "Không thể thêm thẻ")
  });

  function handleAddCard(term: string, definition: string) {
    setActionError(null);
    addMutation.mutate({ term, definition });
  }

  return (
    <div className={`sd-page${set.contentType === "quiz" ? " sd-page--quiz" : ""}`}>
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
              {visibility === "private" ? <span className="sd-private-badge">🔒 Riêng tư</span> : <span className="sd-public-badge">🌐 Công khai</span>}
              <span>
                {set.contentType === "quiz" ? "Quiz" : set.contentType === "grammar" ? "Ngữ pháp" : "Học phần"}
              </span>
              <span className="sd-dot">·</span>
              <span>{totalItems} {set.contentType === "quiz" ? "câu hỏi" : "thuật ngữ"}</span>
            </div>
            <h1 className="sd-title">{set.title}</h1>
            {set.description && <p className="sd-description">{set.description}</p>}
          </div>

          <div className="sd-more-wrap" ref={menuRef}>
            <button
              type="button"
              className="sd-more-btn"
              onClick={() => setMenuOpen((v) => !v)}
              aria-expanded={menuOpen}
              aria-label="Tùy chọn học phần"
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
                  <StudyActionIcon name="edit" /><span>Sửa</span>
                </button>
                <button type="button" className="sd-dropdown-item" onClick={() => void openFolderDialog()}><StudyActionIcon name="folder" /><span>Thêm vào thư mục</span></button>
                <button type="button" className="sd-dropdown-item" onClick={() => void toggleVisibility()}><StudyActionIcon name={visibility === "public" ? "lock" : "globe"} /><span>{visibility === "public" ? "Chuyển sang riêng tư" : "Chuyển sang công khai"}</span></button>
                <div className="sd-dropdown-divider" />
                <button
                  type="button"
                  className="sd-dropdown-item sd-dropdown-item--danger"
                  onClick={() => { setMenuOpen(false); onDelete(); }}
                >
                  <StudyActionIcon name="trash" /><span>Xóa</span>
                </button>
              </div>
            )}
          </div>
        </div>

        {isFlashcardSet && (
          <StudyModes
            activeMode={studyMode}
            onSelectMode={setStudyMode}
            progress={modeProgress}
            onContinue={handleContinueLearning}
          />
        )}
      </div>

      {isFlashcardSet && (
        <div className="sd-learning-wrapper" ref={learningRef}>
          {/* Use loaded pages and the server total for learning progress. */}
          <LearningContainer set={set} mode={studyMode} cards={allCards} totalCount={cardTotal} onExit={() => setStudyMode("flashcards")} />
        </div>
      )}

      {isFlashcardSet && studyMode === "flashcards" && (
        <div className="sd-termlist sd-termlist--below-game">
          <div className="sd-termlist-header">
            <h2 className="sd-termlist-title">
              Thuật ngữ trong học phần này
              <span className="sd-termlist-count">
                ({allCards.length < cardTotal ? `${allCards.length} / ${cardTotal}` : cardTotal})
              </span>
            </h2>
            <div className="sd-termlist-controls">

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

          {cardLoading && allCards.length === 0 && <p role="status">Đang tải thẻ…</p>}

          {/* Search bar */}
          <FlashcardSearchBar
            value={searchQuery}
            resultCount={filteredCards.length}
            totalCount={allCards.length}
            onChange={setSearchQuery}
            onClear={() => setSearchQuery("")}
          />

          {actionError && (
            <div className="sd-action-error" role="alert">{actionError}</div>
          )}

          {cardError && (
            <div className="sd-empty">
              <p>{cardError}</p>
              <button type="button" className="ghost-button" onClick={() => void loadNextPage(allCards.length ? cardPage + 1 : 1, !allCards.length)}>Thử lại</button>
            </div>
          )}

          {!cardError && allCards.length === 0 && !cardLoading && (
            <div className="sd-empty"><p>Chưa có thẻ học nào.</p></div>
          )}

          {(allCards.length > 0 || !cardLoading) && (
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
                          <StudyActionIcon name="trash" /><span>Xóa</span>
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
                    <FlashcardListCard
                      card={card}
                      onToggleStar={onToggleStar}
                      onEdit={() => { setEditingCardId(card.id); setAddingCard(false); }}
                      onDelete={() => setDeleteConfirmId(card.id)}
                    />
                  )}
                </div>
              ))}

              {cardLoading && (
                <div className="loading-skeleton" aria-busy="true">
                  {[1, 2, 3].map((i) => <div key={i} className="skeleton-row" />)}
                </div>
              )}

              {hasMore && (
                <button type="button" className="sd-load-more" disabled={cardLoading}
                  onClick={() => void loadNextPage(cardPage + 1)}>
                  {cardLoading ? "Đang tải…" : "Hiển thị thêm 50 thẻ"}
                </button>
              )}

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

          {!hasMore && allCards.length > 0 && !searchQuery && (
            <p className="sd-all-loaded">Đã hiển thị tất cả {cardTotal} thẻ</p>
          )}
        </div>
      )}

      {set.contentType === "quiz" && <QuizDetailOverview key={`${set.id}:${resumeSessionId ?? ""}`} set={set} resumeSessionId={resumeSessionId} onManageSessions={onManageSessions} onEdit={onEdit} onSave={() => void openFolderDialog()} />}

      {folderDialogOpen && <div className="sd-modal-backdrop" role="presentation" onMouseDown={() => setFolderDialogOpen(false)}><section className="sd-folder-dialog" role="dialog" aria-modal="true" onMouseDown={(event) => event.stopPropagation()}><header><div><p className="eyebrow">Sắp xếp quiz</p><h2>Thêm vào thư mục</h2></div><button className="ghost-button" onClick={() => setFolderDialogOpen(false)}>×</button></header>{folders.length === 0 ? <p>Bạn chưa có thư mục. Hãy tạo thư mục từ thanh bên trước.</p> : <div className="sd-folder-options">{folders.map((folder) => <button key={folder.id} onClick={() => void addToFolder(folder.id)}><span>📁</span><div><strong>{folder.title}</strong><small>{folder.studySetCount} học phần</small></div><b>Thêm</b></button>)}</div>}</section></div>}
    </div>
  );
}
